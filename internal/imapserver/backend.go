package imapserver

import (
	"fmt"
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/jxskiss/base62"
	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/storage"
	"github.com/neilalexander/yggmail/internal/utils"
)

type Backend struct {
	Config  *config.Config
	Log     *log.Logger
	Storage storage.Storage
}

func (b *Backend) Login(_ *imap.ConnInfo, username, password string) (backend.User, error) {
	// If our username is email-like, then take just the localpart
	if pk, err := utils.ParseAddress(username); err == nil {
		if !pk.Equal(b.Config.PublicKey) {
			b.Log.Println("Failed to authenticate IMAP user due to wrong domain", pk, b.Config.PublicKey)
			return nil, fmt.Errorf("failed to authenticate: wrong domain in username")
		}
	}
	username = base62.EncodeToString(b.Config.PublicKey)
	if authed, err := b.Storage.ConfigTryPassword(password); err != nil {
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
