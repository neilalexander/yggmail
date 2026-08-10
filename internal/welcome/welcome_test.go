package welcome

import (
	"fmt"
	"testing"
	"log"
)

func Test_WelcomeGenerate(t *testing.T) {
	newUser := "Tristan"

	// generate welcome message header
	// FIXME: How do we get a nu
	bytesOut, e := welcomeMessageFor(newUser, log.Default())

	if e != nil {
		t.Fail()
	} else if len(bytesOut) == 0 {
		t.Fail()
	}

	fmt.Printf("Out: %v\n", bytesOut)
}
