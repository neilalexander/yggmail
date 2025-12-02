/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package sqlite3

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/neilalexander/yggmail/internal/storage/types"
)

type TableMails struct {
	db               *sql.DB
	writer           *Writer
	selectMails      *sql.Stmt
	selectMail       *sql.Stmt
	selectMailNextID *sql.Stmt
	selectIDForSeq   *sql.Stmt
	searchMail       *sql.Stmt
	createMail       *sql.Stmt
	countMails       *sql.Stmt
	countUnseenMails *sql.Stmt
	updateMailFlags  *sql.Stmt
	deleteMail       *sql.Stmt
	expungeMail      *sql.Stmt
	moveMail         *sql.Stmt
}

const mailsSchema = `
	CREATE TABLE IF NOT EXISTS mails (
		mailbox 	TEXT NOT NULL,
		id			INTEGER NOT NULL DEFAULT 1,
		mail 		BLOB NOT NULL,
		datetime    INTEGER NOT NULL,
		seen		BOOLEAN NOT NULL DEFAULT 0, -- the mail has been read
		answered	BOOLEAN NOT NULL DEFAULT 0, -- the mail has been replied to
		flagged		BOOLEAN NOT NULL DEFAULT 0, -- the mail has been flagged for later attention
		deleted		BOOLEAN NOT NULL DEFAULT 0, -- the email is marked for deletion at next EXPUNGE
		PRIMARY KEY (mailbox, id),
		FOREIGN KEY (mailbox) REFERENCES mailboxes(mailbox) ON DELETE CASCADE ON UPDATE CASCADE
	);

	CREATE VIEW IF NOT EXISTS inboxes AS SELECT * FROM (
		SELECT ROW_NUMBER() OVER (PARTITION BY mailbox) AS seq, * FROM mails
	)
	ORDER BY mailbox, id;
`

const selectMailsStmt = `
	SELECT * FROM inboxes
	ORDER BY mailbox, id
`

const selectMailStmt = `
	SELECT seq, id, mail, datetime, seen, answered, flagged, deleted FROM inboxes
	WHERE mailbox = $1 AND id = $2
	ORDER BY mailbox, id
`

const selectMailCountStmt = `
	SELECT COUNT(*) FROM mails WHERE mailbox = $1
`

const selectMailUnseenStmt = `
	SELECT COUNT(*) FROM mails WHERE mailbox = $1 AND seen = 0
`

const searchMailStmt = `
	SELECT id FROM mails
	WHERE mailbox = $1
	ORDER BY mailbox, id
`

const insertMailStmt = `
	INSERT INTO mails (mailbox, id, mail, datetime) VALUES(
		$1, (
			SELECT IFNULL(MAX(id)+1,1) AS id FROM mails
			WHERE mailbox = $1
		), $2, $3
	)
	RETURNING id;
`

const selectIDForSeqStmt = `
	SELECT id FROM inboxes
	WHERE mailbox = $1 AND seq = $2
`

const selectMailNextID = `
	SELECT IFNULL(MAX(id)+1,1) AS id FROM mails
	WHERE mailbox = $1
`

const updateMailFlagsStmt = `
	UPDATE mails SET seen = $1, answered = $2, flagged = $3, deleted = $4 WHERE mailbox = $5 AND id = $6
`

const deleteMailStmt = `
	UPDATE mails SET deleted = 1 WHERE mailbox = $1 AND id = $2
`

const expungeMailStmt = `
	DELETE FROM mails WHERE mailbox = $1 AND deleted = 1
`

const moveMailStmt = `
	UPDATE mails SET mailbox = $1 WHERE mailbox = $2 AND id = $3
`

func NewTableMails(db *sql.DB, writer *Writer) (*TableMails, error) {
	t := &TableMails{
		db:     db,
		writer: writer,
	}
	_, err := db.Exec(mailsSchema)
	if err != nil {
		return nil, fmt.Errorf("db.Exec: %w", err)
	}
	t.selectMails, err = db.Prepare(selectMailsStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(selectMailsStmt): %w", err)
	}
	t.selectMail, err = db.Prepare(selectMailStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(selectMailStmt): %w", err)
	}
	t.selectMailNextID, err = db.Prepare(selectMailNextID)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(selectMailNextID): %w", err)
	}
	t.selectIDForSeq, err = db.Prepare(selectIDForSeqStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(selectPIDForIDStmt): %w", err)
	}
	t.searchMail, err = db.Prepare(searchMailStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(selectPIDForIDStmt): %w", err)
	}
	t.createMail, err = db.Prepare(insertMailStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(insertMailStmt): %w", err)
	}
	t.updateMailFlags, err = db.Prepare(updateMailFlagsStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(updateMailSeenStmt): %w", err)
	}
	t.deleteMail, err = db.Prepare(deleteMailStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(deleteMailStmt): %w", err)
	}
	t.expungeMail, err = db.Prepare(expungeMailStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(expungeMailStmt): %w", err)
	}
	t.countMails, err = db.Prepare(selectMailCountStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(selectMailCountStmt): %w", err)
	}
	t.countUnseenMails, err = db.Prepare(selectMailUnseenStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(selectMailUnseenStmt): %w", err)
	}
	t.moveMail, err = db.Prepare(moveMailStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(moveMailStmt): %w", err)
	}
	return t, nil
}

func (t *TableMails) MailCreate(mailbox string, data []byte) (int, error) {
	var id int
	err := t.writer.Do(t.db, nil, func(txn *sql.Tx) error {
		return t.createMail.QueryRow(mailbox, data, time.Now().Unix()).Scan(&id)
	})
	return id, err
}

func (t *TableMails) MailSelect(mailbox string, id int) (int, *types.Mail, error) {
	var seq int
	var datetime int64
	mail := &types.Mail{}
	err := t.selectMail.QueryRow(mailbox, id).Scan(
		&seq, &mail.ID, &mail.Mail, &datetime,
		&mail.Seen, &mail.Answered, &mail.Flagged, &mail.Deleted,
	)
	mail.Date = time.Unix(datetime, 0)
	return seq, mail, err
}

func (t *TableMails) MailSearch(mailbox string) ([]uint32, error) {
	var ids []uint32
	rows, err := t.searchMail.Query(mailbox)
	if err != nil {
		return nil, fmt.Errorf("t.searchMail.Query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uint32
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (t *TableMails) MailNextID(mailbox string) (int, error) {
	var id int
	err := t.selectMailNextID.QueryRow(mailbox).Scan(&id)
	return id, err
}

func (t *TableMails) MailIDForSeq(mailbox string, seq int) (int, error) {
	var id int
	err := t.selectIDForSeq.QueryRow(mailbox, seq).Scan(&id)
	return id, err
}

func (t *TableMails) MailUnseen(mailbox string) (int, error) {
	var unseen int
	err := t.countUnseenMails.QueryRow(mailbox).Scan(&unseen)
	return unseen, err
}

func (t *TableMails) MailUpdateFlags(mailbox string, id int, seen, answered, flagged, deleted bool) error {
	return t.writer.Do(t.db, nil, func(txn *sql.Tx) error {
		_, err := t.updateMailFlags.Exec(seen, answered, flagged, deleted, mailbox, id)
		return err
	})
}

func (t *TableMails) MailDelete(mailbox string, id int) error {
	return t.writer.Do(t.db, nil, func(txn *sql.Tx) error {
		_, err := t.deleteMail.Exec(mailbox, id)
		return err
	})
}

func (t *TableMails) MailExpunge(mailbox string) error {
	return t.writer.Do(t.db, nil, func(txn *sql.Tx) error {
		_, err := t.expungeMail.Exec(mailbox)
		return err
	})
}

func (t *TableMails) MailCount(mailbox string) (int, error) {
	var count int
	err := t.countMails.QueryRow(mailbox).Scan(&count)
	return count, err
}

func (t *TableMails) MailMove(mailbox string, id int, destination string) error {
	return t.writer.Do(t.db, nil, func(txn *sql.Tx) error {
		_, err := t.moveMail.Exec(destination, mailbox, id)
		return err
	})
}
