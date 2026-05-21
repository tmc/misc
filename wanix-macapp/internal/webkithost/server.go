package webkithost

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

type assetServer struct {
	server *http.Server
	addr   string
}

func startAssetServer(assets Assets) (*assetServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen asset server: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
	s := &assetServer{
		server: &http.Server{Handler: mux},
		addr:   ln.Addr().String(),
	}
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
