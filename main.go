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
		Addr:              ":8080",
		Handler:           newMux(gateway),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("Gateway running on port 8080")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}