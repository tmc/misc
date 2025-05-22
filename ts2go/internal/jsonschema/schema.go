package jsonschema

import (
	"encoding/json"
)

// Schema represents a JSON Schema object
type Schema struct {
	// Core schema metadata
	ID          string `json:"id,omitempty"`      // schema ID
	Schema      string `json:"$schema,omitempty"` // schema version
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`

	// Type constraints
	MultipleOf       *float64 `json:"multipleOf,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMaximum bool     `json:"exclusiveMaximum,omitempty"`
	Minimum          *float64 `json:"minimum,omitempty"`
	ExclusiveMinimum bool     `json:"exclusiveMinimum,omitempty"`
	MaxLength        *int64   `json:"maxLength,omitempty"`
	MinLength        *int64   `json:"minLength,omitempty"`
	Pattern          string   `json:"pattern,omitempty"`
	Format           string   `json:"format,omitempty"`

	// Object constraints
	Properties           map[string]*Schema     `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	MaxProperties        *int64                 `json:"maxProperties,omitempty"`
	MinProperties        *int64                 `json:"minProperties,omitempty"`
	AdditionalProperties *Schema                `json:"additionalProperties,omitempty"`
	PatternProperties    map[string]*Schema     `json:"patternProperties,omitempty"`
	Dependencies         map[string]interface{} `json:"dependencies,omitempty"`

	// Array constraints
	Items           *Schema `json:"items,omitempty"`
	AdditionalItems *Schema `json:"additionalItems,omitempty"`
	MaxItems        *int64  `json:"maxItems,omitempty"`
	MinItems        *int64  `json:"minItems,omitempty"`
	UniqueItems     bool    `json:"uniqueItems,omitempty"`

	// Enumerated values
	Enum []interface{} `json:"enum,omitempty"`

	// Compound schemas
	AllOf []*Schema `json:"allOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`
	Not   *Schema   `json:"not,omitempty"`

	// Schema references
	Ref         string             `json:"$ref,omitempty"`
	Definitions map[string]*Schema `json:"definitions,omitempty"`

	// Extension keywords
	Extensions map[string]json.RawMessage `json:"-"`

	// Allow Schema to capture all unknown fields
	RawExtensions map[string]json.RawMessage `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling to capture extensions
func (s *Schema) UnmarshalJSON(data []byte) error {
	// Use a type alias to avoid infinite recursion when unmarshaling
	type Alias Schema
	a := &struct {
		*Alias
		RawExtensions map[string]json.RawMessage `json:"-"`
	}{
		Alias: (*Alias)(s),
	}

	// Unmarshal into the alias to get standard properties
	if err := json.Unmarshal(data, a); err != nil {
		return err
	}

	// Unmarshal again into a map to capture all fields
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// Collect all non-standard fields as extensions
	if s.Extensions == nil {
		s.Extensions = make(map[string]json.RawMessage)
	}

	for key, value := range rawMap {
		// Skip standard fields
		switch key {
		case "id", "$schema", "description", "type", "multipleOf", "maximum",
			"exclusiveMaximum", "minimum", "exclusiveMinimum", "maxLength",
			"minLength", "pattern", "format", "properties", "required",
			"maxProperties", "minProperties", "additionalProperties",
			"patternProperties", "dependencies", "items", "additionalItems",
			"maxItems", "minItems", "uniqueItems", "enum", "allOf", "anyOf",
			"oneOf", "not", "$ref", "definitions":
			continue
		}

		// Add to extensions
		s.Extensions[key] = value
	}

	return nil
}

// MarshalJSON implements custom marshaling for Schema
func (s *Schema) MarshalJSON() ([]byte, error) {
	// Use a type alias to avoid infinite recursion when marshaling
	type Alias Schema
	a := struct {
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	// Marshal the standard fields
	data, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}

	// If there are no extensions, return the data as is
	if len(s.Extensions) == 0 {
		return data, nil
	}

	// Unmarshal into a map to add extensions
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, err
	}

	// Add extensions to the map
	for key, value := range s.Extensions {
		rawMap[key] = value
	}

	// Marshal the map back to JSON
	return json.Marshal(rawMap)
}
