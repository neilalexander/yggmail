/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/fatih/color"
	"golang.org/x/term"

	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/imapserver"
	"github.com/neilalexander/yggmail/internal/smtpsender"
	"github.com/neilalexander/yggmail/internal/smtpserver"
	"github.com/neilalexander/yggmail/internal/storage/sqlite3"
	"github.com/neilalexander/yggmail/internal/transport"
	"github.com/neilalexander/yggmail/internal/utils"

	"golang.org/x/crypto/bcrypt"
)

type peerAddrList []string

func (i *peerAddrList) String() string {
	return strings.Join(*i, ", ")
}

func (i *peerAddrList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	rawlog := log.New(color.Output, "", 0)
	green := color.New(color.FgGreen).SprintfFunc()
	log := log.New(rawlog.Writer(), fmt.Sprintf("[  %s  ] ", green("Yggmail")), log.LstdFlags|log.Lmsgprefix)

	var peerAddrs peerAddrList
	database := flag.String("database", "yggmail.db", "SQLite database file")
	smtpaddr := flag.String("smtp", "localhost:1025", "SMTP listen address")
	imapaddr := flag.String("imap", "localhost:1143", "IMAP listen address")
	multicast := flag.Bool("multicast", false, "Connect to Yggdrasil peers on your LAN")
        mcastregexp := flag.String("mcastregexp", ".*", "Regexp for multicast")
	password := flag.Bool("password", false, "Set a new IMAP/SMTP password")
	passwordhash := flag.String("passwordhash", "", "Set a new IMAP/SMTP password (hash)")
	flag.Var(&peerAddrs, "peer", "Connect to a specific Yggdrasil static peer (this option can be given more than once)")
	flag.Parse()

	if flag.NFlag() == 0 {
		fmt.Println("Yggmail must be started with either one or more Yggdrasil peers")
		fmt.Println("specified, multicast enabled, or both.")
		fmt.Println()
		fmt.Println("Available options:")
		fmt.Println()
		flag.PrintDefaults()
		os.Exit(0)
	}

	storage, err := sqlite3.NewSQLite3StorageStorage(*database)
	if err != nil {
		panic(err)
	}
	defer storage.Close()
	log.Printf("Using database file %q\n", *database)

	skStr, err := storage.ConfigGet("private_key")
	if err != nil {
		panic(err)
	}

	sk := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	if skStr == "" {
		if _, sk, err = ed25519.GenerateKey(nil); err != nil {
			panic(err)
		}
		if err := storage.ConfigSet("private_key", hex.EncodeToString(sk)); err != nil {
			panic(err)
		}
		log.Printf("Generated new server identity")
	} else {
		skBytes, err := hex.DecodeString(skStr)
		if err != nil {
			panic(err)
		}
		copy(sk, skBytes)
	}
	pk := sk.Public().(ed25519.PublicKey)
	log.Printf("Mail address: %s@%s\n", hex.EncodeToString(pk), utils.Domain)

	for _, name := range []string{"INBOX", "Outbox"} {
		if err := storage.MailboxCreate(name); err != nil {
			panic(err)
		}
	}

	switch {
	case password != nil && *password:
		log.Println("Please enter your new password:")
		password1, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		fmt.Println()
		log.Println("Please enter your new password again:")
		password2, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		fmt.Println()
		if !bytes.Equal(password1, password2) {
			log.Println("The supplied passwords do not match")
			os.Exit(1)
		}
		
		// trim away whitespace of UTF-8 bytes now as string
		finalPassword := strings.TrimSpace(string(password1))

		// perform hash
		hash, err := bcrypt.GenerateFromPassword([]byte(finalPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("bcrypt.GenerateFromPassword: %v\n", err)
			os.Exit(1)
		} else if err := storage.ConfigSetPassword(string(hash)); err != nil {
			log.Println("Failed to set password:", err)
			os.Exit(1)
		}

		log.Println("Password for IMAP and SMTP has been updated!")
		os.Exit(0)

	case passwordhash != nil && *passwordhash != "":
		var hash string = strings.TrimSpace(*passwordhash);
		if len(hash) == 0 {
			log.Println("Password hash cannot be blank");
			os.Exit(1);
		}
		
		log.Printf("Using password hash: '%v'\n", hash);

		if err := storage.ConfigSetPassword(hash); err != nil {
			log.Println("Failed to set password: ", err);
			os.Exit(1)
		}

		log.Println("Password for IMAP and SMTP has been updated!")
	case (multicast == nil || !*multicast) && len(peerAddrs) == 0:
		log.Printf("You must specify either -peer, -multicast or both!")
		os.Exit(0)

	}

	cfg := &config.Config{
		PublicKey:  pk,
		PrivateKey: sk,
	}

	transport, err := transport.NewYggdrasilTransport(rawlog, sk, pk, peerAddrs, *multicast, *mcastregexp)
	if err != nil {
		panic(err)
	}

	queues := smtpsender.NewQueues(cfg, log, transport, storage)
	var notify *imapserver.IMAPNotify

	imapBackend := &imapserver.Backend{
		Log:     log,
		Config:  cfg,
		Storage: storage,
	}

	_, notify, err = imapserver.NewIMAPServer(imapBackend, *imapaddr, true)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening for IMAP on:", *imapaddr)

	go func() {
		localBackend := &smtpserver.Backend{
			Log:     log,
			Mode:    smtpserver.BackendModeInternal,
			Config:  cfg,
			Storage: storage,
			Queues:  queues,
			Notify:  notify,
		}

		localServer := smtp.NewServer(localBackend)
		localServer.Addr = *smtpaddr
		localServer.Domain = hex.EncodeToString(pk)
		localServer.MaxMessageBytes = 1024 * 1024 * 32
		localServer.MaxRecipients = 50
		localServer.AllowInsecureAuth = true
		localServer.EnableAuth(sasl.Login, func(conn *smtp.Conn) sasl.Server {
			return sasl.NewLoginServer(func(username, password string) error {
				_, err := localBackend.Login(nil, username, password)
				return err
			})
		})

		log.Println("Listening for SMTP on:", localServer.Addr)
		if err := localServer.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		overlayBackend := &smtpserver.Backend{
			Log:     log,
			Mode:    smtpserver.BackendModeExternal,
			Config:  cfg,
			Storage: storage,
			Queues:  queues,
			Notify:  notify,
		}

		overlayServer := smtp.NewServer(overlayBackend)
		overlayServer.Domain = hex.EncodeToString(pk)
		overlayServer.MaxMessageBytes = 1024 * 1024 * 32
		overlayServer.MaxRecipients = 50
		overlayServer.AuthDisabled = true

		if err := overlayServer.Serve(transport.Listener()); err != nil {
			log.Fatal(err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Println("Shutting down")
}
