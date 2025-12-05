package welcome;

import "testing";
import "fmt";

func Test_WelcomeGenerate(t *testing.T) {
	// FIXME: Just make this a valid yggmail destination
	newUser := "Tristan";

	// generate welcome message header
	bytesOut, e := WelcomeMessageFor(newUser);

	if e != nil {
		t.Fail();
	} else if len(bytesOut) == 0 {
		t.Fail()
	}
	
	fmt.Printf("Out: %v\n", bytesOut);
}
