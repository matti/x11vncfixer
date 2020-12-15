package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/matti/betterio"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

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

	var upstream net.Conn
	retryPrinted := false

	for {
		if err := betterio.CheckReaderOpen(conn); err != nil {
			log.Println(conn.RemoteAddr().String(), "client gone or sends garbage", err)
			return
		}

		dialer := net.Dialer{
			Timeout: 100 * time.Millisecond,
		}
		var err error
		upstream, err = dialer.Dial("tcp", upstreamAddress)
		if err != nil {
			if !retryPrinted {
				retryPrinted = true
				log.Println(conn.RemoteAddr().String(), "dial err to", upstreamAddress, "will retry until client disconnects", err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		break
	}
	defer upstream.Close()

	clientProtocolVersionChan := make(chan string, 1)
	go func() {
		log.Println(conn.RemoteAddr().String(), "sending protocolVersion", protocolVersion, "to client")
		conn.Write([]byte(protocolVersion + "\n"))

		clientProtocolVersion := strings.TrimSpace(string(betterio.ReadBytesUntilRune(conn, '\n')))
		log.Println(conn.RemoteAddr().String(), "clientProtocolVersion", clientProtocolVersion)

		clientProtocolVersionChan <- clientProtocolVersion
	}()

	serverProtocolVersionBytes := betterio.ReadBytesUntilRune(upstream, '\n')

	log.Println(conn.RemoteAddr().String(), "serverProtocolVersion", strings.TrimSpace(string(serverProtocolVersionBytes)))
	if err := betterio.CheckReaderOpen(upstream); err != nil {
		log.Println(conn.RemoteAddr().String(), "server", upstream.RemoteAddr().String(), "gone")
		return
	}

	upstream.Write(serverProtocolVersionBytes)

	<-clientProtocolVersionChan
	if err := betterio.CheckReaderOpen(conn); err != nil {
		log.Println(conn.RemoteAddr().String(), "client gone")
		return
	}

	log.Println(conn.RemoteAddr().String(), "copying")
	betterio.CopyBidirUntilCloseAndReturnBytesWritten(conn, upstream)
}
