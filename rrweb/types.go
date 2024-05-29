package rrweb

// Recording represents a recording.
type Recording []EventWithTime

// NodeType represents the different types of nodes
type NodeType int

const (
	Document NodeType = iota
	DocumentType
	Element
	Text
	CDATA
	Comment
)

// note: The types below are direct from rrweb

// EventWithTime represents an event with a timestamp
type EventWithTime struct {
	EventWithoutTime
	Timestamp int64 `json:"timestamp"`
	Delay     *int  `json:"delay,omitempty"`
}

// EventWithoutTime represents an event without a timestamp
type EventWithoutTime struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

// SerializedNode represents a serialized node
type SerializedNode struct {
	Type         NodeType               `json:"type"`
	TagName      string                 `json:"tagName,omitempty"`
	RootID       *int                   `json:"rootId,omitempty"`
	IsShadowHost bool                   `json:"isShadowHost,omitempty"`
	IsShadow     bool                   `json:"isShadow,omitempty"`
	ChildNodes   []SerializedNodeWithId `json:"childNodes,omitempty"`
	TextContent  string                 `json:"textContent,omitempty"`
	Attributes   map[string]string      `json:"attributes,omitempty"`
}

// SerializedNodeWithId represents a serialized node with an ID
type SerializedNodeWithId struct {
	SerializedNode
	ID int `json:"id"`
}

// InitialOffset represents the initial offset
type InitialOffset struct {
	Top  int `json:"top"`
	Left int `json:"left"`
}

// FullSnapshotEventData represents the data for a FullSnapshot event
type FullSnapshotEventData struct {
	Node          SerializedNodeWithId `json:"node"`
	InitialOffset InitialOffset        `json:"initialOffset"`
}

// MetaEventData represents the data for a Meta event
type MetaEventData struct {
	Href   string `json:"href"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// CustomEventData represents the data for a Custom event
type CustomEventData struct {
	Tag     string      `json:"tag"`
	Payload interface{} `json:"payload"`
}

// PluginEventData represents the data for a Plugin event
type PluginEventData struct {
	Plugin  string      `json:"plugin"`
	Payload interface{} `json:"payload"`
}

// EventType represents the different types of events
type EventType int

const (
	DomContentLoaded EventType = iota
	Load
	FullSnapshot
	IncrementalSnapshot
	Meta
	Custom
	Plugin
)

// IncrementalSource represents the source of incremental events
type IncrementalSource int

const (
	Mutation IncrementalSource = iota
	MouseMove
	MouseInteraction
	Scroll
	ViewportResize
	Input
	TouchMove
	MediaInteraction
	StyleSheetRule
	CanvasMutation
	Font
	Log
	Drag
	StyleDeclaration
	Selection
	AdoptedStyleSheet
	CustomElement
)

// TextMutationData represents the data for a text mutation event
type TextMutationData struct {
	Source IncrementalSource `json:"source"`
	ID     int               `json:"id"`
	Text   string            `json:"text"`
}

// ViewportResizeData represents the data for a viewport resize event
type ViewportResizeData struct {
	Source IncrementalSource `json:"source"`
	Width  int               `json:"width"`
	Height int               `json:"height"`
}

// IncrementalSnapshotEvent represents an incremental snapshot event
type IncrementalSnapshotEvent struct {
	Type EventType          `json:"type"`
	Data ViewportResizeData `json:"data"`
}
