package utils

import (
	"crypto/ed25519"
	"fmt"
	"strings"

	"github.com/jxskiss/base62"
)

const Domain = "yggmail"

func CreateAddress(pk ed25519.PublicKey) string {
	return fmt.Sprintf(
		"%s@%s",
		pk, Domain,
	)
}

func ParseAddress(email string) (ed25519.PublicKey, error) {
	at := strings.LastIndex(email, "@")
	if at == 0 {
		return nil, fmt.Errorf("invalid email address")
	}
	if email[at+1:] != Domain {
		return nil, fmt.Errorf("invalid email domain")
	}
	pk, err := base62.DecodeString(email[:at])
	if err != nil {
		return nil, fmt.Errorf("base62.DecodeString: %w", err)
	}
	ed := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(ed, pk)
	return ed, nil
}
