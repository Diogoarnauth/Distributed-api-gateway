package main

import (
	"encoding/json"
	"net/http"
)

func newMux(gateway http.Handler) *http.ServeMux {

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Hello from Go",
		})
	})

	mux.Handle("/api/", gateway)
	mux.Handle("/api", gateway)

	return mux
}