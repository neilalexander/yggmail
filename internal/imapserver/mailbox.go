package imapserver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/backendutil"
	"github.com/emersion/go-message/textproto"
)

type Mailbox struct {
	backend *Backend
	name    string
	user    *User
}

func (mbox *Mailbox) getIDsFromSeqSet(uid bool, seqSet *imap.SeqSet) ([]int32, error) {
	var ids []int32
	for _, set := range seqSet.Set {
		if set.Stop == 0 {
			next, err := mbox.backend.Storage.MailNextID(mbox.name)
			if err != nil {
				return nil, fmt.Errorf("mbox.backend.Storage.MailNextID: %w", err)
			}
			set.Stop = uint32(next - 1)
		}
		for i := set.Start; i <= set.Stop; i++ {
			if !uid {
				pid, err := mbox.backend.Storage.MailIDForSeq(mbox.name, int(i))
				if err != nil {
					return nil, fmt.Errorf("mbox.backend.Storage.MailIDForSeq: %w", err)
				}
				ids = append(ids, int32(pid))
			} else {
				ids = append(ids, int32(i))
			}
		}
	}
	return ids, nil
}

func (mbox *Mailbox) Name() string {
	return mbox.name
}

func (mbox *Mailbox) Info() (*imap.MailboxInfo, error) {
	info := &imap.MailboxInfo{
		Attributes: []string{},
		Delimiter:  "/",
		Name:       mbox.name,
	}
	return info, nil
}

func (mbox *Mailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	status := imap.NewMailboxStatus(mbox.name, items)
	status.PermanentFlags = []string{
		"\\Seen", "\\Answered", "\\Flagged", "\\Deleted",
	}
	status.Flags = status.PermanentFlags

	for _, name := range items {
		switch name {
		case imap.StatusMessages:
			count, err := mbox.backend.Storage.MailCount(mbox.name)
			if err != nil {
				return nil, fmt.Errorf("mbox.backend.Storage.MailCount: %w", err)
			}
			status.Messages = uint32(count)

		case imap.StatusUidNext:
			id, err := mbox.backend.Storage.MailNextID(mbox.name)
			if err != nil {
				return nil, fmt.Errorf("mbox.backend.Storage.MailNextID: %w", err)
			}
			status.UidNext = uint32(id)

		case imap.StatusUidValidity:
			status.UidValidity = 1

		case imap.StatusRecent:
			status.Recent = 0 // TODO

		case imap.StatusUnseen:
			unseen, err := mbox.backend.Storage.MailUnseen(mbox.name)
			if err != nil {
				return nil, fmt.Errorf("mbox.backend.Storage.MailUnseen: %w", err)
			}
			status.Unseen = uint32(unseen)
		}
	}

	return status, nil
}

func (mbox *Mailbox) SetSubscribed(subscribed bool) error {
	return mbox.backend.Storage.MailboxSubscribe(mbox.name, subscribed)
}

func (mbox *Mailbox) Check() error {
	return nil
}

func (mbox *Mailbox) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)

	ids, err := mbox.getIDsFromSeqSet(uid, seqSet)
	if err != nil {
		return fmt.Errorf("mbox.getIDsFromSeqSet: %w", err)
	}

	for _, id := range ids {
		mseq, mid, body, seen, answered, flagged, deleted, datetime, err := mbox.backend.Storage.MailSelect(mbox.name, int(id))
		if err != nil {
			continue
		}

		fetched := imap.NewMessage(uint32(id), items)
		fetched.SeqNum = uint32(mseq)
		fetched.Uid = uint32(mid)

		get := func() (io.Reader, textproto.Header, error) {
			bodyreader := bufio.NewReader(bytes.NewReader(body))
			hdr, err := textproto.ReadHeader(bodyreader)
			if err != nil {
				return nil, textproto.Header{}, fmt.Errorf("textproto.ReadHeader: %w", err)
			}
			return bodyreader, hdr, err
		}

		for _, item := range items {
			switch item {
			case imap.FetchEnvelope:
				_, hdr, err := get()
				if err != nil {
					continue
				}
				if fetched.Envelope, err = backendutil.FetchEnvelope(hdr); err != nil {
					continue
				}

			case imap.FetchBody, imap.FetchBodyStructure:
				bodyreader, hdr, err := get()
				if err != nil {
					continue
				}
				if fetched.BodyStructure, err = backendutil.FetchBodyStructure(hdr, bodyreader, item == imap.FetchBodyStructure); err != nil {
					continue
				}

			case imap.FetchFlags:
				fetched.Flags = []string{}
				if seen {
					fetched.Flags = append(fetched.Flags, "\\Seen")
				}
				if answered {
					fetched.Flags = append(fetched.Flags, "\\Answered")
				}
				if flagged {
					fetched.Flags = append(fetched.Flags, "\\Flagged")
				}
				if deleted {
					fetched.Flags = append(fetched.Flags, "\\Deleted")
				}

			case imap.FetchInternalDate:
				fetched.InternalDate = datetime

			case imap.FetchRFC822Size:
				fetched.Size = uint32(len(body))

			case imap.FetchUid:
				fetched.Uid = uint32(id)

			default:
				section, err := imap.ParseBodySectionName(item)
				if err != nil {
					continue
				}
				bodyreader, hdr, err := get()
				if err != nil {
					continue
				}
				l, err := backendutil.FetchBodySection(hdr, bodyreader, section)
				if err != nil {
					continue
				}
				fetched.Body[section] = l
			}
		}

		ch <- fetched
	}

	return nil
}

func (mbox *Mailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	return mbox.backend.Storage.MailSearch(mbox.name)
}

func (mbox *Mailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	b, err := ioutil.ReadAll(body)
	if err != nil {
		return fmt.Errorf("b.ReadFrom: %w", err)
	}
	id, err := mbox.backend.Storage.MailCreate(mbox.name, b)
	if err != nil {
		return fmt.Errorf("mbox.backend.Storage.MailCreate: %w", err)
	}
	for _, flag := range flags {
		var seen, answered, flagged, deleted bool
		switch flag {
		case "\\Seen":
			seen = true
		case "\\Answered":
			answered = true
		case "\\Flagged":
			flagged = true
		case "\\Deleted":
			deleted = true
		}
		if err := mbox.backend.Storage.MailUpdateFlags(
			mbox.name, id, seen, answered, flagged, deleted,
		); err != nil {
			return err
		}
	}
	return nil
}

func (mbox *Mailbox) UpdateMessagesFlags(uid bool, seqSet *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	ids, err := mbox.getIDsFromSeqSet(uid, seqSet)
	if err != nil {
		return fmt.Errorf("mbox.getIDsFromSeqSet: %w", err)
	}

	for _, id := range ids {
		var seen, answered, flagged, deleted bool
		var mid int
		if op != imap.SetFlags {
			var err error
			_, mid, _, seen, answered, flagged, deleted, _, err = mbox.backend.Storage.MailSelect(mbox.name, int(id))
			if err != nil {
				return fmt.Errorf("mbox.backend.Storage.MailSelect: %w", err)
			}
		}
		for _, flag := range flags {
			switch flag {
			case "\\Seen":
				seen = op != imap.RemoveFlags
			case "\\Answered":
				answered = op != imap.RemoveFlags
			case "\\Flagged":
				flagged = op != imap.RemoveFlags
			case "\\Deleted":
				deleted = op != imap.RemoveFlags
			}
		}

		if err := mbox.backend.Storage.MailUpdateFlags(
			mbox.name, int(mid), seen, answered, flagged, deleted,
		); err != nil {
			return err
		}
	}
	return nil
}

func (mbox *Mailbox) CopyMessages(uid bool, seqSet *imap.SeqSet, destName string) error {
	if destName == "Outbox" {
		return fmt.Errorf("can't copy into Outbox as it is a protected folder")
	}

	ids, err := mbox.getIDsFromSeqSet(uid, seqSet)
	if err != nil {
		return fmt.Errorf("mbox.getIDsFromSeqSet: %w", err)
	}

	for _, id := range ids {
		_, _, body, seen, answered, flagged, deleted, _, err := mbox.backend.Storage.MailSelect(mbox.name, int(id))
		if err != nil {
			return fmt.Errorf("mbox.backend.Storage.MailSelect: %w", err)
		}
		pid, err := mbox.backend.Storage.MailCreate(destName, body)
		if err != nil {
			return fmt.Errorf("mbox.backend.Storage.MailCreate: %w", err)
		}
		if err = mbox.backend.Storage.MailUpdateFlags(mbox.name, pid, seen, answered, flagged, deleted); err != nil {
			return fmt.Errorf("mbox.backend.Storage.MailUpdateFlags: %w", err)
		}
	}
	return nil
}

func (mbox *Mailbox) Expunge() error {
	return mbox.backend.Storage.MailExpunge(mbox.name)
}
