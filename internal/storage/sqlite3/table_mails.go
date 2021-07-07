package sqlite3

import (
	"database/sql"
	"fmt"
	"time"
)

type TableMails struct {
	db               *sql.DB
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
}

const mailsSchema = `
	CREATE TABLE IF NOT EXISTS mails (
		username 	TEXT NOT NULL,
		mailbox 	TEXT NOT NULL,
		id			INTEGER NOT NULL DEFAULT 1,
		mail 		BLOB NOT NULL,
		datetime    INTEGER NOT NULL,
		seen		BOOLEAN NOT NULL DEFAULT 0, -- the mail has been read
		answered	BOOLEAN NOT NULL DEFAULT 0, -- the mail has been replied to
		flagged		BOOLEAN NOT NULL DEFAULT 0, -- the mail has been flagged for later attention
		deleted		BOOLEAN NOT NULL DEFAULT 0, -- the email is marked for deletion at next EXPUNGE
		PRIMARY KEY(username, mailbox, id),
		FOREIGN KEY (username, mailbox) REFERENCES mailboxes(username, mailbox) ON DELETE CASCADE ON UPDATE CASCADE
	);

	DROP VIEW IF EXISTS inboxes;
	CREATE VIEW IF NOT EXISTS inboxes AS SELECT * FROM (
		SELECT ROW_NUMBER() OVER (PARTITION BY username, mailbox) AS seq, * FROM mails
	)
	ORDER BY username, mailbox, id;
`

const selectMailsStmt = `
	SELECT * FROM inboxes
	ORDER BY username, mailbox, id
`

const selectMailStmt = `
	SELECT seq, id, mail, datetime, seen, answered, flagged, deleted FROM inboxes
	WHERE username = $1 AND mailbox = $2 AND id = $3
	ORDER BY username, mailbox, id
`

const selectMailCountStmt = `
	SELECT COUNT(*) FROM mails WHERE username = $1 AND mailbox = $2
`

const selectMailUnseenStmt = `
	SELECT COUNT(*) FROM mails WHERE username = $1 AND mailbox = $2 AND seen = 0
`

const searchMailStmt = `
	SELECT id FROM mails
	WHERE username = $1 AND mailbox = $2
	ORDER BY username, mailbox, id
`

const insertMailStmt = `
	INSERT INTO mails (username, mailbox, id, mail, datetime) VALUES(
		$1, $2, (
			SELECT IFNULL(MAX(id)+1,1) AS id FROM mails
			WHERE username = $1 AND mailbox = $2
		), $3, $4
	)
	RETURNING id;
`

const selectIDForSeqStmt = `
	SELECT id FROM inboxes
	WHERE username = $1 AND mailbox = $2 AND seq = $3
`

const selectMailNextID = `
	SELECT IFNULL(MAX(id)+1,1) AS id FROM mails
	WHERE username = $1 AND mailbox = $2	
`

const updateMailFlagsStmt = `
	UPDATE mails SET seen = $1, answered = $2, flagged = $3, deleted = $4 WHERE username = $5 AND mailbox = $6 AND id = $7
`

const deleteMailStmt = `
	UPDATE mails SET deleted = 1 WHERE username = $1 AND mailbox = $2 AND id = $3
`

const expungeMailStmt = `
	DELETE FROM mails WHERE username = $1 AND mailbox = $2 AND deleted = 1
`

func NewTableMails(db *sql.DB) (*TableMails, error) {
	t := &TableMails{
		db: db,
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
	return t, nil
}

func (t *TableMails) MailCreate(user, mailbox string, data []byte) (int, error) {
	var id int
	err := t.createMail.QueryRow(user, mailbox, data, time.Now().Unix()).Scan(&id)
	return id, err
}

func (t *TableMails) MailSelect(user, mailbox string, id int) (int, int, []byte, bool, bool, bool, bool, time.Time, error) {
	var data []byte
	var seen, answered, flagged, deleted bool
	var ts int64
	var seq, pid int
	err := t.selectMail.QueryRow(user, mailbox, id).Scan(&seq, &pid, &data, &ts, &seen, &answered, &flagged, &deleted)
	return seq, pid, data, seen, answered, flagged, deleted, time.Unix(ts, 0), err
}

func (t *TableMails) MailSearch(user, mailbox string) ([]uint32, error) {
	var ids []uint32
	rows, err := t.searchMail.Query(user, mailbox)
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

func (t *TableMails) MailNextID(user, mailbox string) (int, error) {
	var id int
	err := t.selectMailNextID.QueryRow(user, mailbox).Scan(&id)
	return id, err
}

func (t *TableMails) MailIDForSeq(user, mailbox string, seq int) (int, error) {
	var id int
	err := t.selectIDForSeq.QueryRow(user, mailbox, seq).Scan(&id)
	return id, err
}

func (t *TableMails) MailUnseen(user, mailbox string) (int, error) {
	var unseen int
	err := t.countUnseenMails.QueryRow(user, mailbox).Scan(&unseen)
	return unseen, err
}

func (t *TableMails) MailUpdateFlags(user, mailbox string, id int, seen, answered, flagged, deleted bool) error {
	_, err := t.updateMailFlags.Exec(seen, answered, flagged, deleted, user, mailbox, id)
	return err
}

func (t *TableMails) MailDelete(user, mailbox, id string) error {
	_, err := t.deleteMail.Exec(user, mailbox, id)
	return err
}

func (t *TableMails) MailExpunge(user, mailbox string) error {
	_, err := t.expungeMail.Exec(user, mailbox)
	return err
}

func (t *TableMails) MailCount(user, mailbox string) (int, error) {
	var count int
	err := t.countMails.QueryRow(user, mailbox).Scan(&count)
	return count, err
}
