package webkithost

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

type assetServer struct {
	server *http.Server
	addr   string
	mu     sync.Mutex
	seen   map[string]int
}

func startAssetServer(assets Assets) (*assetServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen asset server: %w", err)
	}
	mux := http.NewServeMux()
	s := &assetServer{
		server: &http.Server{Handler: mux},
		addr:   ln.Addr().String(),
		seen:   make(map[string]int),
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.record(r.URL.Path)
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		asset, err := assets.Open(r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", asset.MIME)
		w.Header().Set("Content-Length", fmt.Sprint(len(asset.Data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(asset.Data)
	})
	go func() {
		_ = s.server.Serve(ln)
	}()
	return s, nil
}

func (s *assetServer) URL() string {
	return "http://" + s.addr + "/"
}

func (s *assetServer) Close(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *assetServer) record(path string) {
	s.mu.Lock()
	if s.seen == nil {
		s.seen = make(map[string]int)
	}
	s.seen[path]++
	s.mu.Unlock()
}

func (s *assetServer) Summary() string {
	if s == nil {
		return "asset server not started"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.seen) == 0 {
		return "no asset requests"
	}
	names := []string{"/", "/wanix.js", "/wanix.wasm", "/wanix.debug.wasm", "/rc.wasm"}
	var b strings.Builder
	for _, name := range names {
		if n := s.seen[name]; n > 0 {
			if b.Len() > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s:%d", name, n)
		}
	}
	if b.Len() == 0 {
		return fmt.Sprintf("%d asset paths requested", len(s.seen))
	}
	return b.String()
}
