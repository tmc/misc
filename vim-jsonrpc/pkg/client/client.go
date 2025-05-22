package client

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmc/misc/vim-jsonrpc/pkg/protocol"
	"github.com/tmc/misc/vim-jsonrpc/pkg/transport"
)

type Client struct {
	transport transport.Transport
	mu        sync.RWMutex
	pending   map[interface{}]chan *protocol.Response
	handlers  map[string]func(params interface{})
	nextID    int64
	closed    bool
}

func New(transportType, addr, socket string) (*Client, error) {
	t, err := transport.NewTransport(transportType, addr, socket)
	if err != nil {
		return nil, err
	}
	
	return &Client{
		transport: t,
		pending:   make(map[interface{}]chan *protocol.Response),
		handlers:  make(map[string]func(params interface{})),
	}, nil
}

func (c *Client) Connect() error {
	go c.readLoop()
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return nil
	}
	c.closed = true
	
	for _, ch := range c.pending {
		close(ch)
	}
	
	return c.transport.Close()
}

func (c *Client) readLoop() {
	for {
		c.mu.RLock()
		closed := c.closed
		c.mu.RUnlock()
		
		if closed {
			return
		}
		
		data, err := c.transport.Read()
		if err != nil {
			return
		}
		
		if len(data) == 0 {
			continue
		}
		
		data = data[:len(data)-1]
		
		msg, err := protocol.ParseMessage(data)
		if err != nil {
			continue
		}
		
		switch m := msg.(type) {
		case *protocol.Response:
			c.handleResponse(m)
		case *protocol.Notification:
			c.handleNotification(m)
		}
	}
}

func (c *Client) handleResponse(resp *protocol.Response) {
	c.mu.RLock()
	ch, exists := c.pending[resp.ID]
	c.mu.RUnlock()
	
	if !exists {
		return
	}
	
	select {
	case ch <- resp:
	default:
	}
	
	c.mu.Lock()
	delete(c.pending, resp.ID)
	c.mu.Unlock()
}

func (c *Client) handleNotification(notif *protocol.Notification) {
	c.mu.RLock()
	handler, exists := c.handlers[notif.Method]
	c.mu.RUnlock()
	
	if exists {
		go handler(notif.Params)
	}
}

func (c *Client) Call(ctx context.Context, method string, params interface{}) (interface{}, error) {
	id := atomic.AddInt64(&c.nextID, 1)
	req := protocol.NewRequest(id, method, params)
	
	respCh := make(chan *protocol.Response, 1)
	c.mu.Lock()
	c.pending[id] = respCh
	c.mu.Unlock()
	
	data, err := json.Marshal(req)
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	
	if err := c.transport.Write(data); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (c *Client) Notify(method string, params interface{}) error {
	notif := protocol.NewNotification(method, params)
	
	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	
	return c.transport.Write(data)
}

func (c *Client) OnNotification(method string, handler func(params interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[method] = handler
}

func (c *Client) CallWithTimeout(method string, params interface{}, timeout time.Duration) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.Call(ctx, method, params)
}