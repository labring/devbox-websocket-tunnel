package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

var (
	interval      = os.Getenv("AUTO_SHUTDOWN_INTERVAL")
	serviceTarget = os.Getenv("AUTO_SHUTDOWN_SERVICE_URL")
	jwtToken      = os.Getenv("JWT_TOKEN")
)

var ActiveNum int32 = 0

func HealthyCheck() {
	shutdownDuration, _ := time.ParseDuration(interval)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	zeroDuration := 0 * time.Minute
	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&ActiveNum) == 0 {
				zeroDuration += 1 * time.Minute
				if zeroDuration >= shutdownDuration {
					sendShutdownRequest()
				}
			} else {
				zeroDuration = 0
			}
		}
	}
}

func sendShutdownRequest() {
	url := serviceTarget + "/opsrequest"
	data := map[string]string{
		"operation": "shutdown",
		"jwt_token": jwtToken,
	}
	jsonData, _ := json.Marshal(data)
	_, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("error:fail to send shutdown request", err)
	}
}
