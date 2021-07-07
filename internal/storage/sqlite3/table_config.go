package sqlite3

import (
	"database/sql"
	"fmt"
)

type TableConfig struct {
	db  *sql.DB
	get *sql.Stmt
	set *sql.Stmt
}

const configSchema = `
	CREATE TABLE IF NOT EXISTS config (
		key 		TEXT NOT NULL,
		value 		TEXT NOT NULL,
		PRIMARY KEY(key)
	);
`

const configGet = `
	SELECT value FROM config WHERE key = $1
`

const configSet = `
	INSERT OR REPLACE INTO config (key, value) VALUES($1, $2)
`

func NewTableConfig(db *sql.DB) (*TableConfig, error) {
	t := &TableConfig{
		db: db,
	}
	_, err := db.Exec(configSchema)
	if err != nil {
		return nil, fmt.Errorf("db.Exec: %w", err)
	}
	t.get, err = db.Prepare(configGet)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(get): %w", err)
	}
	t.set, err = db.Prepare(configSet)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(set): %w", err)
	}
	return t, nil
}

func (t *TableConfig) ConfigGet(key string) (string, error) {
	var value string
	err := t.get.QueryRow(key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (t *TableConfig) ConfigSet(key, value string) error {
	_, err := t.set.Exec(key, value)
	return err
}
