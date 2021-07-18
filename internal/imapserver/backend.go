/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package imapserver

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/storage"
	"github.com/neilalexander/yggmail/internal/utils"
)

type Backend struct {
	Config  *config.Config
	Log     *log.Logger
	Storage storage.Storage
	Server  *IMAPServer
}

func (b *Backend) Login(conn *imap.ConnInfo, username, password string) (backend.User, error) {
	// If our username is email-like, then take just the localpart
	if pk, err := utils.ParseAddress(username); err == nil {
		if !pk.Equal(b.Config.PublicKey) {
			b.Log.Println("Failed to authenticate IMAP user due to wrong domain", pk, b.Config.PublicKey)
			return nil, fmt.Errorf("failed to authenticate: wrong domain in username")
		}
	}
	username = hex.EncodeToString(b.Config.PublicKey)
	if authed, err := b.Storage.ConfigTryPassword(password); err != nil {
		b.Log.Printf("Failed to authenticate IMAP user %q due to error: %s", username, err)
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	} else if !authed {
		b.Log.Printf("Failed to authenticate IMAP user %q\n", username)
		return nil, backend.ErrInvalidCredentials
	}
	defer b.Log.Printf("Authenticated IMAP user from %s as %q\n", conn.RemoteAddr.String(), username)
	user := &User{
		backend:  b,
		username: username,
		conn:     conn,
	}
	return user, nil
}

/*
func (b *Backend) NotifyNew(id int) error {
	b.Server.server.ForEachConn(func(conn server.Conn) {
		notify := false
		for _, cap := range conn.Capabilities() {
			if cap == "NOTIFY" {
				notify = true
			}
		}
		if !notify {
			return
		}
		conn.WaitReady()
		conn.WriteResp()
	})
	return nil
}
*/
