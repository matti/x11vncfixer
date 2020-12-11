package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
)

func main() {
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
			log.Println("handling", conn.RemoteAddr().String())
			handle(conn)
			log.Println("handled", conn.RemoteAddr().String())
		}()
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	upstreamAddress := "localhost:5900"

	dialer := net.Dialer{}
	upstream, err := dialer.Dial("tcp", upstreamAddress)
	if err != nil {
		log.Println("dial err", err)
		return
	}
	defer upstream.Close()

	log.Println("upstream RemoteAddr", upstream.RemoteAddr())

	var sb strings.Builder
	b := make([]byte, 1)
	for {
		_, err := upstream.Read(b)

		if err != nil {
			panic(err)
		}
		if string(b) == "\n" {
			break
		}
		sb.WriteString(string(b))
	}
	serverProtcolVersion := sb.String()
	log.Println("serverProtcolVersion", serverProtcolVersion)

	upstream.Write([]byte("RFB 003.008\n"))

	upstreamClosed := make(chan struct{}, 1)
	clientClosed := make(chan struct{}, 1)

	go broker(upstream, conn, clientClosed)
	go broker(conn, upstream, upstreamClosed)

	var waitFor chan struct{}
	select {
	case <-clientClosed:
		log.Println("client closed")
		upstream.Close()
		waitFor = upstreamClosed
	case <-upstreamClosed:
		log.Println("upstream closed")
		conn.Close()
		waitFor = clientClosed
	}

	<-waitFor
}

func broker(dst, src net.Conn, srcClosed chan struct{}) {
	io.Copy(dst, src)

	srcClosed <- struct{}{}
}
