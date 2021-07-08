package smtpsender

import (
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/neilalexander/yggmail/internal/config"
	"github.com/neilalexander/yggmail/internal/transport"
	"go.uber.org/atomic"
)

type Queues struct {
	Config    *config.Config
	Log       *log.Logger
	Transport transport.Transport
	queues    sync.Map // servername -> *Queue
}

func NewQueues(config *config.Config, log *log.Logger, transport transport.Transport) *Queues {
	return &Queues{
		Config:    config,
		Log:       log,
		Transport: transport,
	}
}

func (qs *Queues) QueueFor(server string) (*Queue, error) {
	v, _ := qs.queues.LoadOrStore(server, &Queue{
		queues:      qs,
		destination: server,
		fifo:        newFIFOQueue(),
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
	backoff     atomic.Int64
	fifo        *fifoQueue
}

func (q *Queue) Queue(mail *QueuedMail) error {
	q.fifo.push(mail)
	if q.running.CAS(false, true) {
		go q.run()
	}
	return nil
}

func (q *Queue) run() {
	defer q.running.Store(false)
	for {
		select {
		case <-q.fifo.wait():
		case <-time.After(time.Second * 10):
			return
		}

		item, ok := q.fifo.pop()
		if !ok {
			continue
		}
		mail, ok := item.(*QueuedMail)
		if !ok {
			continue
		}
		q.queues.Log.Println("Processing mail from", mail.From, "to", mail.Destination)

		if err := func() error {
			conn, err := q.queues.Transport.Dial(q.destination)
			if err != nil {
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

			q.backoff.Store(0)

			if err := client.Mail(mail.From, nil); err != nil {
				q.queues.Log.Println("Remote server", q.destination, "did not accept MAIL:", err)
				return fmt.Errorf("client.Mail: %w", err)
			}

			if err := client.Rcpt(mail.Rcpt); err != nil {
				q.queues.Log.Println("Remote server", q.destination, "did not accept RCPT:", err)
				return fmt.Errorf("client.Rcpt: %w", err)
			}

			writer, err := client.Data()
			if err != nil {
				return fmt.Errorf("client.Data: %w", err)
			}
			defer writer.Close()

			if _, err := writer.Write(mail.Content); err != nil {
				return fmt.Errorf("writer.Write: %w", err)
			}

			return nil
		}(); err != nil {
			retry := time.Second * time.Duration(math.Exp2(float64(q.backoff.Inc())))
			q.queues.Log.Println("Queue error:", err, "- will retry in", retry)
			time.Sleep(retry)
		} else {
			q.queues.Log.Println("Sent mail from", mail.From, "to", mail.Destination)
		}
	}
}

type QueuedMail struct {
	From        string // mail address
	Rcpt        string // mail addresses
	Destination string // server name
	Content     []byte
}
