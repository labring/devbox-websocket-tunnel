package main

import "os"

var (
	listen = os.Getenv("LISTEN")
	target = os.Getenv("TARGET")
)

func main() {
	if listen == "" || target == "" {
		panic("LISTEN or TARGET is not set")
	}
	_ = NewServer(
		listen,
		"/",
		NewHandler(target),
	).Serve()
}
