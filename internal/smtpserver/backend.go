package smtpserver

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/emersion/go-smtp"
	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/smtpsender"
	"github.com/neilalexander/yggmail/internal/storage"
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
}

func (b *Backend) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	switch b.Mode {
	case BackendModeInternal:
		// The connection came from our local listener
		if authed, err := b.Storage.TryAuthenticate(username, password); err != nil {
			b.Log.Printf("Failed to authenticate SMTP user %q due to error: %s", username, err)
			return nil, fmt.Errorf("failed to authenticate: %w", err)
		} else if !authed {
			b.Log.Printf("Failed to authenticate SMTP user %q\n", username)
			return nil, smtp.ErrAuthRequired
		}
		defer b.Log.Printf("Authenticated SMTP user %q\n", username)
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
		if state.Hostname != state.RemoteAddr.String() {
			return nil, fmt.Errorf("You are not who you claim to be")
		}

		pks, err := hex.DecodeString(state.RemoteAddr.String())
		if err != nil {
			return nil, fmt.Errorf("hex.DecodeString: %w", err)
		}

		b.Log.Println("Incoming SMTP session from", state.RemoteAddr.String())
		return &SessionRemote{
			backend: b,
			state:   state,
			public:  pks[:],
		}, nil
	}

	return nil, fmt.Errorf("Anonymous login failed")
}
