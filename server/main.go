package main

import "os"

var (
	listen = os.Getenv("LISTEN")
	target = os.Getenv("TARGET")
	flag   = os.Getenv("ENABLE_AUTO_SHUTDOWN")
)

func main() {
	if flag == "true" {
		go HealthyCheck()
	}
	if listen == "" || target == "" {
		panic("LISTEN or TARGET is not set")
	}
	_ = NewServer(
		listen,
		"/",
		NewHandler(target),
	).Serve()
}
