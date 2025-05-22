package mcpframework

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type Server struct {
	info           Implementation
	capabilities   ServerCapabilities
	instructions   string
	tools          map[string]ToolHandler
	resources      map[string]ResourceHandler
	resourceLister ResourceLister
	mu             sync.RWMutex
	logger         *log.Logger
}

func NewServer(name, version string) *Server {
	return &Server{
		info: Implementation{
			Name:    name,
			Version: version,
		},
		capabilities: ServerCapabilities{
			Tools:     &struct{}{},
			Resources: &struct{}{},
		},
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		logger:    log.New(os.Stderr, "[MCP] ", log.LstdFlags),
	}
}

func (s *Server) SetInstructions(instructions string) {
	s.instructions = instructions
}

func (s *Server) SetLogger(logger *log.Logger) {
	s.logger = logger
}

func (s *Server) RegisterTool(name, description string, schema *ToolSchema, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = handler
}

func (s *Server) RegisterResourceHandler(pattern string, handler ResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[pattern] = handler
}

func (s *Server) SetResourceLister(lister ResourceLister) {
	s.resourceLister = lister
}

func (s *Server) Run(ctx context.Context, input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(input)
	encoder := json.NewEncoder(output)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var rawMessage json.RawMessage
		if err := decoder.Decode(&rawMessage); err != nil {
			if err == io.EOF {
				return nil
			}
			s.logger.Printf("Error decoding message: %v", err)
			continue
		}

		response, err := s.handleMessage(ctx, rawMessage)
		if err != nil {
			s.logger.Printf("Error handling message: %v", err)
			continue
		}

		if response != nil {
			if err := encoder.Encode(response); err != nil {
				s.logger.Printf("Error encoding response: %v", err)
			}
		}
	}
}

func (s *Server) handleMessage(ctx context.Context, rawMessage json.RawMessage) (interface{}, error) {
	var request JSONRPCRequest
	if err := json.Unmarshal(rawMessage, &request); err != nil {
		return nil, fmt.Errorf("invalid JSON-RPC request: %w", err)
	}

	switch request.Method {
	case "initialize":
		return s.handleInitialize(request)
	case "tools/list":
		return s.handleListTools(request)
	case "tools/call":
		return s.handleCallTool(ctx, request)
	case "resources/list":
		return s.handleListResources(ctx, request)
	case "resources/read":
		return s.handleReadResource(ctx, request)
	default:
		return s.makeErrorResponse(request.ID, -32601, "Method not found", nil), nil
	}
}

func (s *Server) handleInitialize(request JSONRPCRequest) (*JSONRPCResponse, error) {
	result := InitializeResult{
		ProtocolVersion: LatestProtocolVersion,
		Capabilities:    s.capabilities,
		ServerInfo:      s.info,
		Instructions:    s.instructions,
	}

	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  result,
	}, nil
}

func (s *Server) handleListTools(request JSONRPCRequest) (*JSONRPCResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.tools))
	for name := range s.tools {
		tool := Tool{
			Name: name,
			InputSchema: &ToolSchema{
				Type:       "object",
				Properties: make(map[string]interface{}),
			},
		}
		tools = append(tools, tool)
	}

	result := ListToolsResult{
		Tools: tools,
	}

	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  result,
	}, nil
}

func (s *Server) handleCallTool(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error) {
	var params CallToolParams
	if request.Params != nil {
		paramsBytes, err := json.Marshal(request.Params)
		if err != nil {
			return s.makeErrorResponse(request.ID, -32602, "Invalid params", nil), nil
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.makeErrorResponse(request.ID, -32602, "Invalid params", nil), nil
		}
	}

	s.mu.RLock()
	handler, exists := s.tools[params.Name]
	s.mu.RUnlock()

	if !exists {
		return s.makeErrorResponse(request.ID, -32601, "Tool not found", nil), nil
	}

	result, err := handler(ctx, params)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      request.ID,
			Result: CallToolResult{
				Content: []interface{}{
					TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error: %s", err.Error()),
					},
				},
				IsError: true,
			},
		}, nil
	}

	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  result,
	}, nil
}

func (s *Server) handleListResources(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error) {
	if s.resourceLister == nil {
		return s.makeErrorResponse(request.ID, -32601, "Resources not supported", nil), nil
	}

	result, err := s.resourceLister(ctx)
	if err != nil {
		return s.makeErrorResponse(request.ID, -32603, "Internal error", err.Error()), nil
	}

	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  result,
	}, nil
}

func (s *Server) handleReadResource(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error) {
	var params ReadResourceParams
	if request.Params != nil {
		paramsBytes, err := json.Marshal(request.Params)
		if err != nil {
			return s.makeErrorResponse(request.ID, -32602, "Invalid params", nil), nil
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return s.makeErrorResponse(request.ID, -32602, "Invalid params", nil), nil
		}
	}

	s.mu.RLock()
	var handler ResourceHandler
	for pattern, h := range s.resources {
		if pattern == params.URI || pattern == "*" {
			handler = h
			break
		}
	}
	s.mu.RUnlock()

	if handler == nil {
		return s.makeErrorResponse(request.ID, -32601, "Resource not found", nil), nil
	}

	result, err := handler(ctx, params.URI)
	if err != nil {
		return s.makeErrorResponse(request.ID, -32603, "Internal error", err.Error()), nil
	}

	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      request.ID,
		Result:  result,
	}, nil
}

func (s *Server) makeErrorResponse(id RequestID, code int, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}