package smtpserver

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"
	"github.com/neilalexander/yggmail/internal/smtpsender"
)

type SessionLocal struct {
	backend *Backend
	state   *smtp.ConnectionState
	from    string
	rcpt    []string
}

func (s *SessionLocal) Mail(from string, opts smtp.MailOptions) error {
	_, host, err := parseAddress(from)
	if err != nil {
		return fmt.Errorf("parseAddress: %w", err)
	}

	if host != hex.EncodeToString(s.backend.Config.PublicKey) {
		return fmt.Errorf("not allowed to send outgoing mail as %s", from)
	}

	s.from = from
	return nil
}

func (s *SessionLocal) Rcpt(to string) error {
	s.rcpt = append(s.rcpt, to)
	return nil
}

func (s *SessionLocal) Data(r io.Reader) error {
	m, err := message.Read(r)
	if err != nil {
		return fmt.Errorf("message.Read: %w", err)
	}

	m.Header.Add(
		"Received", fmt.Sprintf("from %s by Yggmail %s; %s",
			s.state.RemoteAddr.String(),
			hex.EncodeToString(s.backend.Config.PublicKey),
			time.Now().String(),
		),
	)

	servers := make(map[string]struct{})

	for _, rcpt := range s.rcpt {
		localpart, host, err := parseAddress(rcpt)
		if err != nil {
			return fmt.Errorf("parseAddress: %w", err)
		}

		if _, ok := servers[host]; ok {
			continue
		}
		servers[host] = struct{}{}

		if host == hex.EncodeToString(s.backend.Config.PublicKey) {
			var b bytes.Buffer
			if err := m.WriteTo(&b); err != nil {
				return fmt.Errorf("m.WriteTo: %w", err)
			}
			if _, err := s.backend.Storage.MailCreate(localpart, "INBOX", b.Bytes()); err != nil {
				return fmt.Errorf("s.backend.Storage.StoreMessageFor: %w", err)
			}
			continue
		}

		queue, err := s.backend.Queues.QueueFor(host)
		if err != nil {
			return fmt.Errorf("s.backend.Queues.QueueFor: %w", err)
		}

		mail := &smtpsender.QueuedMail{
			From:        s.from,
			Rcpt:        rcpt,
			Destination: host,
		}

		var b bytes.Buffer
		if err := m.WriteTo(&b); err != nil {
			return fmt.Errorf("m.WriteTo: %w", err)
		}
		mail.Content = b.Bytes()

		if err := queue.Queue(mail); err != nil {
			return fmt.Errorf("queue.Queue: %w", err)
		}

		s.backend.Log.Println("Queued mail for", mail.Destination)
	}

	return nil
}

func (s *SessionLocal) Reset() {}

func (s *SessionLocal) Logout() error {
	return nil
}
