/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package smtpsender

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"sync"
	"time"
	"strings"

	"github.com/emersion/go-smtp"
	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/storage"
	"github.com/neilalexander/yggmail/internal/storage/types"
	"github.com/neilalexander/yggmail/internal/transport"
	"github.com/neilalexander/yggmail/internal/utils"
	"go.uber.org/atomic"
)

type Queues struct {
	Config    *config.Config
	Log       *log.Logger
	Transport transport.Transport
	Storage   storage.Storage
	queues    sync.Map // servername -> *Queue
}

func NewQueues(config *config.Config, log *log.Logger, transport transport.Transport, storage storage.Storage) *Queues {
	qs := &Queues{
		Config:    config,
		Log:       log,
		Transport: transport,
		Storage:   storage,
	}
	time.AfterFunc(time.Second*5, qs.manager)
	return qs
}

func (qs *Queues) manager() {
	destinations, err := qs.Storage.QueueListDestinations()
	if err != nil {
		return
	}
	for _, destination := range destinations {
		_, _ = qs.queueFor(destination)
	}
	time.AfterFunc(time.Minute, qs.manager)
}

// TODO: Make this configurable, idk
const EXPECTED_DOMAIN = "yggmail.com"

func (qs *Queues) fixUp(incoming string) (string, error) {
	qs.Log.Printf("incoming: %s", incoming)

	var newAddr = ""
	var els = strings.Split(newAddr, "@")
	if len(els) != 2 {
		return "", fmt.Errorf("email address '%s' is invalid as it has too many @ symbols", incoming)
	}

	var id = els[0]
	var domain = els[1]

	if(domain != EXPECTED_DOMAIN) {
		var newDomain = EXPECTED_DOMAIN
		qs.Log.Printf("Had to apply fixup to domain: %s -> %s", domain, newDomain)
		domain = newDomain
	}

	newAddr = fmt.Sprintf("%s@%s", id, domain)
	qs.Log.Printf("Final calculated address: '%s'", newAddr)
	return newAddr, nil
}

func (qs *Queues) QueueFor(from string, rcpts []string, content []byte) error {
	var (
		localRecipients  int
		remoteRecipients []types.QueueRecipient
	)
	for _, rcpt := range rcpts {
		// Check if `rcpt` needs any fix up
		// and then apply them. <id>@yggmail* -> <id>@yggmail.com
		if rcpt, e := qs.fixUp(rcpt); e != nil {
			qs.Log.Printf("error shimming '%s': %s", rcpt, e)
			return e
		}
	
		addr, err := mail.ParseAddress(rcpt)
		if err != nil {
			return fmt.Errorf("mail.ParseAddress: %w", err)
		}
		pk, err := utils.ParseAddress(addr.Address)
		if err != nil {
			return fmt.Errorf("parseAddress: %w", err)
		}
		host := hex.EncodeToString(pk)
		if host == hex.EncodeToString(qs.Config.PublicKey) {
			localRecipients++
			continue
		}
		remoteRecipients = append(remoteRecipients, types.QueueRecipient{
			Destination: host,
			Address:     rcpt,
		})
	}

	if err := qs.Storage.QueueCreate(
		from, remoteRecipients, localRecipients, content,
	); err != nil {
		return fmt.Errorf("qs.Storage.QueueCreate: %w", err)
	}
	for _, rcpt := range remoteRecipients {
		_, _ = qs.queueFor(rcpt.Destination)
	}

	return nil
}

func (qs *Queues) queueFor(server string) (*Queue, error) {
	v, _ := qs.queues.LoadOrStore(server, &Queue{
		queues:      qs,
		destination: server,
	})
	q, ok := v.(*Queue)
	if !ok {
		return nil, fmt.Errorf("type assertion error")
	}
	if q.running.CompareAndSwap(false, true) {
		go q.run()
	}
	return q, nil
}

type Queue struct {
	queues      *Queues
	destination string
	running     atomic.Bool
}

func (q *Queue) run() {
	defer q.running.Store(false)

	refs, err := q.queues.Storage.QueueMailIDsForDestination(q.destination)
	if err != nil {
		q.queues.Log.Println("Error with queue:", err)
	}
	defer q.queues.Storage.MailExpunge("Outbox") // nolint:errcheck

	for _, ref := range refs {
		if ref.Mailbox != "Outbox" {
			if err := q.queues.Storage.QueueDeleteDestinationForID(
				q.destination, ref.Mailbox, ref.ID,
			); err != nil {
				q.queues.Log.Println("Failed to clean stale queue destination for ID", ref.ID, "due to error:", err)
			}
			continue
		}
		_, mail, err := q.queues.Storage.MailSelect("Outbox", ref.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if err = q.queues.Storage.QueueDeleteDestinationForID(
					q.destination, ref.Mailbox, ref.ID,
				); err != nil {
					q.queues.Log.Println("Failed delete queue destination for ID", ref.ID, "due to error:", err)
				}
			} else {
				q.queues.Log.Println("Failed to get mail", ref.ID, "due to error:", err)
			}
			continue
		}

		q.queues.Log.Println("Sending mail from", ref.From, "to", q.destination)

		if err := func() error {
			conn, err := q.queues.Transport.Dial(q.destination)
			if err != nil {
				return fmt.Errorf("q.queues.Transport.Dial: %w", err)
			}
			defer conn.Close() // nolint:errcheck

			client, err := smtp.NewClient(conn, q.destination)
			if err != nil {
				return fmt.Errorf("smtp.NewClient: %w", err)
			}
			defer client.Close() // nolint:errcheck

			if err := client.Hello(hex.EncodeToString(q.queues.Config.PublicKey)); err != nil {
				q.queues.Log.Println("Remote server", q.destination, "did not accept HELLO:", err)
				return fmt.Errorf("client.Hello: %w", err)
			}

			if err := client.Mail(ref.From, nil); err != nil {
				q.queues.Log.Println("Remote server", q.destination, "did not accept MAIL:", err)
				return fmt.Errorf("client.Mail: %w", err)
			}

			if err := client.Rcpt(ref.Rcpt); err != nil {
				q.queues.Log.Println("Remote server", q.destination, "did not accept RCPT:", err)
				return fmt.Errorf("client.Rcpt: %w", err)
			}

			writer, err := client.Data()
			if err != nil {
				return fmt.Errorf("client.Data: %w", err)
			}

			if _, err := writer.Write(mail.Mail); err != nil {
				_ = writer.Close()
				return fmt.Errorf("writer.Write: %w", err)
			}
			if err := writer.Close(); err != nil {
				return fmt.Errorf("writer.Close: %w", err)
			}

			if err := q.queues.Storage.QueueMarkDelivered(q.destination, ref.ID); err != nil {
				return fmt.Errorf("q.queues.Storage.QueueMarkDelivered: %w", err)
			}

			return nil
		}(); err != nil {
			q.queues.Log.Println("Will retry sending to", q.destination, "later due to error:", err)
			// TODO: Send a mail to the inbox on the first instance?
		} else {
			q.queues.Log.Println("Sent mail from", ref.From, "to", q.destination)
		}
	}
}
