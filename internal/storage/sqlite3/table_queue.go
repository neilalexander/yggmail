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

	"github.com/neilalexander/yggmail/internal/storage/types"
)

type TableQueue struct {
	db                              *sql.DB
	writer                          *Writer
	queueSelectDestinations         *sql.Stmt
	queueSelectIDsForDestination    *sql.Stmt
	queueInsertDestinationForID     *sql.Stmt
	queueDeleteIDForDestination     *sql.Stmt
	queueSelectIsMessagePendingSend *sql.Stmt
}

const queueSchema = `
	CREATE TABLE IF NOT EXISTS queue (
		destination TEXT NOT NULL,
		mailbox TEXT NOT NULL,
		id INTEGER NOT NULL,
		mail TEXT NOT NULL,
		rcpt TEXT NOT NULL,
		PRIMARY KEY (destination, mailbox, id),
		FOREIGN KEY (mailbox, id) REFERENCES mails(mailbox, id) ON DELETE CASCADE ON UPDATE CASCADE
	);
`

const queueSelectDestinationsStmt = `
	SELECT DISTINCT destination FROM queue
`

const queueSelectIDsForDestinationStmt = `
	SELECT id, mail, rcpt FROM queue WHERE destination = $1
	ORDER BY id DESC
`

const queueInsertDestinationForIDStmt = `
	INSERT INTO queue (destination, mailbox, id, mail, rcpt) VALUES($1, $2, $3, $4, $5)
`

const deleteDestinationForIDStmt = `
    DELETE FROM queue WHERE destination = $1 AND mailbox = $2 AND id = $3
`

const queueSelectIsMessagePendingSendStmt = `
	SELECT COUNT(*) FROM queue WHERE mailbox = $1 AND id = $2
`

func NewTableQueue(db *sql.DB, writer *Writer) (*TableQueue, error) {
	t := &TableQueue{
		db:     db,
		writer: writer,
	}
	_, err := db.Exec(queueSchema)
	if err != nil {
		return nil, fmt.Errorf("db.Exec: %w", err)
	}
	t.queueSelectDestinations, err = db.Prepare(queueSelectDestinationsStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(queueSelectDestinationsStmt): %w", err)
	}
	t.queueSelectIDsForDestination, err = db.Prepare(queueSelectIDsForDestinationStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(queueSelectIDsForDestinationStmt): %w", err)
	}
	t.queueInsertDestinationForID, err = db.Prepare(queueInsertDestinationForIDStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(queueInsertDestinationForIDStmt): %w", err)
	}
	t.queueDeleteIDForDestination, err = db.Prepare(deleteDestinationForIDStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(deleteDestinationForIDStmt): %w", err)
	}
	t.queueSelectIsMessagePendingSend, err = db.Prepare(queueSelectIsMessagePendingSendStmt)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(queueSelectIsMessagePendingSendStmt): %w", err)
	}
	return t, nil
}

func (t *TableQueue) QueueListDestinations() ([]string, error) {
	rows, err := t.queueSelectDestinations.Query()
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("t.queueSelectDestinations.Query: %w", err)
	}
	defer rows.Close()
	var destinations []string
	for rows.Next() {
		var destination string
		if err := rows.Scan(&destination); err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		destinations = append(destinations, destination)
	}
	return destinations, nil
}

func (t *TableQueue) QueueMailIDsForDestination(destination string) ([]types.QueuedMail, error) {
	rows, err := t.queueSelectIDsForDestination.Query(destination)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("t.queueSelectDestinations.Query: %w", err)
	}
	defer rows.Close()
	var ids []types.QueuedMail
	for rows.Next() {
		var id int
		var from, rcpt string
		if err := rows.Scan(&id, &from, &rcpt); err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		ids = append(ids, types.QueuedMail{
			ID:   id,
			From: from,
			Rcpt: rcpt,
		})
	}
	return ids, nil
}

func (t *TableQueue) QueueInsertDestinationForID(destination string, id int, from, rcpt string) error {
	return t.writer.Do(t.db, nil, func(txn *sql.Tx) error {
		_, err := t.queueInsertDestinationForID.Exec(destination, "Outbox", id, from, rcpt)
		return err
	})
}

func (t *TableQueue) QueueDeleteDestinationForID(destination string, id int) error {
	return t.writer.Do(t.db, nil, func(txn *sql.Tx) error {
		_, err := t.queueDeleteIDForDestination.Exec(destination, "Outbox", id)
		return err
	})
}

func (t *TableQueue) QueueSelectIsMessagePendingSend(mailbox string, id int) (bool, error) {
	row := t.queueSelectIsMessagePendingSend.QueryRow(mailbox, id)
	if err := row.Err(); err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("row.Err: %w", err)
	} else if err == sql.ErrNoRows {
		return false, nil
	}
	var count int
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("row.Scan: %w", err)
	}
	return count > 0, nil
}
