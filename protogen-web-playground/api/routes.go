package api

import (
	"github.com/gorilla/mux"
)

// RegisterRoutes sets up the API routes
func RegisterRoutes(router *mux.Router, tempDir string) {
	// Create handlers
	generateHandler := NewGenerateHandler(tempDir)
	templateHandler := NewTemplateHandler()

	// Register routes
	router.HandleFunc("/generate", generateHandler.HandleGenerate).Methods("POST")
	router.HandleFunc("/templates/examples", templateHandler.HandleGetExamples).Methods("GET")
	router.HandleFunc("/templates/validate", templateHandler.HandleValidate).Methods("POST")
}