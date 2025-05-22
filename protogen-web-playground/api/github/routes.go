package github

import (
	"github.com/gorilla/mux"
)

// RegisterRoutes sets up the GitHub API routes
func RegisterRoutes(router *mux.Router) {
	// Create handlers
	handler := NewGithubHandler()

	// Register routes
	router.HandleFunc("/gists/{id}", handler.HandleGetGist).Methods("GET")
	router.HandleFunc("/gists", handler.HandleCreateGist).Methods("POST")
	router.HandleFunc("/gists/{id}", handler.HandleUpdateGist).Methods("PUT")
}