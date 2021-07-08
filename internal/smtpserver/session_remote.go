package smtpserver

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"
	"github.com/neilalexander/yggmail/internal/utils"
)

type SessionRemote struct {
	backend *Backend
	state   *smtp.ConnectionState
	public  ed25519.PublicKey
	from    string
}

func (s *SessionRemote) Mail(from string, opts smtp.MailOptions) error {
	pk, err := utils.ParseAddress(from)
	if err != nil {
		return fmt.Errorf("mail.ParseAddress: %w", err)
	}

	if remote := s.state.RemoteAddr.String(); hex.EncodeToString(pk) != remote {
		return fmt.Errorf("not allowed to send incoming mail as %s", from)
	}

	s.from = from
	return nil
}

func (s *SessionRemote) Rcpt(to string) error {
	pk, err := utils.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("mail.ParseAddress: %w", err)
	}

	if !pk.Equal(s.backend.Config.PublicKey) {
		return fmt.Errorf("unexpected recipient for wrong domain")
	}

	return nil
}

func (s *SessionRemote) Data(r io.Reader) error {
	m, err := message.Read(r)
	if err != nil {
		return fmt.Errorf("message.Read: %w", err)
	}

	m.Header.Add(
		"Received", fmt.Sprintf("from Yggmail %s; %s",
			hex.EncodeToString(s.public),
			time.Now().String(),
		),
	)

	var b bytes.Buffer
	if err := m.WriteTo(&b); err != nil {
		return fmt.Errorf("m.WriteTo: %w", err)
	}

	if _, err := s.backend.Storage.MailCreate("INBOX", b.Bytes()); err != nil {
		return fmt.Errorf("s.backend.Storage.StoreMessageFor: %w", err)
	}
	s.backend.Log.Printf("Stored new mail from %s", s.from)

	return nil
}

func (s *SessionRemote) Reset() {}

func (s *SessionRemote) Logout() error {
	return nil
}
