package mcpframework

import (
	"context"
	"encoding/json"
)

const (
	LatestProtocolVersion = "2025-03-26"
	JSONRPCVersion        = "2.0"
)

type RequestID interface{}

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      RequestID   `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      RequestID   `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion  string               `json:"protocolVersion"`
	Capabilities     ClientCapabilities   `json:"capabilities"`
	ClientInfo       Implementation       `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string              `json:"protocolVersion"`
	Capabilities    ServerCapabilities  `json:"capabilities"`
	ServerInfo      Implementation      `json:"serverInfo"`
	Instructions    string              `json:"instructions,omitempty"`
}

type ClientCapabilities struct {
	Experimental interface{} `json:"experimental,omitempty"`
	Roots        *struct{}   `json:"roots,omitempty"`
	Sampling     *struct{}   `json:"sampling,omitempty"`
}

type ServerCapabilities struct {
	Experimental interface{} `json:"experimental,omitempty"`
	Logging      *struct{}   `json:"logging,omitempty"`
	Completions  *struct{}   `json:"completions,omitempty"`
	Prompts      *struct{}   `json:"prompts,omitempty"`
	Resources    *struct{}   `json:"resources,omitempty"`
	Tools        *struct{}   `json:"tools,omitempty"`
}

type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	InputSchema *ToolSchema   `json:"inputSchema"`
}

type ToolSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []interface{} `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Resource struct {
	URI         string      `json:"uri"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	MimeType    string      `json:"mimeType,omitempty"`
	Size        *int64      `json:"size,omitempty"`
}

type ReadResourceParams struct {
	URI string `json:"uri"`
}

type ReadResourceResult struct {
	Contents []interface{} `json:"contents"`
}

type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type ToolHandler func(ctx context.Context, params CallToolParams) (*CallToolResult, error)
type ResourceHandler func(ctx context.Context, uri string) (*ReadResourceResult, error)
type ResourceLister func(ctx context.Context) (*ListResourcesResult, error)