// Package config provides configuration for ts2go
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// Config holds configuration for type mappings and generation options
type Config struct {
	// Map of TypeScript type names to their Go equivalents
	TypeMappings map[string]string `json:"typeMappings"`

	// Whether to use pointers for optional struct fields
	UsePointersForOptionalFields bool `json:"usePointersForOptionalFields"`

	// List of known initialisms to properly format (e.g., "ID", "URL")
	Initialisms []string `json:"initialisms"`

	// Custom imports to include
	CustomImports []string `json:"customImports"`

	// SpecialTypes defines special types that need custom code generation
	SpecialTypes []SpecialType `json:"specialTypes"`

	// Transformers enables/disables specific transformers
	Transformers TransformerConfig `json:"transformers"`

	// PluginPaths contains paths to Go plugin files that implement TypeTransformer
	PluginPaths []string `json:"pluginPaths"`
}

// SpecialType represents a special type that needs custom handling
type SpecialType struct {
	// TypeName is the TypeScript type name
	TypeName string `json:"typeName"`

	// GoTypeName is the Go type name
	GoTypeName string `json:"goTypeName"`

	// IsInterface indicates if this is an interface rather than a struct
	IsInterface bool `json:"isInterface"`

	// Methods defines methods to implement on this type
	Methods []Method `json:"methods"`

	// Fields defines fields for struct types
	Fields []Field `json:"fields"`

	// Imports are additional imports needed for this type
	Imports []string `json:"imports"`

	// CustomCode is raw code to include for this type
	CustomCode string `json:"customCode"`
}

// Method defines a method on a type
type Method struct {
	// Name is the method name
	Name string `json:"name"`

	// Signature is the full method signature, e.g., "func (t TypeName) MethodName() ReturnType"
	Signature string `json:"signature"`

	// Body is the method body
	Body string `json:"body"`
}

// Field defines a field in a struct
type Field struct {
	// Name is the field name in Go
	Name string `json:"name"`

	// Type is the Go type
	Type string `json:"type"`

	// JSONName is the name in JSON (defaults to Name)
	JSONName string `json:"jsonName"`

	// Optional indicates if the field is optional (for omitempty tag)
	Optional bool `json:"optional"`

	// Description is a description of the field
	Description string `json:"description"`
}

// TransformerConfig enables/disables specific transformers
type TransformerConfig struct {
	// EnableDefault enables the default transformer
	EnableDefault bool `json:"enableDefault"`

	// EnableMCP enables the MCP transformer
	EnableMCP bool `json:"enableMCP"`
}

// NewDefaultConfig returns a default configuration
func NewDefaultConfig() *Config {
	return &Config{
		TypeMappings: map[string]string{
			"string":    "string",
			"number":    "float64",
			"boolean":   "bool",
			"any":       "interface{}",
			"void":      "struct{}",
			"null":      "nil",
			"undefined": "nil",
			"object":    "map[string]interface{}",
		},
		UsePointersForOptionalFields: true,
		Initialisms: []string{
			"ID", "URL", "URI", "JSON", "XML", "HTTP", "HTML", "API",
			"SQL", "RPC", "TCP", "UDP", "IP", "DNS", "EOF", "UUID", "MIME",
		},
		CustomImports: []string{"encoding/json"},
		Transformers: TransformerConfig{
			EnableDefault: true,
			EnableMCP:     false,
		},
	}
}

// LoadConfig loads configuration from a file or returns default config
func LoadConfig(configPath string) *Config {
	config := NewDefaultConfig()

	if configPath == "" {
		// Try to find config in default locations
		possiblePaths := []string{
			"ts2go.config.json",
			"ts2go.json",
			".ts2go.json",
			filepath.Join(os.Getenv("HOME"), ".ts2go/config.json"),
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}

		if configPath == "" {
			return config
		}
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("Warning: Could not read config file %s: %v", configPath, err)
		return config
	}

	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Warning: Could not parse config file %s: %v", configPath, err)
		return config
	}

	return config
}

// SaveConfig saves the configuration to a file
func SaveConfig(config *Config, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// GenerateDefaultConfig generates a default configuration file
func GenerateDefaultConfig(configPath string) error {
	config := NewDefaultConfig()
	return SaveConfig(config, configPath)
}
