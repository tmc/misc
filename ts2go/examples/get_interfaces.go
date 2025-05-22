// This file demonstrates extracting TypeScript interfaces from a file
package mcp

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/tmc/misc/ts2go/internal/cdn"
	v8 "rogchap.com/v8go"
)

func main() {
	inFile := flag.String("in", "", "Input .ts file to parse")
	flag.Parse()

	if *inFile == "" {
		fmt.Println("Usage: go run examples/get_interfaces.go -in=path/to/typescript_file.ts")
		os.Exit(1)
	}

	// Get the TypeScript library from CDN
	tsLibPath, err := cdn.FetchTypeScript()
	if err != nil {
		log.Fatalf("Failed to fetch TypeScript from CDN: %v", err)
	}

	// Create a context and load TypeScript
	ctx := v8.NewContext()
	defer ctx.Close()

	// Read TypeScript library
	tsSource, err := os.ReadFile(tsLibPath)
	if err != nil {
		log.Fatalf("Failed to read typescript.js: %v", err)
	}

	// Load TypeScript into context
	if _, err := ctx.RunScript(string(tsSource), "typescript.js"); err != nil {
		log.Fatalf("Failed to load typescript.js: %v", err)
	}

	// Read input TypeScript file
	tsCode, err := os.ReadFile(*inFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// Parse script to extract interfaces
	script := fmt.Sprintf(`
	(function() {
		// Create source file
		const sourceFile = ts.createSourceFile(
			"%s",
			%q,
			ts.ScriptTarget.Latest,
			true
		);

		// Function to collect interfaces
		function findInterfaces(node) {
			let interfaces = [];
			
			// Check if this node is an interface declaration
			if (node.kind === ts.SyntaxKind.InterfaceDeclaration) {
				const interfaceInfo = {
					name: node.name.text,
					members: []
				};
				
				// Get interface members
				if (node.members) {
					node.members.forEach(member => {
						if (member.name) {
							const memberInfo = {
								name: member.name.text,
								kind: ts.SyntaxKind[member.kind],
							};
							
							// Get type information if available
							if (member.type) {
								memberInfo.type = ts.SyntaxKind[member.type.kind];
								if (member.type.typeName) {
									memberInfo.typeName = member.type.typeName.text;
								}
							}
							
							interfaceInfo.members.push(memberInfo);
						}
					});
				}
				
				interfaces.push(interfaceInfo);
			}
			
			// Recursively search through child nodes
			ts.forEachChild(node, child => {
				interfaces = interfaces.concat(findInterfaces(child));
			});
			
			return interfaces;
		}
		
		// Find all interfaces in the source file
		const interfaces = findInterfaces(sourceFile);
		return JSON.stringify(interfaces);
	})();
	`, *inFile, string(tsCode))

	// Execute the script
	result, err := ctx.RunScript(script, "extract-interfaces.js")
	if err != nil {
		log.Fatalf("Failed to extract interfaces: %v", err)
	}

	// Parse the JSON result
	var interfaces []interface{}
	if err := json.Unmarshal([]byte(result.String()), &interfaces); err != nil {
		log.Fatalf("Failed to parse interfaces JSON: %v", err)
	}

	// Pretty print the result
	jsonData, err := json.MarshalIndent(interfaces, "", "  ")
	if err != nil {
		log.Fatalf("Failed to format interfaces JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}
