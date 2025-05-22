package client

import (
	"context"
	"testing"
	"time"

	"github.com/tmc/misc/vim-jsonrpc/pkg/protocol"
)

type mockTransport struct {
	writeData [][]byte
	closed    bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		writeData: make([][]byte, 0),
	}
}

func (t *mockTransport) Read() ([]byte, error) {
	time.Sleep(10 * time.Millisecond)
	return []byte{}, nil
}

func (t *mockTransport) Write(data []byte) error {
	t.writeData = append(t.writeData, data)
	return nil
}

func (t *mockTransport) Close() error {
	t.closed = true
	return nil
}

func TestClientNotify(t *testing.T) {
	transport := newMockTransport()
	client := &Client{
		transport: transport,
		pending:   make(map[interface{}]chan *protocol.Response),
		handlers:  make(map[string]func(params interface{})),
	}
	
	err := client.Notify("test_notification", "test_param")
	if err != nil {
		t.Fatalf("Notify failed: %v", err)
	}
	
	if len(transport.writeData) != 1 {
		t.Errorf("Expected 1 write, got %d", len(transport.writeData))
	}
}

func TestClientOnNotification(t *testing.T) {
	client := &Client{
		transport: newMockTransport(),
		pending:   make(map[interface{}]chan *protocol.Response),
		handlers:  make(map[string]func(params interface{})),
	}
	
	done := make(chan interface{}, 1)
	client.OnNotification("test_notify", func(params interface{}) {
		done <- params
	})
	
	notif := &protocol.Notification{
		JSONRPC: "2.0",
		Method:  "test_notify",
		Params:  "test_param",
	}
	
	client.handleNotification(notif)
	
	select {
	case received := <-done:
		if received != "test_param" {
			t.Errorf("Expected received 'test_param', got %v", received)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Notification handler not called within timeout")
	}
}

func TestClientHandleResponse(t *testing.T) {
	client := &Client{
		transport: newMockTransport(),
		pending:   make(map[interface{}]chan *protocol.Response),
		handlers:  make(map[string]func(params interface{})),
	}
	
	respCh := make(chan *protocol.Response, 1)
	client.pending[1] = respCh
	
	resp := &protocol.Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  "test_result",
	}
	
	client.handleResponse(resp)
	
	select {
	case receivedResp := <-respCh:
		if receivedResp.Result != "test_result" {
			t.Errorf("Expected result 'test_result', got %v", receivedResp.Result)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Response not received in time")
	}
	
	if _, exists := client.pending[1]; exists {
		t.Error("Expected pending request to be removed")
	}
}

func TestClientCallTimeout(t *testing.T) {
	transport := newMockTransport()
	client := &Client{
		transport: transport,
		pending:   make(map[interface{}]chan *protocol.Response),
		handlers:  make(map[string]func(params interface{})),
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	
	_, err := client.Call(ctx, "test_method", nil)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected timeout error, got %v", err)
	}
}

func TestClientClose(t *testing.T) {
	transport := newMockTransport()
	client := &Client{
		transport: transport,
		pending:   make(map[interface{}]chan *protocol.Response),
		handlers:  make(map[string]func(params interface{})),
	}
	
	err := client.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	
	if !transport.closed {
		t.Error("Expected transport to be closed")
	}
	
	if !client.closed {
		t.Error("Expected client to be closed")
	}
}