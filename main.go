package main

import (
	"log"
	"net/http"
	"time"

	"distributed-api-gateway/internal/config"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Loaded %d routes", len(cfg.Routes))

	gateway, err := NewGateway(cfg.Routes)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    ":8080",
		Handler: newMux(gateway, cfg.Routes),
		// Client -> gateway limits (gateway -> upstream ones are in transport.go).
		// ReadHeaderTimeout: max time to receive the request headers (Slowloris).
		ReadHeaderTimeout: 5 * time.Second,
		// ReadTimeout: max time to read the whole request, headers + body.
		ReadTimeout: 30 * time.Second,
		// IdleTimeout: how long a keep-alive connection may sit unused
		// between two requests before the server closes it.
		IdleTimeout: 60 * time.Second,
		// No WriteTimeout on purpose: it would also cut off slow but valid
		// upstream responses (and streams). The upstream wait is already
		// limited by responseHeaderTimeout in transport.go.
	}

	log.Println("Gateway running on port 8080")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
