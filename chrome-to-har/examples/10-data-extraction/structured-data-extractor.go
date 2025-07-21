// Structured data extraction example
// This example shows how to extract structured data from websites
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type StructuredData struct {
	URL           string                 `json:"url"`
	Timestamp     time.Time              `json:"timestamp"`
	Success       bool                   `json:"success"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	JSONLDData    []map[string]interface{} `json:"jsonld_data"`
	MicrodataData []map[string]interface{} `json:"microdata_data"`
	OpenGraphData map[string]string      `json:"opengraph_data"`
	TwitterCards  map[string]string      `json:"twitter_cards"`
	MetaData      map[string]string      `json:"meta_data"`
	SchemaOrg     []map[string]interface{} `json:"schema_org"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run structured-data-extractor.go <url>")
		fmt.Println("Example: go run structured-data-extractor.go https://example.com")
		os.Exit(1)
	}

	url := os.Args[1]
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create Chrome browser
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; StructuredDataExtractor/1.0)"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Extract structured data
	result := extractStructuredData(chromeCtx, url)
	
	// Display results
	displayResults(result)
	
	// Save report
	saveReport(result)
}

func extractStructuredData(chromeCtx context.Context, url string) StructuredData {
	startTime := time.Now()
	
	result := StructuredData{
		URL:           url,
		Timestamp:     startTime,
		Success:       false,
		OpenGraphData: make(map[string]string),
		TwitterCards:  make(map[string]string),
		MetaData:      make(map[string]string),
	}

	// Create recorder
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, 60*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for dynamic content
		
		// Extract JSON-LD data
		chromedp.Evaluate(`
			(function() {
				const jsonLdScripts = document.querySelectorAll('script[type="application/ld+json"]');
				const jsonLdData = [];
				
				jsonLdScripts.forEach(script => {
					try {
						const data = JSON.parse(script.textContent);
						jsonLdData.push(data);
					} catch (e) {
						console.error('Error parsing JSON-LD:', e);
					}
				});
				
				return jsonLdData;
			})()
		`, &result.JSONLDData),
		
		// Extract Microdata
		chromedp.Evaluate(`
			(function() {
				const microdataElements = document.querySelectorAll('[itemscope]');
				const microdataData = [];
				
				microdataElements.forEach(element => {
					const item = {
						itemtype: element.getAttribute('itemtype'),
						properties: {}
					};
					
					const properties = element.querySelectorAll('[itemprop]');
					properties.forEach(prop => {
						const name = prop.getAttribute('itemprop');
						const value = prop.getAttribute('content') || prop.textContent.trim();
						item.properties[name] = value;
					});
					
					microdataData.push(item);
				});
				
				return microdataData;
			})()
		`, &result.MicrodataData),
		
		// Extract Open Graph data
		chromedp.Evaluate(`
			(function() {
				const ogTags = document.querySelectorAll('meta[property^="og:"]');
				const ogData = {};
				
				ogTags.forEach(tag => {
					const property = tag.getAttribute('property');
					const content = tag.getAttribute('content');
					ogData[property] = content;
				});
				
				return ogData;
			})()
		`, &result.OpenGraphData),
		
		// Extract Twitter Card data
		chromedp.Evaluate(`
			(function() {
				const twitterTags = document.querySelectorAll('meta[name^="twitter:"]');
				const twitterData = {};
				
				twitterTags.forEach(tag => {
					const name = tag.getAttribute('name');
					const content = tag.getAttribute('content');
					twitterData[name] = content;
				});
				
				return twitterData;
			})()
		`, &result.TwitterCards),
		
		// Extract general meta data
		chromedp.Evaluate(`
			(function() {
				const metaTags = document.querySelectorAll('meta[name], meta[property]');
				const metaData = {};
				
				metaTags.forEach(tag => {
					const name = tag.getAttribute('name') || tag.getAttribute('property');
					const content = tag.getAttribute('content');
					if (name && content) {
						metaData[name] = content;
					}
				});
				
				return metaData;
			})()
		`, &result.MetaData),
		
		// Extract Schema.org data from various sources
		chromedp.Evaluate(`
			(function() {
				const schemaData = [];
				
				// Check for schema.org microdata
				const schemaElements = document.querySelectorAll('[itemtype*="schema.org"]');
				schemaElements.forEach(element => {
					const schema = {
						type: element.getAttribute('itemtype'),
						properties: {}
					};
					
					const properties = element.querySelectorAll('[itemprop]');
					properties.forEach(prop => {
						const name = prop.getAttribute('itemprop');
						let value = prop.getAttribute('content') || prop.textContent.trim();
						
						// Handle special cases
						if (prop.tagName === 'TIME' && prop.getAttribute('datetime')) {
							value = prop.getAttribute('datetime');
						} else if (prop.tagName === 'A' && prop.getAttribute('href')) {
							value = prop.getAttribute('href');
						}
						
						schema.properties[name] = value;
					});
					
					schemaData.push(schema);
				});
				
				return schemaData;
			})()
		`, &result.SchemaOrg),
		
		rec.Stop(),
	)

	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	result.Success = true
	return result
}

func displayResults(result StructuredData) {
	fmt.Printf("Structured Data Extraction Results for %s\n", result.URL)
	fmt.Printf("Status: %s\n", map[bool]string{true: "✓ SUCCESS", false: "✗ FAILED"}[result.Success])
	
	if !result.Success {
		fmt.Printf("Error: %s\n", result.ErrorMessage)
		return
	}

	// Display JSON-LD data
	if len(result.JSONLDData) > 0 {
		fmt.Printf("\n=== JSON-LD Data ===\n")
		for i, data := range result.JSONLDData {
			fmt.Printf("JSON-LD %d:\n", i+1)
			if dataType, ok := data["@type"]; ok {
				fmt.Printf("  Type: %v\n", dataType)
			}
			if context, ok := data["@context"]; ok {
				fmt.Printf("  Context: %v\n", context)
			}
			// Display key properties
			for key, value := range data {
				if key != "@type" && key != "@context" {
					fmt.Printf("  %s: %v\n", key, truncateValue(value))
				}
			}
			fmt.Println()
		}
	}

	// Display Microdata
	if len(result.MicrodataData) > 0 {
		fmt.Printf("\n=== Microdata ===\n")
		for i, item := range result.MicrodataData {
			fmt.Printf("Microdata %d:\n", i+1)
			if itemType, ok := item["itemtype"]; ok {
				fmt.Printf("  Type: %v\n", itemType)
			}
			if properties, ok := item["properties"].(map[string]interface{}); ok {
				for key, value := range properties {
					fmt.Printf("  %s: %v\n", key, truncateValue(value))
				}
			}
			fmt.Println()
		}
	}

	// Display Open Graph data
	if len(result.OpenGraphData) > 0 {
		fmt.Printf("\n=== Open Graph Data ===\n")
		for key, value := range result.OpenGraphData {
			fmt.Printf("  %s: %s\n", key, truncateString(value, 100))
		}
	}

	// Display Twitter Cards
	if len(result.TwitterCards) > 0 {
		fmt.Printf("\n=== Twitter Cards ===\n")
		for key, value := range result.TwitterCards {
			fmt.Printf("  %s: %s\n", key, truncateString(value, 100))
		}
	}

	// Display Schema.org data
	if len(result.SchemaOrg) > 0 {
		fmt.Printf("\n=== Schema.org Data ===\n")
		for i, schema := range result.SchemaOrg {
			fmt.Printf("Schema %d:\n", i+1)
			if schemaType, ok := schema["type"]; ok {
				fmt.Printf("  Type: %v\n", schemaType)
			}
			if properties, ok := schema["properties"].(map[string]interface{}); ok {
				for key, value := range properties {
					fmt.Printf("  %s: %v\n", key, truncateValue(value))
				}
			}
			fmt.Println()
		}
	}

	// Display summary
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("JSON-LD entries: %d\n", len(result.JSONLDData))
	fmt.Printf("Microdata entries: %d\n", len(result.MicrodataData))
	fmt.Printf("Open Graph properties: %d\n", len(result.OpenGraphData))
	fmt.Printf("Twitter Card properties: %d\n", len(result.TwitterCards))
	fmt.Printf("Schema.org entries: %d\n", len(result.SchemaOrg))
	fmt.Printf("Total meta tags: %d\n", len(result.MetaData))
}

func truncateValue(value interface{}) string {
	str := fmt.Sprintf("%v", value)
	return truncateString(str, 100)
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}

func saveReport(result StructuredData) {
	// Save as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	filename := fmt.Sprintf("structured-data-%d.json", time.Now().Unix())
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed structured data saved to %s\n", filename)
}