package storage

import "github.com/neilalexander/yggmail/internal/storage/types"

type Storage interface {
	ConfigGet(key string) (string, error)
	ConfigSet(key, value string) error
	ConfigSetPassword(password string) error
	ConfigTryPassword(password string) (bool, error)

	MailboxSelect(mailbox string) (bool, error)
	MailNextID(mailbox string) (int, error)
	MailIDForSeq(mailbox string, id int) (int, error)
	MailUnseen(mailbox string) (int, error)
	MailboxList(onlySubscribed bool) ([]string, error)
	MailboxCreate(name string) error
	MailboxRename(old, new string) error
	MailboxDelete(name string) error
	MailboxSubscribe(name string, subscribed bool) error

	MailCreate(mailbox string, data []byte) (int, error)
	MailSelect(mailbox string, id int) (int, *types.Mail, error)
	MailSearch(mailbox string) ([]uint32, error)
	MailUpdateFlags(mailbox string, id int, seen, answered, flagged, deleted bool) error
	MailDelete(mailbox, id string) error
	MailExpunge(mailbox string) error
	MailCount(mailbox string) (int, error)
}
