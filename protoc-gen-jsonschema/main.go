package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/misc/protoc-gen-jsonschema/jsonschema"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	
	// Output options
	outputDir := flags.String("output_dir", "", "output directory for the generated schema files")
	fileExtension := flags.String("file_extension", "json", "file extension for output schema files")
	indent := flags.Bool("indent", true, "indent the output JSON")
	
	// Schema behavior options
	includeNullable := flags.Bool("nullable", false, "set whether fields should be nullable by default")
	embedDefinitions := flags.Bool("embed_defs", false, "whether to embed definitions in the schema or use references")
	allFieldsRequired := flags.Bool("all_fields_required", false, "mark all fields as required")
	disallowAdditionalProps := flags.Bool("disallow_additional_properties", false, "disallow additional properties in objects")
	
	// Field options
	jsonFieldnames := flags.Bool("json_fieldnames", true, "use JSON field names instead of protobuf field names")
	enumsAsConstants := flags.Bool("enums_as_constants", false, "represent enums as constants rather than strings with enum values")
	bigIntsAsStrings := flags.Bool("bigints_as_strings", true, "represent 64-bit integers as strings")
	prefixWithPackage := flags.Bool("prefix_schema_files_with_package", false, "prefix schema filenames with package name")
	requireOneOfFields := flags.Bool("enforce_oneof", false, "enforce oneof fields with JSON Schema oneOf")
	
	// Debug options
	debug := flags.Bool("debug", false, "enable debug logging")
	
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}
	
	opts.Run(func(p *protogen.Plugin) error {
		// Parse comma-separated parameters (traditional protoc style)
		pluginParams := make(map[string]bool)
		specificMessages := make(map[string]bool)
		
		for _, param := range strings.Split(p.Request.GetParameter(), ",") {
			param = strings.TrimSpace(param)
			if param == "debug" {
				*debug = true
			} else if param == "all_fields_required" {
				*allFieldsRequired = true
			} else if param == "allow_null_values" || param == "nullable" {
				*includeNullable = true
			} else if param == "disallow_additional_properties" {
				*disallowAdditionalProps = true
			} else if param == "disallow_bigints_as_strings" {
				*bigIntsAsStrings = false
			} else if param == "prefix_schema_files_with_package" {
				*prefixWithPackage = true
			} else if param == "enums_as_constants" {
				*enumsAsConstants = true
			} else if param == "enforce_oneof" {
				*requireOneOfFields = true
			} else if strings.HasPrefix(param, "file_extension=") {
				*fileExtension = strings.TrimPrefix(param, "file_extension=")
			} else if strings.HasPrefix(param, "messages=") {
				// Parse messages=[Message1+Message2+Message3] format
				messagesStr := strings.TrimPrefix(param, "messages=")
				messagesStr = strings.TrimPrefix(messagesStr, "[")
				messagesStr = strings.TrimSuffix(messagesStr, "]")
				
				for _, msgName := range strings.Split(messagesStr, "+") {
					specificMessages[strings.TrimSpace(msgName)] = true
				}
				
				if *debug {
					fmt.Fprintf(os.Stderr, "Filtering to specific messages: %v\n", specificMessages)
				}
			} else if param != "" {
				pluginParams[param] = true
			}
		}
		
		// Create the output directory if it doesn't exist
		if *outputDir != "" {
			if err := os.MkdirAll(*outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %v", err)
			}
		}
		
		// Create generator with configured options
		generator := jsonschema.NewGenerator(*includeNullable, *embedDefinitions)
		generator.AllFieldsRequired = *allFieldsRequired
		generator.DisallowAdditionalProps = *disallowAdditionalProps
		generator.JSONFieldnames = *jsonFieldnames
		generator.EnumsAsConstants = *enumsAsConstants
		generator.FileExtension = *fileExtension
		generator.PrefixWithPackage = *prefixWithPackage
		generator.BigIntsAsStrings = *bigIntsAsStrings
		generator.IncludeRequiredOneOfFields = !*requireOneOfFields
		
		if *debug {
			fmt.Fprintf(os.Stderr, "Configuration: %+v\n", generator)
		}
		
		// Process each proto file
		for _, file := range p.Files {
			if !file.Generate {
				continue
			}
			
			// Generate schema for each message
			for _, msg := range file.Messages {
				// Skip map entry messages (they are handled differently)
				if msg.Desc.IsMapEntry() {
					continue
				}
				
				// Skip if we're only generating specific messages
				if len(specificMessages) > 0 && !specificMessages[msg.GoIdent.GoName] {
					if *debug {
						fmt.Fprintf(os.Stderr, "Skipping message %s (not in specific messages list)\n", msg.GoIdent.GoName)
					}
					continue
				}
				
				// Generate the JSON schema
				jsonData, err := generator.GenerateJSON(msg, *indent)
				if err != nil {
					return fmt.Errorf("failed to generate schema for %s: %v", msg.GoIdent.GoName, err)
				}
				
				// Determine output file name
				outputFile := msg.GoIdent.GoName
				if *prefixWithPackage && file.Desc.Package() != "" {
					outputFile = string(file.Desc.Package()) + "." + outputFile
				}
				outputFile = outputFile + "." + *fileExtension
				
				if *debug {
					fmt.Fprintf(os.Stderr, "Generating schema for %s -> %s\n", msg.GoIdent.GoName, outputFile)
				}
				
				if *outputDir != "" {
					// Write to file system directly
					outputPath := filepath.Join(*outputDir, outputFile)
					if err := ioutil.WriteFile(outputPath, jsonData, 0644); err != nil {
						return fmt.Errorf("failed to write schema file: %v", err)
					}
				} else {
					// Use the protogen API to create output
					genFile := p.NewGeneratedFile(outputFile, "")
					if _, err := genFile.Write(jsonData); err != nil {
						return fmt.Errorf("failed to write to generated file: %v", err)
					}
				}
			}
			
			// Also generate schema for top-level enums if requested
			if pluginParams["generate_enum_schemas"] {
				if *debug {
					fmt.Fprintf(os.Stderr, "Enum schema generation not yet implemented\n")
					fmt.Fprintf(os.Stderr, "Found %d top-level enums\n", len(file.Enums))
				}
				// TODO: Implement enum schema generation
			}
		}
		
		return nil
	})
}