package types

import (
	"encoding/json"
	"fmt"
)

// Content is the wire format for content.
//
// The Type field distinguishes the type of the content.
// At most one of Text, MIMEType, Data, and Resource is non-zero.
type Content struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	MIMEType string    `json:"mimeType,omitempty"`
	Data     string    `json:"data,omitempty"`
	Resource *Resource `json:"resource,omitempty"`
}

// Resource is the wire format for embedded resources.
//
// The URI field describes the resource location. At most one of Text and Blob
// is non-zero.
type Resource struct {
	URI      string  `json:"uri"`
	MIMEType string  `json:"mimeType,omitempty"`
	Text     string  `json:"text,omitempty"`
	Blob     *string `json:"blob,omitempty"` // blob is a pointer to distinguish empty from missing data
}

// UnmarshalJSON implements custom unmarshaling to capture content types
func (c *Content) UnmarshalJSON(data []byte) error {
	// Use a type alias to avoid infinite recursion
	type wireContent Content
	var c2 wireContent
	if err := json.Unmarshal(data, &c2); err != nil {
		return err
	}
	switch c2.Type {
	case "text", "image", "audio", "resource":
	default:
		return fmt.Errorf("unrecognized content type %s", c.Type)
	}
	*c = Content(c2)
	return nil
}

// TextContent represents text provided to or from an LLM.
type TextContent struct {
	Type        string      `json:"type"`
	Text        string      `json:"text"`
	Annotations interface{} `json:"annotations,omitempty"`
}

// ImageContent represents an image provided to or from an LLM.
type ImageContent struct {
	Type        string      `json:"type"`
	Data        string      `json:"data"`
	MIMEType    string      `json:"mimeType"`
	Annotations interface{} `json:"annotations,omitempty"`
}

// AudioContent represents audio provided to or from an LLM.
type AudioContent struct {
	Type        string      `json:"type"`
	Data        string      `json:"data"`
	MIMEType    string      `json:"mimeType"`
	Annotations interface{} `json:"annotations,omitempty"`
}

// EmbeddedResource represents a resource embedded within content.
type EmbeddedResource struct {
	Type        string      `json:"type"`
	Resource    interface{} `json:"resource"`
	Annotations interface{} `json:"annotations,omitempty"`
}
