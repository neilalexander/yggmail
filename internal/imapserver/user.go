package imapserver

import (
	"errors"
	"fmt"

	"github.com/emersion/go-imap/backend"
)

type User struct {
	backend  *Backend
	username string
}

func (u *User) Username() string {
	return u.username
}

func (u *User) ListMailboxes(subscribed bool) (mailboxes []backend.Mailbox, err error) {
	names, err := u.backend.Storage.MailboxList(u.username, subscribed)
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
	ok, _ := u.backend.Storage.MailboxSelect(u.username, name)
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
	return u.backend.Storage.MailboxCreate(u.username, name)
}

func (u *User) DeleteMailbox(name string) error {
	if name == "INBOX" {
		return errors.New("Cannot delete INBOX")
	}
	return u.backend.Storage.MailboxDelete(u.username, name)
}

func (u *User) RenameMailbox(existingName, newName string) error {
	if existingName == "INBOX" {
		return errors.New("Cannot rename INBOX")
	}
	return u.backend.Storage.MailboxRename(u.username, existingName, newName)
}

func (u *User) Logout() error {
	return nil
}
