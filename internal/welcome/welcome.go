package welcome

import (
	"bytes"
	"fmt"
	"github.com/emersion/go-message"
	"github.com/neilalexander/yggmail/internal/storage"
	"log"
)

const (
	WEBSITE_URL = "https://github.com/neilalexander/yggmail"
	CODE_URL    = "https://github.com/neilalexander/yggmail"
	WELCOME_SUBJECT = "Welcome to Yggmail!"
	WELCOME_BODY = `
	<html>
	<body>
	<h1>Welcome to <i>YggMail!</i></h1>
	<br>
	
	<p>Hey <b>%s</b>!</p>
	
	<p>We'd like to welcome you to Yggmail!</p>
	
	<p>You're about to embark in both a revolution and an
	evolution as you know it. The revolution is that this
	mailing system uses the new and experimental Yggdrasil
	internet routing system, the evolution is that it's
	good old email as you know it.</p>
	
	<br>
	<p>Want to learn more? See the <a href="%s">website</a></p>
	
	<p>Thinking of contributing; we'd be more than happy
	to work together. Our project is hosted on <a href="%s">GitHub</a>.</p>
	</body>
	</html>
	`
)

func Onboard(user string, storage storage.Storage, log *log.Logger) {
	// Fetch onboarding status
	if f, e := storage.ConfigGet("onboarding_done"); e == nil {

		// If we haven't onboarded yet
		if len(f) == 0 {
			log.Printf("Performing onboarding...\n")

			// takes in addr and output writer
			welcomeMsg, e := welcomeMessageFor(user, log)
			if e != nil {
				log.Println("Failure to generate welcome message")
			}
			var welcomeId int
			if id, e := storage.MailCreate("INBOX", welcomeMsg); e != nil {
				log.Printf("Failed to store welcome message: %v\n", e)
				panic("See above")
			} else {
				welcomeId = id
			}

			if storage.MailUpdateFlags("INBOX", welcomeId, false, false, false, false) != nil {
				panic("Could not set flags on onboarding message")
			}

			// set flag to never do it again
			if storage.ConfigSet("onboarding_done", "true") != nil {
				panic("Error storing onboarding flag")
			}

			log.Printf("Onboarding done\n")
		} else {
			log.Printf("Onboarding not required\n")
		}
	} else {
		panic("Error fetching onboarding status")
	}

}

func welcomeMessageFor(yourYggMailAddr string, log *log.Logger) ([]byte, error) {
	var hdr = welcomeTo(yourYggMailAddr, log)

	var buff = bytes.NewBuffer([]byte{})

	// writer writes to underlying writer (our buffer)
	// but returns a writer just for the body part
	// (it will encode header to underlying writer
	// first)
	msgWrt, e := message.CreateWriter(buff, hdr)
	if e != nil {
		return nil, e
	}

	var formattedBody = fmt.Sprintf(WELCOME_BODY, yourYggMailAddr, WEBSITE_URL, CODE_URL)

	if _, e := msgWrt.Write([]byte(formattedBody)); e != nil {
		return nil, e
	}

	return buff.Bytes(), nil
}

func welcomeTo(yourYggMailAddr string, log *log.Logger) message.Header {
	// header would be a nice preview of what to expect
	// of the message
	var welcomeHdr = message.Header{}
	welcomeHdr.Add("From", "Yggmail Team")
	welcomeHdr.Add("To", yourYggMailAddr+"@yggmail")
	welcomeHdr.Add("Subject", WELCOME_SUBJECT)
	welcomeHdr.Add("Content-Type", "text/html")

	log.Printf("Generated welcome mesg '%v'\n", welcomeHdr)
	return welcomeHdr
}
