package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"context"
	"log"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Initialize router
	r := chi.NewRouter()

	// Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Define the main proxy endpoint (to be implemented later)
	r.Post("/", proxyHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}
	log.Printf("Server listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	var anthropicReq AnthropicRequest
	err := json.NewDecoder(r.Body).Decode(&anthropicReq)
	if err \!= nil {
		http.Error(w, "Error decoding request body", http.StatusBadRequest)
		return
	}

	ollamaResp, err := callOllama(anthropicReq)
	if err \!= nil {
		http.Error(w, fmt.Sprintf("Error calling Ollama: %s", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(ollamaResp)
	if err \!= nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
