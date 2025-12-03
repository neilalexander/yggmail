package welcome;

import "fmt";
import "github.com/emersion/go-message/textproto";

var welcome_subject string = "Welcome to YggMail!";
var welcome_body string =
`
We'd like to welcome you to YggMail!

You're about to embark in both a revolution and an
evolution as you know it. The revolution is that this
mailing system uses the new and experimental Yggdrasil
internet routing system, the evolution is that it's
good old email as you know it.
`;

var fakeMail_id = 69_420;

func welcomeTo(yourYggMailAddr string) textproto.Header {
	// var f *imap.Message = imap.NewMessage(fakeMail_id, nil);


	// header would be a nice preview of what to expect
	// of the message
	var welcome_hdr = textproto.Header{};
	welcome_hdr.Add("From", "YggMail Team");
	welcome_hdr.Add("To", yourYggMailAddr+"@yggmail");
	welcome_hdr.Add("Subject", welcome_subject);

	fmt.Printf("Generated welcome mesg '%v'\n", welcome_hdr);
	return welcome_hdr;
}
