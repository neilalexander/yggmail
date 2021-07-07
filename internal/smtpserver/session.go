package smtpserver

import (
	"fmt"
	"strings"
)

func parseAddress(email string) (string, string, error) {
	at := strings.LastIndex(email, "@")
	if at == 0 {
		return "", "", fmt.Errorf("invalid email address")
	}
	return email[:at], email[at+1:], nil
}
