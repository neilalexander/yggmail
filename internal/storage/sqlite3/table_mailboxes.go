package sqlite3

import (
	"database/sql"
	"fmt"
)

type TableMailboxes struct {
	db                      *sql.DB
	selectMailboxes         *sql.Stmt
	listMailboxes           *sql.Stmt
	listMailboxesSubscribed *sql.Stmt
	createMailbox           *sql.Stmt
	renameMailbox           *sql.Stmt
	deleteMailbox           *sql.Stmt
	subscribeMailbox        *sql.Stmt
}

const mailboxesSchema = `
	CREATE TABLE IF NOT EXISTS mailboxes (
		mailbox 	TEXT NOT NULL DEFAULT('INBOX'),
		subscribed  BOOLEAN NOT NULL DEFAULT 1,
		PRIMARY 	KEY(mailbox)
	);
`

const mailboxesList = `
	SELECT mailbox FROM mailboxes
`

const mailboxesListSubscribed = `
	SELECT mailbox FROM mailboxes WHERE subscribed = 1
`

const mailboxesSelect = `
	SELECT mailbox FROM mailboxes WHERE mailbox = $1
`

const mailboxesCreate = `
	INSERT OR IGNORE INTO mailboxes (mailbox) VALUES($1)
`

const mailboxesRename = `
	UPDATE mailboxes SET mailbox = $1 WHERE mailbox = $2
`

const mailboxesDelete = `
	DELETE FROM mailboxes WHERE mailbox = $1
`

const mailboxesSubscribe = `
	UPDATE mailboxes SET subscribed = $1 WHERE mailbox = $2
`

func NewTableMailboxes(db *sql.DB) (*TableMailboxes, error) {
	t := &TableMailboxes{
		db: db,
	}
	_, err := db.Exec(mailboxesSchema)
	if err != nil {
		return nil, fmt.Errorf("db.Exec: %w", err)
	}
	t.listMailboxes, err = db.Prepare(mailboxesList)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(mailboxesCreate): %w", err)
	}
	t.listMailboxesSubscribed, err = db.Prepare(mailboxesListSubscribed)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(mailboxesCreate): %w", err)
	}
	t.selectMailboxes, err = db.Prepare(mailboxesSelect)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(mailboxesSelect): %w", err)
	}
	t.createMailbox, err = db.Prepare(mailboxesCreate)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(mailboxesCreate): %w", err)
	}
	t.deleteMailbox, err = db.Prepare(mailboxesDelete)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(mailboxesDelete): %w", err)
	}
	t.renameMailbox, err = db.Prepare(mailboxesRename)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(mailboxesRename): %w", err)
	}
	t.subscribeMailbox, err = db.Prepare(mailboxesSubscribe)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(mailboxesSubscribe): %w", err)
	}
	return t, nil
}

func (t *TableMailboxes) MailboxList(onlySubscribed bool) ([]string, error) {
	stmt := t.listMailboxes
	if onlySubscribed {
		stmt = t.listMailboxesSubscribed
	}
	rows, err := stmt.Query()
	if err != nil {
		return nil, fmt.Errorf("t.listMailboxes.Query: %w", err)
	}
	defer rows.Close()
	var mailboxes []string
	for rows.Next() {
		var mailbox string
		if err := rows.Scan(&mailbox); err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		mailboxes = append(mailboxes, mailbox)
	}
	return mailboxes, nil
}

func (t *TableMailboxes) MailboxSelect(mailbox string) (bool, error) {
	row := t.selectMailboxes.QueryRow(mailbox)
	if err := row.Err(); err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("row.Err: %w", err)
	} else if err == sql.ErrNoRows {
		return false, nil
	}
	var got string
	if err := row.Scan(&got); err != nil {
		return false, fmt.Errorf("row.Scan: %w", err)
	}
	return mailbox == got, nil
}

func (t *TableMailboxes) MailboxCreate(name string) error {
	_, err := t.createMailbox.Exec(name)
	return err
}

func (t *TableMailboxes) MailboxRename(old, new string) error {
	_, err := t.renameMailbox.Exec(old, new)
	return err
}

func (t *TableMailboxes) MailboxDelete(name string) error {
	_, err := t.deleteMailbox.Exec(name)
	return err
}

func (t *TableMailboxes) MailboxSubscribe(name string, subscribed bool) error {
	sn := 1
	if !subscribed {
		sn = 0
	}
	_, err := t.subscribeMailbox.Exec(sn, name)
	return err
}
