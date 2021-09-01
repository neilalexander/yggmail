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

const ERROR_OPEN_DB int = 1
const ERROR_PASSWORD int = 2
const ERROR_START int = 3
const ERROR_SMTP int = 4
const ERROR_OVERLAY_SMTP int = 5
const ERROR_IMAP int = 6
const ERROR_AUTH int = 7

type Yggmail struct {
	storage           *sqlite3.SQLite3Storage
	imapServer        *imapserver.IMAPServer
	localSmtpServer   *smtp.Server
	overlaySmtpServer *smtp.Server
	DatabaseName      string
	Logger            Logger
	AccountName       string
}

func (ym *Yggmail) OpenDatabase() {
	if err := ym.openDatabase(); err != nil {
		ym.sendError(ERROR_OPEN_DB, "%s", *err)
	}
}

func (ym *Yggmail) openDatabase() *error {
	storage, err := sqlite3.NewSQLite3StorageStorage(ym.DatabaseName)
	if err != nil {
		return &err
	}
	ym.storage = storage
	ym.sendLog("Using database file %s ", ym.DatabaseName)
	return nil
}

func (ym *Yggmail) CreatePassword(password string) {
	if err := ym.createPassword(password); err != nil {
		ym.sendError(ERROR_PASSWORD, fmt.Sprint(err))
	}
}

func (ym *Yggmail) createPassword(password string) *error {
	if ym.storage == nil {
		ym.OpenDatabase()
	}
	if err := ym.storage.ConfigSetPassword(strings.TrimSpace(string(password))); err != nil {
		log.Printf("Failed to set password: %s", err)
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

func (ym *Yggmail) Start(smtpaddr string, imapaddr string, multicast bool, peers string) {
	if err := ym.start(smtpaddr, imapaddr, multicast, peers); err != nil {
		ym.sendError(ERROR_START, "%s", *err)
	}
}

// Start starts imap and smtp server, peers is be a comma separated sting
func (ym *Yggmail) start(smtpaddr string, imapaddr string, multicast bool, peers string) *error {

	logWriter := LogWriter{
		Output: log.New(color.Output, "", 0).Writer(),
		Logger: ym.Logger,
	}

	yggmailLog := log.New(&logWriter, "[  Yggmail  ] ", 0)
	var peerAddrs peerAddrList = strings.Split(peers, ",")

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
		yggmailLog.Printf("Generated new server identity")
	} else {
		skBytes, err := hex.DecodeString(skStr)
		if err != nil {
			return &err
		}
		copy(sk, skBytes)
	}
	pk := sk.Public().(ed25519.PublicKey)
	ym.sendLog("Mail address: %s@%s", hex.EncodeToString(pk), utils.Domain)
	ym.AccountName = hex.EncodeToString(pk)

	for _, name := range []string{"INBOX", "Outbox"} {
		if err := ym.storage.MailboxCreate(name); err != nil {
			return &err
		} else {
			yggmailLog.Printf("Mailbox created: %s", name)
		}
	}

	if !multicast && len(peerAddrs) == 0 {
		yggmailLog.Printf("You must specify either -peer, -multicast or both!")
		err := errors.New("You must specify either -peer, -multicast or both!")
		return &err
	} else {
		yggmailLog.Printf("multicast/peer Address check successfully passed")
	}

	cfg := &config.Config{
		PublicKey:  pk,
		PrivateKey: sk,
	}

	yggdrasilLog := log.New(&logWriter, "", 0)
	transport, err := transport.NewYggdrasilTransport(yggdrasilLog, sk, pk, peerAddrs, multicast)
	if err != nil {
		return &err
	}

	queues := smtpsender.NewQueues(cfg, yggmailLog, transport, ym.storage)
	var notify *imapserver.IMAPNotify

	imapBackend := &imapserver.Backend{
		Log:     yggmailLog,
		Config:  cfg,
		Storage: ym.storage,
	}

	ym.imapServer, notify, err = imapserver.NewIMAPServer(imapBackend, imapaddr, true)
	if err != nil {
		return &err
	}
	yggmailLog.Printf("Listening for IMAP on: %s", imapaddr)
	localBackend := &smtpserver.Backend{
		Log:     yggmailLog,
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
		Log:     yggmailLog,
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
				ym.sendError(ERROR_AUTH, "SMTP login error: %s", err)
				return err
			})
		})

		ym.sendLog("Listening for SMTP on: %s", ym.localSmtpServer.Addr)
		if err := ym.localSmtpServer.ListenAndServe(); err != nil {
			ym.sendError(ERROR_SMTP, "SMTP error %s", err)
		}
	}()

	go func() {
		if err := ym.overlaySmtpServer.Serve(transport.Listener()); err != nil {
			ym.sendError(ERROR_OVERLAY_SMTP, "OVERLAY SMTP error %s", err)
		}
	}()

	return nil
}

func (ym *Yggmail) Stop() {
	ym.sendLog("Shutting down yggmail...")
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

func (ym *Yggmail) sendError(errorId int, format string, a ...interface{}) {
	if ym.Logger != nil {
		ym.Logger.LogError(errorId, fmt.Sprintf("[  Yggmail  ] %s", fmt.Sprintf(format, a...)))
	}
}

func (ym *Yggmail) sendLog(format string, a ...interface{}) {
	if ym.Logger != nil {
		ym.Logger.LogMessage(fmt.Sprintf("[  Yggmail  ] %s", fmt.Sprintf(format, a...)))
	}
}
