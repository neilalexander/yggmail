/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

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
	m.Header.Add(
		"Delivery-Date", time.Now().UTC().Format(time.RFC822),
	)

	var b bytes.Buffer
	if err := m.WriteTo(&b); err != nil {
		return fmt.Errorf("m.WriteTo: %w", err)
	}

	if id, err := s.backend.Storage.MailCreate("INBOX", b.Bytes()); err != nil {
		return fmt.Errorf("s.backend.Storage.StoreMessageFor: %w", err)
	} else {
		s.backend.Log.Printf("Stored new mail from %s", s.from)

		if count, err := s.backend.Storage.MailCount("INBOX"); err == nil {
			if err := s.backend.Notify.NotifyNew(id, count); err != nil {
				s.backend.Log.Println("Failed to notify:", s.from)
			}
		}
	}

	return nil
}

func (s *SessionRemote) Reset() {}

func (s *SessionRemote) Logout() error {
	return nil
}
