package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"log"
	"net"
	"os"

	"github.com/neilalexander/yggmail/internal/transport"
)

var dst = flag.String("dst", "", "Destination public key to proxy to")
var peeraddr = flag.String("peer", "", "Yggdrasil static peer")

func main() {
	flag.Parse()

	/*
		pk, sk, err := ed25519.GenerateKey(nil)
		if err != nil {
			panic(err)
		}
	*/

	sk := make(ed25519.PrivateKey, ed25519.PrivateKeySize)
	if sks, err := hex.DecodeString("f50a6a4688ca602307dcf282304583b1746093a558e934d3e9a817bb1e7be77b7c824efcd702e80a3a6912e15ebc4e13454022947ce8ee46ddb871e8b9a9147f"); err != nil {
		panic(err)
	} else {
		copy(sk, sks)
	}
	pk := sk.Public().(ed25519.PublicKey)

	log.Println("Private key:", hex.EncodeToString(sk))

	log := log.New(os.Stdout, "", 0)
	transport, err := transport.NewYggdrasilTransport(log, sk, pk, *peeraddr)
	if err != nil {
		panic(err)
	}

	listener, err := net.Listen("tcp", "localhost:1026")
	if err != nil {
		panic(err)
	}

	log.Println("Proxying", listener.Addr(), "to", *dst)

	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		log.Println("Accepted connection from", conn.RemoteAddr())

		upstream, err := transport.Dial(*dst)
		if err != nil {
			log.Println("Failed to dial upstream:", err)
			conn.Close()
			continue
		}

		go func(conn, upstream net.Conn) {
			defer conn.Close()
			defer upstream.Close()
			var b [1024]byte
			for {
				n, err := conn.Read(b[:])
				if err != nil {
					log.Println("conn.Read:", err)
					return
				}
				_, err = upstream.Write(b[:n])
				if err != nil {
					log.Println("upstream.Write:", err)
					return
				}
			}
		}(conn, upstream)

		go func(conn, upstream net.Conn) {
			defer conn.Close()
			defer upstream.Close()
			var b [1024]byte
			for {
				n, err := upstream.Read(b[:])
				if err != nil {
					log.Println("upstream.Read:", err)
					return
				}
				_, err = conn.Write(b[:n])
				if err != nil {
					log.Println("conn.Write:", err)
					return
				}
			}
		}(conn, upstream)
	}
}
