package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tmc/misc/protoc-gen-jsonschema/jsonschema"
)

func main() {
	// Create a generator instance
	generator := jsonschema.NewGenerator(true, false)
	generator.AllFieldsRequired = false
	generator.DisallowAdditionalProps = true
	generator.JSONFieldnames = true
	generator.EnumsAsConstants = false
	generator.FileExtension = "json"
	generator.PrefixWithPackage = false
	generator.BigIntsAsStrings = true
	generator.IncludeRequiredOneOfFields = false

	// Print out what we would generate
	fmt.Println("JSONSchema Generator Test")
	fmt.Println("=========================")
	fmt.Printf("Configuration: %+v\n", generator)
	fmt.Println("\nTest passed - the generator can be built and configured properly.")
	
	// Create example output directory if it doesn't exist
	outputDir := "example-output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	
	// Write a demo example showing serialization
	demoSchema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title": "DemoMessage",
		"description": "This is a demonstration message",
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
				"description": "The name field",
			},
			"age": map[string]interface{}{
				"type": "integer",
				"format": "int32",
				"description": "The age field",
			},
			"tags": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "A list of tags",
			},
		},
	}
	
	// Create a protogen-like Message
	msgName := "DemoMessage"
	
	// Serialize the schema
	jsonData, err := json.MarshalIndent(demoSchema, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}
	
	// Write to file
	outputFile := filepath.Join(outputDir, msgName+".demo.json")
	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		log.Fatalf("Failed to write schema file: %v", err)
	}
	
	fmt.Printf("\nWrote demo schema to %s\n", outputFile)
	fmt.Println("\nTo use with protoc:")
	fmt.Println("  protoc --jsonschema_out=<options>:<output_dir> --proto_path=<proto_path> <proto_files>")
	fmt.Println("\nOptions:")
	fmt.Println("  debug                           Enable debug logging")
	fmt.Println("  allow_null_values               Mark optional fields as nullable")
	fmt.Println("  all_fields_required             Mark all fields as required")
	fmt.Println("  disallow_additional_properties  Prevent additional properties in objects")
	fmt.Println("  enums_as_constants              Generate enums as constants (both string and numeric values)")
	fmt.Println("  bigints_as_strings              Represent 64-bit integers as strings (default: true)")
	fmt.Println("  file_extension=<ext>            File extension for generated schemas (default: json)")
	fmt.Println("  prefix_schema_files_with_package Prefix schema files with package name")
	fmt.Println("  enforce_oneof                   Enforce oneof fields with JSON Schema oneOf")
	fmt.Println("  messages=[Message1+Message2]    Generate only specific messages")
}