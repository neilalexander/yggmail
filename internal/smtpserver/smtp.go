package smtpserver

import (
	"github.com/emersion/go-smtp"
)

type SMTPServer struct {
	server  *smtp.Server
	backend smtp.Backend
}

func NewSMTPServer(backend smtp.Backend) *SMTPServer {
	s := &SMTPServer{
		server:  smtp.NewServer(backend),
		backend: backend,
	}
	return s
}
