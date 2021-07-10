package smtpserver

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/emersion/go-smtp"
	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/imapserver"
	"github.com/neilalexander/yggmail/internal/smtpsender"
	"github.com/neilalexander/yggmail/internal/storage"
	"github.com/neilalexander/yggmail/internal/utils"
)

type BackendMode int

const (
	BackendModeInternal BackendMode = iota
	BackendModeExternal
)

type Backend struct {
	Mode    BackendMode
	Log     *log.Logger
	Config  *config.Config
	Queues  *smtpsender.Queues
	Storage storage.Storage
	Notify  *imapserver.IMAPNotify
}

func (b *Backend) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	switch b.Mode {
	case BackendModeInternal:
		// If our username is email-like, then take just the localpart
		if pk, err := utils.ParseAddress(username); err == nil {
			if !pk.Equal(b.Config.PublicKey) {
				return nil, fmt.Errorf("failed to authenticate: wrong domain in username")
			}
		}
		username = hex.EncodeToString(b.Config.PublicKey)
		// The connection came from our local listener
		if authed, err := b.Storage.ConfigTryPassword(password); err != nil {
			b.Log.Printf("Failed to authenticate SMTP user %q due to error: %s", username, err)
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		} else if !authed {
			b.Log.Printf("Failed to authenticate SMTP user %q\n", username)
			return nil, smtp.ErrAuthRequired
		}
		defer b.Log.Printf("Authenticated SMTP user from %s as %q\n", state.RemoteAddr.String(), username)
		return &SessionLocal{
			backend: b,
			state:   state,
		}, nil

	case BackendModeExternal:
		return nil, fmt.Errorf("Not expecting authenticated connection on external backend")
	}

	return nil, fmt.Errorf("Authenticated login failed")
}

func (b *Backend) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	switch b.Mode {
	case BackendModeInternal:
		return nil, fmt.Errorf("Not expecting anonymous connection on internal backend")

	case BackendModeExternal:
		// The connection came from our overlay listener, so we should check
		// that they are who they claim to be
		pks, err := hex.DecodeString(state.RemoteAddr.String())
		if err != nil {
			return nil, fmt.Errorf("hex.DecodeString: %w", err)
		}
		remote := hex.EncodeToString(pks)
		if state.Hostname != remote {
			return nil, fmt.Errorf("You are not who you claim to be")
		}

		b.Log.Println("Incoming SMTP session from", remote)
		return &SessionRemote{
			backend: b,
			state:   state,
			public:  pks[:],
		}, nil
	}

	return nil, fmt.Errorf("Anonymous login failed")
}
