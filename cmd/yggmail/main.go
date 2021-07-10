package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"golang.org/x/term"

	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/imapserver"
	"github.com/neilalexander/yggmail/internal/smtpsender"
	"github.com/neilalexander/yggmail/internal/smtpserver"
	"github.com/neilalexander/yggmail/internal/storage/sqlite3"
	"github.com/neilalexander/yggmail/internal/transport"
	"github.com/neilalexander/yggmail/internal/utils"
)

var database = flag.String("database", "yggmail.db", "SQLite database file")
var smtpaddr = flag.String("smtp", "localhost:1025", "SMTP listen address")
var imapaddr = flag.String("imap", "localhost:1143", "IMAP listen address")
var peeraddr = flag.String("peer", "", "Connect to a specific Yggdrasil static peer")
var multicast = flag.Bool("multicast", false, "Connect to Yggdrasil peers on your LAN")
var password = flag.Bool("password", false, "Set a new IMAP/SMTP password")

func main() {
	rawlog := log.New(os.Stdout, "", 0)
	log := log.New(rawlog.Writer(), "[  \033[32mYggmail\033[0m  ] ", 0)

	flag.Parse()
	if flag.NFlag() == 0 {
		fmt.Println("Yggmail must be started with either an Yggdrasil peer")
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
		if err := storage.ConfigSetPassword(strings.TrimSpace(string(password1))); err != nil {
			log.Println("Failed to set password:", err)
			os.Exit(1)
		}

		log.Println("Password for IMAP and SMTP has been updated!")
		os.Exit(0)

	case (multicast == nil || !*multicast) && (peeraddr == nil || *peeraddr == ""):
		log.Printf("You must specify either -peer, -multicast or both!")
		os.Exit(0)
	}

	cfg := &config.Config{
		PublicKey:  pk,
		PrivateKey: sk,
	}
	wg := &sync.WaitGroup{}
	wg.Add(2)

	transport, err := transport.NewYggdrasilTransport(rawlog, sk, pk, *peeraddr, *multicast)
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
		defer wg.Done()

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
		localServer.MaxMessageBytes = 1024 * 1024
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
		defer wg.Done()

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
		overlayServer.MaxMessageBytes = 1024 * 1024
		overlayServer.MaxRecipients = 50
		overlayServer.AuthDisabled = true

		if err := overlayServer.Serve(transport.Listener()); err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}
