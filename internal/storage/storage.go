/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

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
	MailDelete(mailbox string, id int) error
	MailExpunge(mailbox string) error
	MailCount(mailbox string) (int, error)
	MailMove(mailbox string, id int, destination string) error

	QueueListDestinations() ([]string, error)
	QueueMailIDsForDestination(destination string) ([]types.QueuedMail, error)
	QueueInsertDestinationForID(destination string, id int, from, rcpt string) error
	QueueDeleteDestinationForID(destination string, id int) error
	QueueSelectIsMessagePendingSend(mailbox string, id int) (bool, error)
}
