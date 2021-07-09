package smtpsender

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/mail"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/storage"
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
	go qs.manager()
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
	time.AfterFunc(time.Minute*5, qs.manager)
}

func (qs *Queues) QueueFor(from string, rcpts []string, content []byte) error {
	pid, err := qs.Storage.MailCreate("Outbox", content)
	if err != nil {
		return fmt.Errorf("q.queues.Storage.MailCreate: %w", err)
	}

	for _, rcpt := range rcpts {
		addr, err := mail.ParseAddress(rcpt)
		if err != nil {
			return fmt.Errorf("mail.ParseAddress: %w", err)
		}
		pk, err := utils.ParseAddress(addr.Address)
		if err != nil {
			return fmt.Errorf("parseAddress: %w", err)
		}
		host := hex.EncodeToString(pk)

		if err := qs.Storage.QueueInsertDestinationForID(host, pid, from, rcpt); err != nil {
			return fmt.Errorf("qs.Storage.QueueInsertDestinationForID: %w", err)
		}

		_, _ = qs.queueFor(host)
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
	if q.running.CAS(false, true) {
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
	defer q.queues.Storage.MailExpunge("Outbox") // nolint:errcheck

	refs, err := q.queues.Storage.QueueMailIDsForDestination(q.destination)
	if err != nil {
		q.queues.Log.Println("Error with queue:", err)
	}

	q.queues.Log.Println("There are", len(refs), "mail(s) queued for", q.destination)

	for _, ref := range refs {
		_, mail, err := q.queues.Storage.MailSelect("Outbox", ref.ID)
		if err != nil {
			q.queues.Log.Println("Failed to get mail", ref.ID, "due to error:", err)
			continue
		}

		q.queues.Log.Println("Sending mail from", ref.From, "to", q.destination)

		if err := func() error {
			conn, err := q.queues.Transport.Dial(q.destination)
			if err != nil {
				q.queues.Log.Println("Failed to dial destination", q.destination, "due to error:", err)
				return fmt.Errorf("q.queues.Transport.Dial: %w", err)
			}
			defer conn.Close()

			client, err := smtp.NewClient(conn, q.destination)
			if err != nil {
				return fmt.Errorf("smtp.NewClient: %w", err)
			}
			defer client.Close()

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
			defer writer.Close()

			if _, err := writer.Write(mail.Mail); err != nil {
				return fmt.Errorf("writer.Write: %w", err)
			}

			if err := q.queues.Storage.QueueDeleteDestinationForID(q.destination, ref.ID); err != nil {
				return fmt.Errorf("q.queues.Storage.QueueDeleteDestinationForID: %w", err)
			}

			if remaining, err := q.queues.Storage.QueueSelectIsMessagePendingSend("Outbox", ref.ID); err != nil {
				return fmt.Errorf("q.queues.Storage.QueueSelectIsMessagePendingSend: %w", err)
			} else if !remaining {
				return q.queues.Storage.MailDelete("Outbox", ref.ID)
			}

			return nil
		}(); err != nil {
			q.queues.Log.Println("Failed to send message:", err, "- will retry")
			// TODO: Send a mail to the inbox on the first instance?
		} else {
			q.queues.Log.Println("Sent mail from", ref.From, "to", q.destination)
		}
	}
}
