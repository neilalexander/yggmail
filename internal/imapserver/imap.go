package imapserver

import (
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/server"
)

type IMAPServer struct {
	server  *server.Server
	backend *Backend
}

func NewIMAPServer(backend *Backend) (*IMAPServer, error) {
	s := &IMAPServer{
		server:  server.New(backend),
		backend: backend,
	}
	s.server.Enable(idle.NewExtension())
	return s, nil
}
