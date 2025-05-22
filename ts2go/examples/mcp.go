// Code generated from /Users/tmc/go/src/github.com/modelcontextprotocol/modelcontextprotocol/schema/2025-03-26/schema.ts; DO NOT EDIT.
package mcp

import (
	"encoding/json"
)


// 
const LATEST_PROTOCOL_VERSION = "2025-03-26"

// 
const JSONRPC_VERSION = "2.0"



// JSONRPCMessage represents / JSON-RPC types Refers to any valid JSON-RPC object that can be decoded off the wire, or encoded to be sent.
type JSONRPCMessage interface{}

// JSONRPCBatchRequest represents A JSON-RPC batch request, as described in https:www.jsonrpc.org/specification#batch.
type JSONRPCBatchRequest []interface{}

// JSONRPCBatchResponse represents A JSON-RPC batch response, as described in https:www.jsonrpc.org/specification#batch.
type JSONRPCBatchResponse []interface{}

// ProgressToken represents A progress token, used to associate progress notifications with the original request.
type ProgressToken interface{}

// Cursor represents An opaque token used to represent a cursor for pagination.
type Cursor string

// RequestId represents A uniquely identifying ID for a request in JSON-RPC.
type RequestId interface{}

// EmptyResult represents / Empty result A response that indicates success but carries no data.
type EmptyResult Result

// Role represents The sender or recipient of messages and data in a conversation.
type Role string

// LoggingLevel represents The severity of a log message.
// These map to syslog message severities, as specified in RFC-5424:
// https:datatracker.ietf.org/doc/html/rfc5424#section-6.2.1
type LoggingLevel string

// ClientRequest represents / Client messages
type ClientRequest interface{}

// ClientNotification represents 
type ClientNotification interface{}

// ClientResult represents 
type ClientResult interface{}

// ServerRequest represents / Server messages
type ServerRequest interface{}

// ServerNotification represents 
type ServerNotification interface{}

// ServerResult represents 
type ServerResult interface{}



// Request 
type Request struct {
	Method string `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// Notification 
type Notification struct {
	Method string `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// Result 
type Result struct {
	_meta interface{} `json:"_meta,omitempty"`
}

// JSONRPCRequest A request that expects a response.
type JSONRPCRequest struct {
	Jsonrpc interface{} `json:"jsonrpc"`
	Id RequestId `json:"id"`
}

// JSONRPCNotification A notification which does not expect a response.
type JSONRPCNotification struct {
	Jsonrpc interface{} `json:"jsonrpc"`
}

// JSONRPCResponse A successful (non-error) response to a request.
type JSONRPCResponse struct {
	Jsonrpc interface{} `json:"jsonrpc"`
	Id RequestId `json:"id"`
	Result Result `json:"result"`
}

// JSONRPCError A response to a request that indicates an error occurred.
type JSONRPCError struct {
	Jsonrpc interface{} `json:"jsonrpc"`
	Id RequestId `json:"id"`
	Error interface{} `json:"error"`
}

// CancelledNotification / Cancellation This notification can be sent by either side to indicate that it is cancelling a previously-issued request.
 
  The request SHOULD still be in-flight, but due to communication latency, it is always possible that this notification MAY arrive after the request has already finished.
 
  This notification indicates that the result will be unused, so any associated processing SHOULD cease.
 
  A client MUST NOT attempt to cancel its `initialize` request.
type CancelledNotification struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// InitializeRequest / Initialization This request is sent from the client to the server when it first connects, asking it to begin initialization.
type InitializeRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// InitializeResult After receiving an initialize request from the client, the server sends this response.
type InitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo Implementation `json:"serverInfo"`
	Instructions string `json:"instructions,omitempty"`
}

// InitializedNotification This notification is sent from the client to the server after initialization has finished.
type InitializedNotification struct {
	Method interface{} `json:"method"`
}

// ClientCapabilities Capabilities a client may support. Known capabilities are defined here, in this schema, but this is not a closed set: any client can define its own, additional capabilities.
type ClientCapabilities struct {
	Experimental interface{} `json:"experimental,omitempty"`
	Roots interface{} `json:"roots,omitempty"`
	Sampling interface{} `json:"sampling,omitempty"`
}

// ServerCapabilities Capabilities that a server may support. Known capabilities are defined here, in this schema, but this is not a closed set: any server can define its own, additional capabilities.
type ServerCapabilities struct {
	Experimental interface{} `json:"experimental,omitempty"`
	Logging interface{} `json:"logging,omitempty"`
	Completions interface{} `json:"completions,omitempty"`
	Prompts interface{} `json:"prompts,omitempty"`
	Resources interface{} `json:"resources,omitempty"`
	Tools interface{} `json:"tools,omitempty"`
}

// Implementation Describes the name and version of an MCP implementation.
type Implementation struct {
	Name string `json:"name"`
	Version string `json:"version"`
}

// PingRequest / Ping A ping, issued by either the server or the client, to check that the other party is still alive. The receiver must promptly respond, or else may be disconnected.
type PingRequest struct {
	Method interface{} `json:"method"`
}

// ProgressNotification / Progress notifications An out-of-band notification used to inform the receiver of a progress update for a long-running request.
type ProgressNotification struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// PaginatedRequest / Pagination
type PaginatedRequest struct {
	Params interface{} `json:"params,omitempty"`
}

// PaginatedResult 
type PaginatedResult struct {
	NextCursor Cursor `json:"nextCursor,omitempty"`
}

// ListResourcesRequest / Resources Sent from the client to request a list of resources the server has.
type ListResourcesRequest struct {
	Method interface{} `json:"method"`
}

// ListResourcesResult The server's response to a resources/list request from the client.
type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

// ListResourceTemplatesRequest Sent from the client to request a list of resource templates the server has.
type ListResourceTemplatesRequest struct {
	Method interface{} `json:"method"`
}

// ListResourceTemplatesResult The server's response to a resources/templates/list request from the client.
type ListResourceTemplatesResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
}

// ReadResourceRequest Sent from the client to the server, to read a specific resource URI.
type ReadResourceRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// ReadResourceResult The server's response to a resources/read request from the client.
type ReadResourceResult struct {
	Contents []interface{} `json:"contents"`
}

// ResourceListChangedNotification An optional notification from the server to the client, informing it that the list of resources it can read from has changed. This may be issued by servers without any previous subscription from the client.
type ResourceListChangedNotification struct {
	Method interface{} `json:"method"`
}

// SubscribeRequest Sent from the client to request resources/updated notifications from the server whenever a particular resource changes.
type SubscribeRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// UnsubscribeRequest Sent from the client to request cancellation of resources/updated notifications from the server. This should follow a previous resources/subscribe request.
type UnsubscribeRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// ResourceUpdatedNotification A notification from the server to the client, informing it that a resource has changed and may need to be read again. This should only be sent if the client previously sent a resources/subscribe request.
type ResourceUpdatedNotification struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// Resource A known resource that the server is capable of reading.
type Resource struct {
	Uri string `json:"uri"`
	Name string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Annotations Annotations `json:"annotations,omitempty"`
	Size float64 `json:"size,omitempty"`
}

// ResourceTemplate A template description for resources available on the server.
type ResourceTemplate struct {
	UriTemplate string `json:"uriTemplate"`
	Name string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Annotations Annotations `json:"annotations,omitempty"`
}

// ResourceContents The contents of a specific resource or sub-resource.
type ResourceContents struct {
	Uri string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
}

// TextResourceContents 
type TextResourceContents struct {
	Text string `json:"text"`
}

// BlobResourceContents 
type BlobResourceContents struct {
	Blob string `json:"blob"`
}

// ListPromptsRequest / Prompts Sent from the client to request a list of prompts and prompt templates the server has.
type ListPromptsRequest struct {
	Method interface{} `json:"method"`
}

// ListPromptsResult The server's response to a prompts/list request from the client.
type ListPromptsResult struct {
	Prompts []Prompt `json:"prompts"`
}

// GetPromptRequest Used by the client to get a prompt provided by the server.
type GetPromptRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// GetPromptResult The server's response to a prompts/get request from the client.
type GetPromptResult struct {
	Description string `json:"description,omitempty"`
	Messages []PromptMessage `json:"messages"`
}

// Prompt A prompt or prompt template that the server offers.
type Prompt struct {
	Name string `json:"name"`
	Description string `json:"description,omitempty"`
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument Describes an argument that a prompt can accept.
type PromptArgument struct {
	Name string `json:"name"`
	Description string `json:"description,omitempty"`
	Required bool `json:"required,omitempty"`
}

// PromptMessage Describes a message returned as part of a prompt.
 
  This is similar to `SamplingMessage`, but also supports the embedding of
  resources from the MCP server.
type PromptMessage struct {
	Role Role `json:"role"`
	Content interface{} `json:"content"`
}

// EmbeddedResource The contents of a resource, embedded into a prompt or tool call result.
 
  It is up to the client how best to render embedded resources for the benefit
  of the LLM and/or the user.
type EmbeddedResource struct {
	Type interface{} `json:"type"`
	Resource interface{} `json:"resource"`
	Annotations Annotations `json:"annotations,omitempty"`
}

// PromptListChangedNotification An optional notification from the server to the client, informing it that the list of prompts it offers has changed. This may be issued by servers without any previous subscription from the client.
type PromptListChangedNotification struct {
	Method interface{} `json:"method"`
}

// ListToolsRequest / Tools Sent from the client to request a list of tools the server has.
type ListToolsRequest struct {
	Method interface{} `json:"method"`
}

// ListToolsResult The server's response to a tools/list request from the client.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolResult The server's response to a tool call.
 
  Any errors that originate from the tool SHOULD be reported inside the result
  object, with `isError` set to true, _not_ as an MCP protocol-level error
  response. Otherwise, the LLM would not be able to see that an error occurred
  and self-correct.
 
  However, any errors in _finding_ the tool, an error indicating that the
  server does not support tool calls, or any other exceptional conditions,
  should be reported as an MCP error response.
type CallToolResult struct {
	Content []interface{} `json:"content"`
	IsError bool `json:"isError,omitempty"`
}

// CallToolRequest Used by the client to invoke a tool provided by the server.
type CallToolRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// ToolListChangedNotification An optional notification from the server to the client, informing it that the list of tools it offers has changed. This may be issued by servers without any previous subscription from the client.
type ToolListChangedNotification struct {
	Method interface{} `json:"method"`
}

// ToolAnnotations Additional properties describing a Tool to clients.
 
  NOTE: all properties in ToolAnnotations are hints.
  They are not guaranteed to provide a faithful description of
  tool behavior (including descriptive properties like `title`).
 
  Clients should never make tool use decisions based on ToolAnnotations
  received from untrusted servers.
type ToolAnnotations struct {
	Title string `json:"title,omitempty"`
	ReadOnlyHint bool `json:"readOnlyHint,omitempty"`
	DestructiveHint bool `json:"destructiveHint,omitempty"`
	IdempotentHint bool `json:"idempotentHint,omitempty"`
	OpenWorldHint bool `json:"openWorldHint,omitempty"`
}

// Tool Definition for a tool the client can call.
type Tool struct {
	Name string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema"`
	Annotations ToolAnnotations `json:"annotations,omitempty"`
}

// SetLevelRequest / Logging A request from the client to the server, to enable or adjust logging.
type SetLevelRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// LoggingMessageNotification Notification of a log message passed from server to client. If no logging/setLevel request has been sent from the client, the server MAY decide which messages to send automatically.
type LoggingMessageNotification struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// CreateMessageRequest / Sampling A request from the server to sample an LLM via the client. The client has full discretion over which model to select. The client should also inform the user before beginning sampling, to allow them to inspect the request (human in the loop) and decide whether to approve it.
type CreateMessageRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// CreateMessageResult The client's response to a sampling/create_message request from the server. The client should inform the user before returning the sampled message, to allow them to inspect the response (human in the loop) and decide whether to allow the server to see it.
type CreateMessageResult struct {
	Model string `json:"model"`
	StopReason interface{} `json:"stopReason,omitempty"`
}

// SamplingMessage Describes a message issued to or received from an LLM API.
type SamplingMessage struct {
	Role Role `json:"role"`
	Content interface{} `json:"content"`
}

// Annotations Optional annotations for the client. The client can use annotations to inform how objects are used or displayed
type Annotations struct {
	Audience []Role `json:"audience,omitempty"`
	Priority float64 `json:"priority,omitempty"`
}

// TextContent Text provided to or from an LLM.
type TextContent struct {
	Type interface{} `json:"type"`
	Text string `json:"text"`
	Annotations Annotations `json:"annotations,omitempty"`
}

// ImageContent An image provided to or from an LLM.
type ImageContent struct {
	Type interface{} `json:"type"`
	Data string `json:"data"`
	MimeType string `json:"mimeType"`
	Annotations Annotations `json:"annotations,omitempty"`
}

// AudioContent Audio provided to or from an LLM.
type AudioContent struct {
	Type interface{} `json:"type"`
	Data string `json:"data"`
	MimeType string `json:"mimeType"`
	Annotations Annotations `json:"annotations,omitempty"`
}

// ModelPreferences The server's preferences for model selection, requested of the client during sampling.
 
  Because LLMs can vary along multiple dimensions, choosing the "best" model is
  rarely straightforward.  Different models excel in different areas—some are
  faster but less capable, others are more capable but more expensive, and so
  on. This interface allows servers to express their priorities across multiple
  dimensions to help clients make an appropriate selection for their use case.
 
  These preferences are always advisory. The client MAY ignore them. It is also
  up to the client to decide how to interpret these preferences and how to
  balance them against other considerations.
type ModelPreferences struct {
	Hints []ModelHint `json:"hints,omitempty"`
	CostPriority float64 `json:"costPriority,omitempty"`
	SpeedPriority float64 `json:"speedPriority,omitempty"`
	IntelligencePriority float64 `json:"intelligencePriority,omitempty"`
}

// ModelHint Hints to use for model selection.
 
  Keys not declared here are currently left unspecified by the spec and are up
  to the client to interpret.
type ModelHint struct {
	Name string `json:"name,omitempty"`
}

// CompleteRequest / Autocomplete A request from the client to the server, to ask for completion options.
type CompleteRequest struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
}

// CompleteResult The server's response to a completion/complete request
type CompleteResult struct {
	Completion interface{} `json:"completion"`
}

// ResourceReference A reference to a resource or resource template definition.
type ResourceReference struct {
	Type interface{} `json:"type"`
	Uri string `json:"uri"`
}

// PromptReference Identifies a prompt.
type PromptReference struct {
	Type interface{} `json:"type"`
	Name string `json:"name"`
}

// ListRootsRequest / Roots Sent from the server to request a list of root URIs from the client. Roots allow
  servers to ask for specific directories or files to operate on. A common example
  for roots is providing a set of repositories or directories a server should operate
  on.
 
  This request is typically used when the server needs to understand the file system
  structure or access specific locations that the client has permission to read from.
type ListRootsRequest struct {
	Method interface{} `json:"method"`
}

// ListRootsResult The client's response to a roots/list request from the server.
  This result contains an array of Root objects, each representing a root directory
  or file that the server can operate on.
type ListRootsResult struct {
	Roots []Root `json:"roots"`
}

// Root Represents a root directory or file that the server can operate on.
type Root struct {
	Uri string `json:"uri"`
	Name string `json:"name,omitempty"`
}

// RootsListChangedNotification A notification from the client to the server, informing it that the list of roots has changed.
  This notification should be sent whenever the client adds, removes, or modifies any root.
  The server should then request an updated list of roots using the ListRootsRequest.
type RootsListChangedNotification struct {
	Method interface{} `json:"method"`
}

