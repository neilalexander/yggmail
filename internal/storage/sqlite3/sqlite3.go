package sqlite3

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type SQLite3Storage struct {
	*TableConfig
	*TableMailboxes
	*TableMails
}

func NewSQLite3StorageStorage(filename string) (*SQLite3Storage, error) {
	db, err := sql.Open("sqlite3", "file:"+filename+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	s := &SQLite3Storage{}
	s.TableConfig, err = NewTableConfig(db)
	if err != nil {
		return nil, fmt.Errorf("NewTableConfig: %w", err)
	}
	s.TableMailboxes, err = NewTableMailboxes(db)
	if err != nil {
		return nil, fmt.Errorf("NewTableMailboxes: %w", err)
	}
	s.TableMails, err = NewTableMails(db)
	if err != nil {
		return nil, fmt.Errorf("NewTableMails: %w", err)
	}
	return s, nil
}
