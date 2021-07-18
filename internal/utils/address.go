/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package utils

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
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
	pk, err := hex.DecodeString(email[:at])
	if err != nil {
		return nil, fmt.Errorf("hex.DecodeString: %w", err)
	}
	ed := make(ed25519.PublicKey, ed25519.PublicKeySize)
	copy(ed, pk)
	return ed, nil
}
