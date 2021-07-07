package sqlite3

import (
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type TableUsers struct {
	db          *sql.DB
	getPassword *sql.Stmt
	insertUser  *sql.Stmt
}

const usersSchema = `
	CREATE TABLE IF NOT EXISTS users (
		username 	TEXT NOT NULL,
		password 	TEXT NOT NULL,
		PRIMARY KEY(username)
	);
`

const usersGetPassword = `
	SELECT password FROM users WHERE username = $1
`

const usersInsertUser = `
	INSERT INTO users (username, password) VALUES($1, $2)
`

func NewTableUsers(db *sql.DB) (*TableUsers, error) {
	t := &TableUsers{
		db: db,
	}
	_, err := db.Exec(usersSchema)
	if err != nil {
		return nil, fmt.Errorf("db.Exec: %w", err)
	}
	t.getPassword, err = db.Prepare(usersGetPassword)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(getPassword): %w", err)
	}
	t.insertUser, err = db.Prepare(usersInsertUser)
	if err != nil {
		return nil, fmt.Errorf("db.Prepare(usersInsertUser): %w", err)
	}
	return t, nil
}

func (t *TableUsers) CreateUser(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bcrypt.GenerateFromPassword: %w", err)
	}
	if _, err := t.insertUser.Exec(username, hash); err != nil {
		return fmt.Errorf("t.insertUser.Exec: %w", err)
	}
	return nil
}

func (t *TableUsers) TryAuthenticate(username, password string) (bool, error) {
	var dbPasswordHash []byte
	err := t.getPassword.QueryRow(username).Scan(&dbPasswordHash)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = bcrypt.CompareHashAndPassword(dbPasswordHash, []byte(password))
	if err == nil {
		return true, nil
	}
	return false, err
}
