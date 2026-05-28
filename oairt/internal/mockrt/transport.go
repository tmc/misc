package mockrt

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
)

// conn is the minimal WebSocket surface mockrt needs. It is intentionally
// small so the underlying library can be swapped (Workstream A5).
type conn interface {
	ReadText(ctx context.Context) ([]byte, error)
	WriteText(ctx context.Context, data []byte) error
	Close() error
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

func upgrade(w http.ResponseWriter, r *http.Request) (conn, error) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return &gorillaConn{c: c}, nil
}

type gorillaConn struct{ c *websocket.Conn }

func (g *gorillaConn) ReadText(_ context.Context) ([]byte, error) {
	_, data, err := g.c.ReadMessage()
	return data, err
}

func (g *gorillaConn) WriteText(_ context.Context, data []byte) error {
	return g.c.WriteMessage(websocket.TextMessage, data)
}

func (g *gorillaConn) Close() error { return g.c.Close() }
