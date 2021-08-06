package yggmail

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
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

type Logger interface {
	Log(msg string)
}

type Yggmail struct {
	storage           *sqlite3.SQLite3Storage
	imapServer        *imapserver.IMAPServer
	localSmtpServer   *smtp.Server
	overlaySmtpServer *smtp.Server
	DatabaseName      string
	Logger            Logger
	AccountName       string
}

func (ym *Yggmail) OpenDatabase() *error {
	storage, err := sqlite3.NewSQLite3StorageStorage(ym.DatabaseName)
	if err != nil {
		return &err
	}
	ym.storage = storage
	log.Printf("Using database file %q\n", ym.DatabaseName)
	return nil
}

func (ym *Yggmail) CreatePasswordLogError(password string) {
	if err := ym.CreatePassword(password); err != nil {
		ym.Logger.Log(fmt.Sprint(err))
	}
}

func (ym *Yggmail) CreatePassword(password string) *error {
	if ym.storage == nil {
		ym.OpenDatabase()
	}
	if err := ym.storage.ConfigSetPassword(strings.TrimSpace(string(password))); err != nil {
		log.Printf("Failed to set password:", err)
		return &err
	}
	return nil
}

func (ym *Yggmail) CloseDatabase() {
	if ym.storage != nil {
		ym.storage.Close()
		ym.storage = nil
	}
}

func (ym *Yggmail) StartLogError(smtpaddr string, imapaddr string, multicast bool, peers string) {
	if err := ym.Start(smtpaddr, imapaddr, multicast, peers); err != nil {
		ym.sendError("Error: %v", err)
	}
}

func (ym *Yggmail) sendError(format string, a ...interface{}) {
	if ym.Logger != nil {
		ym.Logger.Log(fmt.Sprintf(format, a...))
	}
}

// Start starts imap and smtp server, peers is be a comma separated sting
func (ym *Yggmail) Start(smtpaddr string, imapaddr string, multicast bool, peers string) *error {
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
		return &err
	}

	sk := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	if skStr == "" {
		if _, sk, err = ed25519.GenerateKey(nil); err != nil {
			return &err
		}
		if err := ym.storage.ConfigSet("private_key", hex.EncodeToString(sk)); err != nil {
			return &err
		}
		log.Printf("Generated new server identity")
	} else {
		skBytes, err := hex.DecodeString(skStr)
		if err != nil {
			return &err
		}
		copy(sk, skBytes)
	}
	pk := sk.Public().(ed25519.PublicKey)
	log.Printf("Mail address: %s@%s\n", hex.EncodeToString(pk), utils.Domain)
	ym.AccountName = hex.EncodeToString(pk)

	for _, name := range []string{"INBOX", "Outbox"} {
		if err := ym.storage.MailboxCreate(name); err != nil {
			return &err
		} else {
			log.Printf("Mailbox created: %s", name)
		}
	}

	if !multicast && len(peerAddrs) == 0 {
		log.Printf("You must specify either -peer, -multicast or both!")
		err := errors.New("You must specify either -peer, -multicast or both!")
		return &err
	} else {
		log.Printf("multicast/peer Address check successfully passed")
	}

	cfg := &config.Config{
		PublicKey:  pk,
		PrivateKey: sk,
	}

	transport, err := transport.NewYggdrasilTransport(rawlog, sk, pk, peerAddrs, multicast)
	if err != nil {
		return &err
	}
	log.Printf("YggdrasilTransport created...")

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
		return &err
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

	return nil
}

func (ym *Yggmail) Stop() {
	log.Println("Shutting yggmail down")
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
