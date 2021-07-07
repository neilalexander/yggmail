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

	"github.com/emersion/go-imap/server"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"golang.org/x/term"

	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/imapserver"
	"github.com/neilalexander/yggmail/internal/smtpsender"
	"github.com/neilalexander/yggmail/internal/smtpserver"
	"github.com/neilalexander/yggmail/internal/storage/sqlite3"
	"github.com/neilalexander/yggmail/internal/transport"
)

var database = flag.String("database", "yggmail.db", "SQLite database file")
var smtpaddr = flag.String("smtp", "localhost:1025", "SMTP listen address")
var imapaddr = flag.String("imap", "localhost:1026", "IMAP listen address")
var peeraddr = flag.String("peer", "", "Yggdrasil static peer")
var createuser = flag.String("createuser", "", "Create a user")

func main() {
	flag.Parse()

	rawlog := log.New(os.Stdout, "", 0)
	log := log.New(rawlog.Writer(), "[  \033[32mYggmail\033[0m  ] ", 0)

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
	log.Println("Mail domain:", hex.EncodeToString(pk))

	switch {
	case createuser != nil && *createuser != "":
		fmt.Printf("New password: ")
		password1, err := term.ReadPassword(0)
		if err != nil {
			panic(err)
		}
		fmt.Println()
		fmt.Printf("Confirm password: ")
		password2, err := term.ReadPassword(0)
		if err != nil {
			panic(err)
		}
		fmt.Println()
		if !bytes.Equal(password1, password2) {
			fmt.Println("The supplied passwords do not match")
			os.Exit(1)
		}
		if err := storage.CreateUser(*createuser, strings.TrimSpace(string(password1))); err != nil {
			fmt.Printf("Failed to create user %q\n", *createuser)
			os.Exit(1)
		}
		if err := storage.MailboxCreate(*createuser, "INBOX"); err != nil {
			panic(err)
		}
		fmt.Printf("Created user %q\n", *createuser)
		fmt.Printf("Email address will be %s@%s\n", *createuser, hex.EncodeToString(pk))
		os.Exit(0)
	}

	cfg := &config.Config{
		PublicKey:  pk,
		PrivateKey: sk,
	}
	wg := &sync.WaitGroup{}
	wg.Add(2)

	transport, err := transport.NewYggdrasilTransport(rawlog, sk, pk, *peeraddr)
	if err != nil {
		panic(err)
	}

	queues := smtpsender.NewQueues(cfg, log, transport)

	go func() {
		defer wg.Done()

		imapBackend := &imapserver.Backend{
			Log:     log,
			Config:  cfg,
			Storage: storage,
		}

		imapServer := server.New(imapBackend)
		imapServer.Addr = *imapaddr
		imapServer.AllowInsecureAuth = true
		imapServer.EnableAuth(sasl.Login, func(conn server.Conn) sasl.Server {
			return sasl.NewLoginServer(func(username, password string) error {
				_, err := imapBackend.Login(nil, username, password)
				return err
			})
		})

		log.Println("Listening for IMAP on:", imapServer.Addr)
		if err := imapServer.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()

		localBackend := &smtpserver.Backend{
			Log:     log,
			Mode:    smtpserver.BackendModeInternal,
			Config:  cfg,
			Storage: storage,
			Queues:  queues,
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
