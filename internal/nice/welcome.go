package welcome;

import "fmt";
// import "github.com/emersion/go-message/textproto";
import "github.com/emersion/go-message";
import "io";
import "bytes";

func giveReader() io.Reader {
	var p_r, p_o = io.Pipe();

	io.WriteString(p_o, welcome_body)

	return p_r;
}

// func MakeItTo(yourYggMailAddr string, to io.Writer) error {
	// var ent, e = makeIt0(yourYggMailAddr); // TODO: do error check
	// if e != nil {
		// return e
	// }
// 
	// return ent.WriteTo(to);
// }

func WelcomeMessageFor(yourYggMailAddr string) ([]byte, error) {
	var hdr message.Header = welcomeTo(yourYggMailAddr);

	var buff *bytes.Buffer = bytes.NewBuffer([]byte{})

	// writer writes to underlying writer (our buffer)
	// but returns a writer just for the body part
	// (it will encode header to underlying writer
	// first)
	msg_wrt, e := message.CreateWriter(buff, hdr);
	if e != nil {
		return nil, e
	}

	if _, e := msg_wrt.Write([]byte(welcome_body)); e != nil {
		return nil, e
	}
	// var ent, e = message.New(hdr, body_rdr)

	return buff.Bytes(), nil
}


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

func welcomeTo(yourYggMailAddr string) message.Header {
	// var f *imap.Message = imap.NewMessage(fakeMail_id, nil);


	// header would be a nice preview of what to expect
	// of the message
	var welcome_hdr = message.Header{};
	welcome_hdr.Add("From", "YggMail Team");
	welcome_hdr.Add("To", yourYggMailAddr+"@yggmail");
	welcome_hdr.Add("Subject", welcome_subject);

	fmt.Printf("Generated welcome mesg '%v'\n", welcome_hdr);
	return welcome_hdr;
}
