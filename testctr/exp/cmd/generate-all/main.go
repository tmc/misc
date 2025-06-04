package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"generate-all/parser"
)

var (
	outputDir = flag.String("out", "./modules", "Output directory for generated modules")
	verbose   = flag.Bool("v", false, "Verbose output")
)

func main() {
	flag.Parse()

	// Modules to generate
	modules := []string{"mysql", "postgres", "redis", "mongodb", "qdrant"}

	fmt.Printf("Generating testctr modules to %s\n", *outputDir)

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Generate each module
	for _, module := range modules {
		moduleDir := filepath.Join(*outputDir, module)
		
		if *verbose {
			fmt.Printf("Generating %s module...\n", module)
		}

		// Create module directory
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			log.Fatalf("Failed to create module directory %s: %v", moduleDir, err)
		}

		// Generate the module files using our parser package
		if err := parser.GenerateModuleFiles(module, moduleDir); err != nil {
			log.Fatalf("Failed to generate %s module: %v", module, err)
		}

		if *verbose {
			fmt.Printf("✓ Generated %s module in %s\n", module, moduleDir)
		}
	}

	fmt.Printf("✓ Successfully generated %d modules in %s\n", len(modules), *outputDir)
	fmt.Println("\nModules generated:")
	for _, module := range modules {
		fmt.Printf("  - %s/\n", module)
		fmt.Printf("    ├── %s.go\n", module)
		fmt.Printf("    ├── doc.go\n")
		fmt.Printf("    └── %s_test.go\n", module)
	}
	
	fmt.Println("\nUsage example:")
	fmt.Printf("  import \"path/to/%s/mysql\"\n", *outputDir)
	fmt.Printf("  container := testctr.New(t, \"mysql:8.0.36\", mysql.Default())\n")
}