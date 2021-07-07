package storage

import "time"

type Storage interface {
	ConfigGet(key string) (string, error)
	ConfigSet(key, value string) error

	TryAuthenticate(username, password string) (bool, error)

	MailboxSelect(user, mailbox string) (bool, error)
	MailNextID(user, mailbox string) (int, error)
	MailIDForSeq(user, mailbox string, id int) (int, error)
	MailUnseen(user, mailbox string) (int, error)
	MailboxList(user string, onlySubscribed bool) ([]string, error)
	MailboxCreate(user, name string) error
	MailboxRename(user, old, new string) error
	MailboxDelete(user, name string) error
	MailboxSubscribe(user, name string, subscribed bool) error

	MailCreate(user, mailbox string, data []byte) (int, error)
	MailSelect(user, mailbox string, id int) (int, int, []byte, bool, bool, bool, bool, time.Time, error)
	MailSearch(user, mailbox string) ([]uint32, error)
	MailUpdateFlags(user, mailbox string, id int, seen, answered, flagged, deleted bool) error
	MailDelete(user, mailbox, id string) error
	MailExpunge(user, mailbox string) error
	MailCount(user, mailbox string) (int, error)
}
