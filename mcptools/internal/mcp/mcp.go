// Package mcp implements the Model Context Protocol.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// Protocol constants
const (
	Version     = "2024-01-16"
	JSONRPCVer  = "2.0"
	ContentText = "text"
	ContentData = "data"
)

// Message represents a JSON-RPC 2.0 message.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.Number    `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Content represents tool output content.
type Content struct {
	Type     string `json:"type"`               // text or data
	Text     string `json:"text,omitempty"`     // for text content
	Data     []byte `json:"data,omitempty"`     // for binary data
	MimeType string `json:"mimeType,omitempty"` // optional MIME type
}

// Tool represents an executable MCP tool.
type Tool interface {
	Name() string
	Description() string
	Handle(ctx context.Context, args json.RawMessage) ([]Content, error)
}

// Server handles MCP protocol communication.
type Server struct {
	name    string
	version string
	tools   map[string]Tool
}

// NewServer creates a new MCP server.
func NewServer(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   make(map[string]Tool),
	}
}

// RegisterTool registers a tool with the server.
func (s *Server) RegisterTool(t Tool) error {
	if t == nil {
		return fmt.Errorf("nil tool")
	}
	s.tools[t.Name()] = t
	return nil
}

// Handle processes an incoming message.
func (s *Server) Handle(ctx context.Context, msg []byte) ([]byte, error) {
	var req Message
	if err := json.Unmarshal(msg, &req); err != nil {
		return s.errorResponse(nil, -32700, "parse error")
	}

	if req.JSONRPC != JSONRPCVer {
		return s.errorResponse(req.ID, -32600, "invalid JSON-RPC version")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.ID)
	case "listTools":
		return s.handleListTools(req.ID)
	default:
		return s.handleToolCall(ctx, req.ID, req.Method, req.Params)
	}
}

func (s *Server) handleInitialize(id *json.Number) ([]byte, error) {
	resp := struct {
		Name            string `json:"name"`
		Version         string `json:"version"`
		ProtocolVersion string `json:"protocolVersion"`
	}{
		Name:            s.name,
		Version:         s.version,
		ProtocolVersion: Version,
	}
	return s.successResponse(id, resp)
}

func (s *Server) handleListTools(id *json.Number) ([]byte, error) {
	var tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	for _, t := range s.tools {
		tools = append(tools, struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}{
			Name:        t.Name(),
			Description: t.Description(),
		})
	}
	return s.successResponse(id, struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}{Tools: tools})
}

func (s *Server) handleToolCall(ctx context.Context, id *json.Number, method string, params json.RawMessage) ([]byte, error) {
	tool, ok := s.tools[method]
	if !ok {
		return s.errorResponse(id, -32601, fmt.Sprintf("method %q not found", method))
	}

	content, err := tool.Handle(ctx, params)
	if err != nil {
		return s.errorResponse(id, -32000, err.Error())
	}

	return s.successResponse(id, struct {
		Content []Content `json:"content"`
	}{Content: content})
}

func (s *Server) successResponse(id *json.Number, result interface{}) ([]byte, error) {
	resp := Message{
		JSONRPC: JSONRPCVer,
		ID:      id,
	}
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		resp.Result = data
	}
	return json.Marshal(resp)
}

func (s *Server) errorResponse(id *json.Number, code int, msg string) ([]byte, error) {
	resp := Message{
		JSONRPC: JSONRPCVer,
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: msg,
		},
	}
	return json.Marshal(resp)
}

// Transport represents a bidirectional MCP transport.
type Transport interface {
	io.ReadWriteCloser
	Context() context.Context
}

// StdioTransport implements Transport over stdin/stdout.
type StdioTransport struct {
	ctx context.Context
	in  io.Reader
	out io.Writer
}

// NewStdioTransport creates a new stdio transport.
func NewStdioTransport(ctx context.Context) *StdioTransport {
	return &StdioTransport{ctx: ctx}
}

func (t *StdioTransport) Read(p []byte) (n int, err error)  { return t.in.Read(p) }
func (t *StdioTransport) Write(p []byte) (n int, err error) { return t.out.Write(p) }
func (t *StdioTransport) Close() error                      { return nil }
func (t *StdioTransport) Context() context.Context          { return t.ctx }
