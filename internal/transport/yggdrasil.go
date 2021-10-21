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

	iwt "github.com/Arceliar/ironwood/types"
	"github.com/fatih/color"
	gologme "github.com/gologme/log"
	"github.com/neilalexander/utp"
	"github.com/yggdrasil-network/yggdrasil-go/src/config"
	"github.com/yggdrasil-network/yggdrasil-go/src/core"
	"github.com/yggdrasil-network/yggdrasil-go/src/multicast"
)

type YggdrasilTransport struct {
	Sessions *utp.Socket
}

func NewYggdrasilTransport(log *log.Logger, sk ed25519.PrivateKey, pk ed25519.PublicKey, peers []string, mcast bool) (*YggdrasilTransport, error) {
	config := &config.NodeConfig{
		PublicKey:  hex.EncodeToString(pk),
		PrivateKey: hex.EncodeToString(sk),
		Peers:      peers,
		MulticastInterfaces: []config.MulticastInterfaceConfig{
			{
				Regex:  ".*",
				Beacon: true,
				Listen: true,
			},
		},
		NodeInfo: map[string]interface{}{
			"name": "Yggmail",
		},
		NodeInfoPrivacy: true,
	}
	yellow := color.New(color.FgYellow).SprintfFunc()
	glog := gologme.New(log.Writer(), fmt.Sprintf("[ %s ] ", yellow("Yggdrasil")), gologme.LstdFlags | gologme.Lmsgprefix)
	glog.EnableLevel("warn")
	glog.EnableLevel("error")
	glog.EnableLevel("info")

	core := &core.Core{}
	if err := core.Start(config, glog); err != nil {
		return nil, fmt.Errorf("core.Start: %w", err)
	}
	if mcast {
		multicast := &multicast.Multicast{}
		if err := multicast.Init(core, config, glog, nil); err != nil {
			return nil, fmt.Errorf("multicast.Init: %w", err)
		}
		if err := multicast.Start(); err != nil {
			return nil, fmt.Errorf("multicast.Start: %w", err)
		}
	}
	us, err := utp.NewSocketFromPacketConnNoClose(core)
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
