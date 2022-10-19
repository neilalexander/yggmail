/*
 *  Copyright (c) 2021 Neil Alexander
 *
 *  This Source Code Form is subject to the terms of the Mozilla Public
 *  License, v. 2.0. If a copy of the MPL was not distributed with this
 *  file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package transport

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"regexp"

	iwt "github.com/Arceliar/ironwood/types"
	"github.com/fatih/color"
	gologme "github.com/gologme/log"
	"github.com/neilalexander/utp"
	"github.com/yggdrasil-network/yggdrasil-go/src/core"
	"github.com/yggdrasil-network/yggdrasil-go/src/multicast"
)

type YggdrasilTransport struct {
	Sessions *utp.Socket
}

func NewYggdrasilTransport(log *log.Logger, sk ed25519.PrivateKey, pk ed25519.PublicKey, peers []string, mcast bool) (*YggdrasilTransport, error) {
	yellow := color.New(color.FgYellow).SprintfFunc()
	glog := gologme.New(log.Writer(), fmt.Sprintf("[ %s ] ", yellow("Yggdrasil")), gologme.LstdFlags|gologme.Lmsgprefix)
	glog.EnableLevel("warn")
	glog.EnableLevel("error")
	glog.EnableLevel("info")

	var ygg *core.Core
	var err error

	// Setup the Yggdrasil node itself.
	{
		options := []core.SetupOption{
			core.NodeInfo(map[string]interface{}{
				"name": hex.EncodeToString(pk) + "@yggmail",
			}),
			core.NodeInfoPrivacy(true),
		}
		for _, peer := range peers {
			options = append(options, core.Peer{URI: peer})
		}
		if ygg, err = core.New(sk[:], glog, options...); err != nil {
			panic(err)
		}
	}

	// Setup the multicast module.
	{
		options := []multicast.SetupOption{
			multicast.MulticastInterface{
				Regex:  regexp.MustCompile(".*"),
				Beacon: true,
				Listen: true,
			},
		}
		if _, err = multicast.New(ygg, glog, options...); err != nil {
			panic(err)
		}
	}

	us, err := utp.NewSocketFromPacketConnNoClose(ygg)
	if err != nil {
		return nil, fmt.Errorf("utp.NewSocketFromPacketConnNoClose: %w", err)
	}
	return &YggdrasilTransport{
		Sessions: us,
	}, nil
}

func (t *YggdrasilTransport) Dial(host string) (net.Conn, error) {
	addr := make(iwt.Addr, ed25519.PublicKeySize)
	k, err := hex.DecodeString(host)
	if err != nil {
		return nil, err
	}
	copy(addr, k)
	return t.Sessions.DialAddr(addr)
}

func (t *YggdrasilTransport) Listener() net.Listener {
	return t.Sessions
}
