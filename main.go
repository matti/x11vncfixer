package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/matti/betterio"
)

func main() {
	protocolVersion := os.Args[1]
	upstreamAddress := os.Args[2]

	fmt.Println("listen :5901")
	ln, err := net.Listen("tcp", "0.0.0.0:5901")

	if err != nil {
		panic(err)
	}

	for {
		conn, e := ln.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				log.Printf("accept temp err: %v", ne)
				continue
			}

			log.Printf("accept err: %v", e)
			return
		}

		go func() {
			log.Println(conn.RemoteAddr().String(), "handling")
			handle(conn, protocolVersion, upstreamAddress)
			log.Println(conn.RemoteAddr().String(), "handled")
		}()
	}
}

func handle(conn net.Conn, protocolVersion string, upstreamAddress string) {
	defer conn.Close()
	clientProtocolVersionChan := make(chan string, 1)
	go func() {
		log.Println(conn.RemoteAddr().String(), "sending protocolVersion", protocolVersion, "to client")
		conn.Write([]byte(protocolVersion + "\n"))

		clientProtocolVersion := strings.TrimSpace(string(betterio.ReadBytesUntilRune(conn, '\n')))
		log.Println(conn.RemoteAddr().String(), "clientProtocolVersion", clientProtocolVersion)

		clientProtocolVersionChan <- clientProtocolVersion
	}()

	dialer := net.Dialer{}
	upstream, err := dialer.Dial("tcp", upstreamAddress)
	if err != nil {
		log.Println("dial err", err)
		return
	}
	defer upstream.Close()

	serverProtocolVersionBytes := betterio.ReadBytesUntilRune(upstream, '\n')

	log.Println(conn.RemoteAddr().String(), "serverProtocolVersion", strings.TrimSpace(string(serverProtocolVersionBytes)))

	upstream.Write(serverProtocolVersionBytes)

	<-clientProtocolVersionChan

	log.Println(conn.RemoteAddr().String(), "copying")
	betterio.CopyBidirUntilCloseAndReturnBytesWritten(conn, upstream)
}
