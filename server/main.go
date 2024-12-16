package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	listen = os.Getenv("LISTEN")
	target = os.Getenv("TARGET")
	flag   = os.Getenv("ENABLE_AUTO_SHUTDOWN")
)

func main() {
	if listen == "" || target == "" {
		log.Fatalf("LISTEN or TARGET is not set")
	}

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	handler := NewHandler(target)

	if flag == "true" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			HealthyCheck(ctx, handler)
		}()
	}

	server := NewServer(
		"/",
		handler,
		WithListener(listener),
	)

	go func() {
		log.Println("Started ws server")
		err := server.Serve()
		if err != nil {
			log.Fatalf("Failed to ws serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down gracefully")
	wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	if err != nil {
		log.Fatalf("Failed to shutdown gracefully: %v", err)
	}
}
