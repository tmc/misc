// Relay serves a hermetic local relay, native echo peer, blob, and browser assets.
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/tmc/go-iroh/gossip"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/tmc/go-iroh/blobs"
	"github.com/tmc/go-iroh/iroh"
	"github.com/tmc/go-iroh/netaddr"
	"github.com/tmc/go-iroh/relay"
	"github.com/tmc/go-iroh/relayserver"
)

func run(ctx context.Context, web, storeDir string) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	address := "http://" + listener.Addr().String()
	relayURL, err := netaddr.ParseRelayURL(address + "/")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	relayHandler := relayserver.New()
	mux.Handle("/relay", relayHandler)
	mux.Handle("/ping", relayHandler)
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "browser-import") })
	mux.Handle("/", http.FileServer(http.Dir(web)))
	server := &http.Server{Handler: mux}
	defer server.Close()
	go server.Serve(listener)
	ep, err := iroh.Bind(ctx, iroh.WithRelayMode(relay.ModeCustom(relay.MapFromURLs(relayURL))), iroh.WithoutIPTransports(), iroh.WithALPNs("wasmbind-echo/1", "wasmbind-raw/1", blobs.ALPN, gossip.ALPN))
	if err != nil {
		return err
	}
	store, err := blobs.NewFSStore(storeDir)
	if err != nil {
		return err
	}
	data := make([]byte, 70*1024)
	for i := range data {
		data[i] = byte(i*17 + 11)
	}
	hash, err := store.Add(data)
	if err != nil {
		return err
	}
	g := gossip.NewGossip(ep)
	var topicID gossip.TopicID
	topicID[0] = 1
	topic, err := g.Subscribe(ctx, topicID, nil)
	if err != nil {
		return err
	}
	defer topic.Close()
	go func() {
		timer := time.NewTicker(100 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				_ = topic.Broadcast(ctx, []byte("relay-gossip"))
			}
		}
	}()
	router, err := iroh.NewRouter(ep, map[string]iroh.ProtocolHandler{
		gossip.ALPN: g,
		"wasmbind-raw/1": iroh.ProtocolHandlerFunc(func(ctx context.Context, c *iroh.Conn) error {
			s, err := c.AcceptStream(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			_, err = io.Copy(s, s)
			return err
		}),
		"wasmbind-echo/1": iroh.ProtocolHandlerFunc(func(ctx context.Context, c *iroh.Conn) error {
			s, err := c.AcceptStream(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			for {
				var header [4]byte
				if _, err := io.ReadFull(s, header[:]); err != nil {
					return nil
				}
				n := binary.BigEndian.Uint32(header[:])
				if n > 1<<20 {
					return fmt.Errorf("frame exceeds one MiB")
				}
				data := make([]byte, n)
				if _, err := io.ReadFull(s, data); err != nil {
					return err
				}
				if _, err := s.Write(header[:]); err != nil {
					return err
				}
				if _, err := s.Write(data); err != nil {
					return err
				}
			}
		}),
		blobs.ALPN: iroh.ProtocolHandlerFunc(func(ctx context.Context, c *iroh.Conn) error {
			s, err := c.AcceptStream(ctx)
			if err != nil {
				return err
			}
			return blobs.ServeBlob(ctx, s, store)
		}),
	}, nil)
	if err != nil {
		return err
	}
	defer router.Shutdown(context.Background())
	if err := ep.Online(ctx); err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"url": address, "relay": address + "/", "peer": ep.ID().String(), "hash": hash.String(), "compiler": runtime.Version()}); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
func main() {
	web := flag.String("web", "", "browser assets directory")
	store := flag.String("store", "", "blob store directory")
	flag.Parse()
	if *web == "" || *store == "" {
		log.Fatal("-web and -store are required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, *web, *store); err != nil {
		log.Fatal(err)
	}
}
