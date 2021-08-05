package yggmail

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/fatih/color"

	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/imapserver"
	"github.com/neilalexander/yggmail/internal/smtpsender"
	"github.com/neilalexander/yggmail/internal/smtpserver"
	"github.com/neilalexander/yggmail/internal/storage/sqlite3"
	"github.com/neilalexander/yggmail/internal/transport"
	"github.com/neilalexander/yggmail/internal/utils"
)

type peerAddrList []string

type Yggmail struct {
	storage           *sqlite3.SQLite3Storage
	imapServer        *imapserver.IMAPServer
	localSmtpServer   *smtp.Server
	overlaySmtpServer *smtp.Server
	DatabaseName      string
}

func (ym *Yggmail) OpenDatabase() {
	storage, err := sqlite3.NewSQLite3StorageStorage(ym.DatabaseName)
	if err != nil {
		panic(err)
	}
	ym.storage = storage
	log.Printf("Using database file %q\n", ym.DatabaseName)
}

func (ym *Yggmail) CreatePassword(password string) {
	if ym.storage == nil {
		ym.OpenDatabase()
	}
	if err := ym.storage.ConfigSetPassword(strings.TrimSpace(string(password))); err != nil {
		log.Println("Failed to set password:", err)
		ym.CloseDatabase()
		os.Exit(1)
	}
}

func (ym *Yggmail) CloseDatabase() {
	if ym.storage != nil {
		ym.storage.Close()
		ym.storage = nil
	}
}

// Start starts imap and smtp server, peers is be a comma separated sting
func (ym *Yggmail) Start(smtpaddr string, imapaddr string, multicast bool, peers string) {
	rawlog := log.New(color.Output, "", 0)
	green := color.New(color.FgGreen).SprintfFunc()
	log := log.New(rawlog.Writer(), fmt.Sprintf("[  %s  ] ", green("Yggmail")), 0)

	var peerAddrs peerAddrList = strings.Split(peers, ",")
	/*database := flag.String("database", "yggmail.db", "SQLite database file")
	smtpaddr := flag.String("smtp", "localhost:1025", "SMTP listen address")
	imapaddr := flag.String("imap", "localhost:1143", "IMAP listen address")
	multicast := flag.Bool("multicast", false, "Connect to Yggdrasil peers on your LAN")
	password := flag.Bool("password", false, "Set a new IMAP/SMTP password")
	flag.Var(&peerAddrs, "peer", "Connect to a specific Yggdrasil static peer (this option can be given more than once)")
	flag.Parse()*/

	skStr, err := ym.storage.ConfigGet("private_key")
	if err != nil {
		panic(err)
	}

	sk := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	if skStr == "" {
		if _, sk, err = ed25519.GenerateKey(nil); err != nil {
			panic(err)
		}
		if err := ym.storage.ConfigSet("private_key", hex.EncodeToString(sk)); err != nil {
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
		if err := ym.storage.MailboxCreate(name); err != nil {
			panic(err)
		}
	}

	if !multicast && len(peerAddrs) == 0 {
		log.Printf("You must specify either -peer, -multicast or both!")
		os.Exit(0)
	}

	cfg := &config.Config{
		PublicKey:  pk,
		PrivateKey: sk,
	}

	transport, err := transport.NewYggdrasilTransport(rawlog, sk, pk, peerAddrs, multicast)
	if err != nil {
		panic(err)
	}

	queues := smtpsender.NewQueues(cfg, log, transport, ym.storage)
	var notify *imapserver.IMAPNotify

	imapBackend := &imapserver.Backend{
		Log:     log,
		Config:  cfg,
		Storage: ym.storage,
	}

	ym.imapServer, notify, err = imapserver.NewIMAPServer(imapBackend, imapaddr, true)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening for IMAP on:", imapaddr)
	localBackend := &smtpserver.Backend{
		Log:     log,
		Mode:    smtpserver.BackendModeInternal,
		Config:  cfg,
		Storage: ym.storage,
		Queues:  queues,
		Notify:  notify,
	}

	ym.localSmtpServer = smtp.NewServer(localBackend)
	ym.localSmtpServer.Addr = smtpaddr
	ym.localSmtpServer.Domain = hex.EncodeToString(pk)
	ym.localSmtpServer.MaxMessageBytes = 1024 * 1024
	ym.localSmtpServer.MaxRecipients = 50
	ym.localSmtpServer.AllowInsecureAuth = true

	overlayBackend := &smtpserver.Backend{
		Log:     log,
		Mode:    smtpserver.BackendModeExternal,
		Config:  cfg,
		Storage: ym.storage,
		Queues:  queues,
		Notify:  notify,
	}

	ym.overlaySmtpServer = smtp.NewServer(overlayBackend)
	ym.overlaySmtpServer.Domain = hex.EncodeToString(pk)
	ym.overlaySmtpServer.MaxMessageBytes = 1024 * 1024
	ym.overlaySmtpServer.MaxRecipients = 50
	ym.overlaySmtpServer.AuthDisabled = true

	go func() {
		ym.localSmtpServer.EnableAuth(sasl.Login, func(conn *smtp.Conn) sasl.Server {
			return sasl.NewLoginServer(func(username, password string) error {
				_, err := localBackend.Login(nil, username, password)
				return err
			})
		})

		log.Println("Listening for SMTP on:", ym.localSmtpServer.Addr)
		if err := ym.localSmtpServer.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		if err := ym.overlaySmtpServer.Serve(transport.Listener()); err != nil {
			log.Fatal(err)
		}
	}()

}

func (ym *Yggmail) Stop() {
	log.Println("Shutting down")
	if ym.localSmtpServer != nil {
		ym.localSmtpServer.Close()
		ym.localSmtpServer = nil
	}
	if ym.overlaySmtpServer != nil {
		ym.overlaySmtpServer.Close()
		ym.overlaySmtpServer = nil
	}
	if ym.imapServer != nil {
		ym.imapServer.Stop()
		ym.imapServer = nil
	}

	ym.CloseDatabase()
}
