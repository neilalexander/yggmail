package welcome;

import "testing";
// import "github.com/emersion/go-message/textproto";

func Test_WelcomeGenerate(t *testing.T) {
	// FIXME: Just make this a valid yggmail destination
	newUser := "Tristan";

	// generate welcome message header
	hdr := welcomeTo(newUser);
	if !hdr.Has("Subject") {
		t.Fail();
	}

	if !hdr.Has("From") {
		t.Fail()
	}

	if !hdr.Has("To") {
		t.Fail();
	} else if hdr.Get("To") != "Tristan@yggmail" {
		t.Fail()
	}
}
