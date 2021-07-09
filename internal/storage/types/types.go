package types

import "time"

type Mail struct {
	Mailbox  string
	ID       int
	Mail     []byte
	Date     time.Time
	Seen     bool
	Answered bool
	Flagged  bool
	Deleted  bool
}
