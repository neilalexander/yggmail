package sqlite3

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/atomic"
)

type SQLite3Storage struct {
	*TableConfig
	*TableMailboxes
	*TableMails
	*TableQueue
	writer *Writer
}

func NewSQLite3StorageStorage(filename string) (*SQLite3Storage, error) {
	db, err := sql.Open("sqlite3", "file:"+filename+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	s := &SQLite3Storage{
		writer: &Writer{
			todo: make(chan writerTask),
		},
	}
	s.TableConfig, err = NewTableConfig(db, s.writer)
	if err != nil {
		return nil, fmt.Errorf("NewTableConfig: %w", err)
	}
	s.TableMailboxes, err = NewTableMailboxes(db, s.writer)
	if err != nil {
		return nil, fmt.Errorf("NewTableMailboxes: %w", err)
	}
	s.TableMails, err = NewTableMails(db, s.writer)
	if err != nil {
		return nil, fmt.Errorf("NewTableMails: %w", err)
	}
	s.TableQueue, err = NewTableQueue(db, s.writer)
	if err != nil {
		return nil, fmt.Errorf("NewTableQueue: %w", err)
	}
	return s, nil
}

type Writer struct {
	running atomic.Bool
	todo    chan writerTask
}

type writerTask struct {
	db   *sql.DB
	txn  *sql.Tx
	f    func(txn *sql.Tx) error
	wait chan error
}

func (w *Writer) Do(db *sql.DB, txn *sql.Tx, f func(txn *sql.Tx) error) error {
	if !w.running.Load() {
		go w.run()
	}
	task := writerTask{
		db:   db,
		txn:  txn,
		f:    f,
		wait: make(chan error, 1),
	}
	w.todo <- task
	return <-task.wait
}

func (w *Writer) run() {
	if !w.running.CAS(false, true) {
		return
	}
	defer w.running.Store(false)
	for task := range w.todo {
		if task.db != nil && task.txn != nil {
			task.wait <- task.f(task.txn)
		} else if task.db != nil && task.txn == nil {
			func() {
				txn, err := task.db.Begin()
				if err != nil {
					return
				}
				err = task.f(txn)
				task.wait <- err
				if err == nil {
					_ = txn.Commit()
				} else {
					_ = txn.Rollback()
				}
			}()
		} else {
			task.wait <- task.f(nil)
		}
		close(task.wait)
	}
}
