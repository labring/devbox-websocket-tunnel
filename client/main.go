package main

import (
	"flag"
	"io"
	"net/url"
	"os"
)

var (
	target   string
	insecure bool
)

func init() {
	flag.StringVar(&target, "target", "ws://127.0.0.1:8081/ws", "target url")
	flag.BoolVar(&insecure, "insecure", false, "tls insecure")
}

func main() {
	flag.Parse()

	u, err := url.Parse(target)
	if err != nil {
		panic(err)
	}
	conn, err := NewDialer(
		WithURL(u),
		WithInsecure(insecure),
	).Dial()
	if err != nil {
		panic(err)
	}
	go func() {
		_, _ = io.Copy(os.Stdout, conn)
	}()
	_, _ = io.Copy(conn, os.Stdin)
}
