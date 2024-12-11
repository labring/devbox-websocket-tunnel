package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var (
	listen = os.Getenv("LISTEN")
	target = os.Getenv("TARGET")
	flag   = os.Getenv("ENABLE_AUTO_SHUTDOWN")
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for system signals to initiate graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		cancel()
	}()

	if flag == "true" {
		go HealthyCheck(ctx)
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
}
