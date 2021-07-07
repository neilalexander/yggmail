package imapserver

import (
	"fmt"
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/storage"
)

type Backend struct {
	Config  *config.Config
	Log     *log.Logger
	Storage storage.Storage
}

func (b *Backend) Login(_ *imap.ConnInfo, username, password string) (backend.User, error) {
	if authed, err := b.Storage.TryAuthenticate(username, password); err != nil {
		b.Log.Printf("Failed to authenticate IMAP user %q due to error: %s", username, err)
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	} else if !authed {
		b.Log.Printf("Failed to authenticate IMAP user %q\n", username)
		return nil, backend.ErrInvalidCredentials
	}
	defer b.Log.Printf("Authenticated IMAP user %q\n", username)
	user := &User{
		backend:  b,
		username: username,
	}
	return user, nil
}
