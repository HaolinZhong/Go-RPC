package main

import (
	"Go-RPC/codec"
	"Go-RPC/network"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

func startServer(addr chan string) {
	l, err := net.Listen("tcp", ":7070")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	network.Accept(l)
}

func main() {
	addr := make(chan string)
	go startServer(addr)

	conn, _ := net.Dial("tcp", <-addr)
	defer func() { conn.Close() }()

	time.Sleep(time.Second)

	_ = json.NewEncoder(conn).Encode(network.DefaultOption)
	cc := codec.NewGobCodec(conn)

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		h := &codec.Header{
			MethodName: "Foo.Sum",
			RequestId:  uint64(i),
		}
		_ = cc.Write(h, fmt.Sprintf("rpc request %d", h.RequestId))
		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}
}
