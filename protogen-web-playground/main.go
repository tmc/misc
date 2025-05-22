package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"github.com/tmc/misc/protogen-web-playground/api"
	"github.com/tmc/misc/protogen-web-playground/api/github"
	"github.com/tmc/misc/protogen-web-playground/api/websocket"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Define command-line flags
	port := flag.String("port", getEnvWithDefault("PORT", "8080"), "HTTP service port")
	tempDir := flag.String("temp-dir", getEnvWithDefault("TEMP_DIR", os.TempDir()), "Directory for temporary files")
	flag.Parse()

	// Create router
	router := mux.NewRouter()

	// Set up API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	api.RegisterRoutes(apiRouter, *tempDir)
	github.RegisterRoutes(apiRouter.PathPrefix("/github").Subrouter())

	// Set up WebSocket route
	wsHub := websocket.NewHub()
	go wsHub.Run()
	router.HandleFunc("/ws/session", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(wsHub, w, r)
	})

	// Serve static files from the frontend/build directory in production mode
	router.PathPrefix("/").Handler(serveStatic("./frontend/build"))

	// Set up CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Accept-Language", "Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	// Create server
	addr := fmt.Sprintf(":%s", *port)
	server := &http.Server{
		Addr:    addr,
		Handler: c.Handler(router),
	}

	// Start server
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(server.ListenAndServe())
}

// serveStatic returns an HTTP handler that serves static files from a directory
func serveStatic(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := filepath.Abs(r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		path = filepath.Join(dir, path)

		// Check if file exists and serve index.html for SPA routes
		_, err = os.Stat(path)
		if os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.FileServer(http.Dir(dir)).ServeHTTP(w, r)
	})
}

// getEnvWithDefault returns the value of the environment variable or a default value
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}