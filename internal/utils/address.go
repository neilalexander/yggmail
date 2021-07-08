package utils

import (
	"fmt"
	"strings"
)

const TLD = ".yggmail"

func ParseAddress(email string) (string, string, error) {
	if !strings.HasSuffix(email, TLD) {
		return "", "", fmt.Errorf("invalid TLD")
	}
	at := strings.LastIndex(email, "@")
	if at == 0 {
		return "", "", fmt.Errorf("invalid email address")
	}
	return email[:at], strings.TrimSuffix(email[at+1:], TLD), nil
}
