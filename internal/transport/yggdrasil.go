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

	"github.com/fatih/color"
	gologme "github.com/gologme/log"
	"github.com/yggdrasil-network/yggdrasil-go/src/config"
	"github.com/yggdrasil-network/yggdrasil-go/src/core"
	"github.com/yggdrasil-network/yggdrasil-go/src/multicast"
	"github.com/yggdrasil-network/yggquic"
)

type YggdrasilTransport struct {
	yggquic *yggquic.YggdrasilTransport
}

func NewYggdrasilTransport(log *log.Logger, sk ed25519.PrivateKey, pk ed25519.PublicKey, peers []string, mcast bool, mcastregexp string) (*YggdrasilTransport, error) {
	yellow := color.New(color.FgYellow).SprintfFunc()
	glog := gologme.New(log.Writer(), fmt.Sprintf("[ %s ] ", yellow("Yggdrasil")), gologme.LstdFlags|gologme.Lmsgprefix)
	glog.EnableLevel("warn")
	glog.EnableLevel("error")
	glog.EnableLevel("info")

	cfg := config.GenerateConfig()
	copy(cfg.PrivateKey, sk)
	if err := cfg.GenerateSelfSignedCertificate(); err != nil {
		return nil, err
	}

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
		if ygg, err = core.New(cfg.Certificate, glog, options...); err != nil {
			panic(err)
		}
	}

	// Setup the multicast module.
	{
		options := []multicast.SetupOption{
			multicast.MulticastInterface{
				Regex:  regexp.MustCompile(mcastregexp),
				Beacon: mcast,
				Listen: mcast,
			},
		}
		if _, err = multicast.New(ygg, glog, options...); err != nil {
			panic(err)
		}
	}

	yq, err := yggquic.New(ygg, *cfg.Certificate, nil)
	if err != nil {
		panic(err)
	}

	return &YggdrasilTransport{
		yggquic: yq,
	}, nil
}

func (t *YggdrasilTransport) Dial(host string) (net.Conn, error) {
	c, err := t.yggquic.Dial("yggdrasil", host)
	if err != nil {
		return nil, err
	}
	// Needed to kick the stream so the other side "speaks first"
	_, err = c.Write([]byte(" "))
	return c, err
}

func (t *YggdrasilTransport) Listener() net.Listener {
	return t.yggquic
}
