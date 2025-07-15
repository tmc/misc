package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// testServer creates a test HTTP server with various endpoints for testing
func createTestServer() *httptest.Server {
	mux := http.NewServeMux()

	// Basic HTML page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <h1>Test Page</h1>
    <p>This is a test page for churl.</p>
    <div id="content">Main content here</div>
</body>
</html>`)
	})

	// JSON endpoint
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := map[string]interface{}{
			"message":   "Hello from API",
			"timestamp": time.Now().Unix(),
			"method":    r.Method,
		}
		json.NewEncoder(w).Encode(data)
	})

	// Echo headers endpoint
	mux.HandleFunc("/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		headers := make(map[string]string)
		for k, v := range r.Header {
			headers[k] = strings.Join(v, ", ")
		}
		json.NewEncoder(w).Encode(headers)
	})

	// Basic auth endpoint
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testpass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Test"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "Authenticated as: %s", user)
	})

	// POST data echo endpoint
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		w.Header().Set("Content-Type", "text/plain")
		w.Write(body)
	})

	// Redirect endpoint
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/redirected", http.StatusFound)
	})

	mux.HandleFunc("/redirected", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "You have been redirected")
	})

	// Slow endpoint for timeout testing
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		fmt.Fprintf(w, "Slow response")
	})

	// Dynamic content endpoint (simulates JavaScript-generated content)
	mux.HandleFunc("/dynamic", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Dynamic Content</title>
</head>
<body>
    <div id="dynamic">Loading...</div>
    <script>
        setTimeout(function() {
            document.getElementById('dynamic').innerHTML = 'Dynamic content loaded';
        }, 100);
    </script>
</body>
</html>`)
	})

	return httptest.NewServer(mux)
}
