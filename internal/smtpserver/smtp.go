package smtpserver

import (
	"github.com/emersion/go-smtp"
	"github.com/neilalexander/yggmail/internal/imapserver"
)

type SMTPServer struct {
	server  *smtp.Server
	backend smtp.Backend
	notify  *imapserver.IMAPNotify
}

func NewSMTPServer(backend smtp.Backend, notify *imapserver.IMAPNotify) *SMTPServer {
	s := &SMTPServer{
		server:  smtp.NewServer(backend),
		backend: backend,
		notify:  notify,
	}
	return s
}
