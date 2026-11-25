package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"service": "orders",
			"status":  "success",
			"message": "Request processed by upstream",
		})
	})

	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	log.Println("Upstream running on port 8081")

	log.Fatal(server.ListenAndServe())
}
