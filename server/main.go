package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	listen = os.Getenv("LISTEN")
	target = os.Getenv("TARGET")
	flag   = os.Getenv("ENABLE_AUTO_SHUTDOWN")
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup

	if flag == "true" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			HealthyCheck(ctx)
		}()
	}

	if listen == "" || target == "" {
		panic("LISTEN or TARGET is not set")
	}

	go func() {
		err := NewServer(
			listen,
			"/",
			NewHandler(target),
		).Serve()
		if err != nil {
			log.Fatalf("Failed to ws serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down gracefully")
	wg.Wait()
}
