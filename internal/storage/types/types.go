/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

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

type QueuedMail struct {
	ID   int
	From string
	Rcpt string
}
