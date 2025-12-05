/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package imapserver

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
)

type User struct {
	backend  *Backend
	username string
	conn     *imap.ConnInfo
	log			 *log.Logger
}

func (u *User) Username() string {
	return hex.EncodeToString(u.backend.Config.PublicKey)
}

func (u *User) ListMailboxes(subscribed bool) (mailboxes []backend.Mailbox, err error) {
	names, err := u.backend.Storage.MailboxList(subscribed)
	if err != nil {
		return nil, err
	}

	for _, mailbox := range names {
		mailboxes = append(mailboxes, &Mailbox{
			backend: u.backend,
			user:    u,
			name:    mailbox,
		})
	}

	return
}

func (u *User) GetMailbox(name string) (mailbox backend.Mailbox, err error) {
	if name == "" {
		return &Mailbox{
			backend: u.backend,
			user:    u,
			name:    "",
		}, nil
	}
	ok, _ := u.backend.Storage.MailboxSelect(name)
	if !ok {
		return nil, fmt.Errorf("mailbox %q not found", name)
	}
	return &Mailbox{
		backend: u.backend,
		user:    u,
		name:    name,
	}, nil
}

func (u *User) CreateMailbox(name string) error {
	u.log.Printf("Creating mailbox '%s'...\n", name)
	return u.backend.Storage.MailboxCreate(name)
}

func (u *User) DeleteMailbox(name string) error {
	switch name {
	case "INBOX", "Outbox", "Sent":
		return errors.New("Cannot delete " + name)
	default:
		if e := u.backend.Storage.MailboxDelete(name); e != nil {
			 u.log.Printf("Error deleting mailbox '%s': %w\n", e)	
			 return e;
		} else {
			return e;
		}
	}
}

func (u *User) RenameMailbox(existingName, newName string) error {
	switch existingName {
	case "INBOX", "Outbox", "Sent":
		return errors.New("Cannot rename " + existingName)
	default:
		return u.backend.Storage.MailboxRename(existingName, newName)
	}
}

func (u *User) Logout() error {
	return nil
}
