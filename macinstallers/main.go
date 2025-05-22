package main

import (
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Define your custom Date type and SoftwareUpdateCatalog structs here
// (Include the structs provided in the previous response)

// Date is a custom type to handle the plist date format.
type Date time.Time

// UnmarshalXML implements xml.Unmarshaler for the Date type.
func (d *Date) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var v string
	err := decoder.DecodeElement(&v, &start)
	if err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

// String implements the Stringer interface for Date
func (d Date) String() string {
	return time.Time(d).Format(time.RFC3339)
}

// SoftwareUpdateCatalog represents the root of the SU catalog XML.
type SoftwareUpdateCatalog struct {
	XMLName xml.Name     `xml:"plist"`
	Dict    TopLevelDict `xml:"dict"`
}

type TopLevelDict struct {
	Items []DictItem `xml:"*"` // Key, CatalogVersion, ApplePostURL, IndexDate, Products. Each is a DictItem
}

// DictItem represents a key-value pair within a Dict.
type DictItem struct {
	Key     string    `xml:"key,omitempty"` // IMPORTANT: omitempty to avoid empty key tags when not needed
	Integer *int64    `xml:"integer,omitempty"`
	String  *string   `xml:"string,omitempty"`
	Date    *Date     `xml:"date,omitempty"`
	Dict    *Products `xml:"dict,omitempty"` // Embedded `Products` struct here
}

// Products now just contains the list of ProductWrapper
type Products struct {
	ProductList []ProductWrapper `xml:",any"` // Handle all product entries
}

// ProductWrapper captures the product ID as the key and the ProductInfo.
type ProductWrapper struct {
	XMLName xml.Name     `xml:""` // Important: Capture the product ID element name
	Key     string       `xml:"key"`
	Dict    *ProductInfo `xml:"dict,omitempty"` // Use pointer to ProductInfo
}

type ProductInfo struct {
	Items []ProductItem `xml:"*"` // ServerMetadataURL, Packages, PostDate, Distributions, DeferredSUEnablementDate, State, ExtendedMetaInfo are ProductItem.
}

type ProductItem struct {
	Key              string                   `xml:"key,omitempty"` // The key of the value.
	String           *string                  `xml:"string,omitempty"`
	Date             *Date                    `xml:"date,omitempty"`
	Dict             *Distributions           `xml:"dict,omitempty"`             // Distributions
	Array            *Packages                `xml:"array,omitempty"`            // Packages
	ExtendedMetaInfo *ExtendedMetaInfoWrapper `xml:"ExtendedMetaInfo,omitempty"` // Wrap ExtendedMetaInfo
}

type ExtendedMetaInfoWrapper struct {
	Dict *ExtendedMetaInfo `xml:"dict"`
}

// Packages represents an array of Package structs.
type Packages struct {
	PackageList []Package `xml:"dict"` // Direct children of array are dict elements
}

type Package struct {
	Items []PackageItem `xml:"*"`
}

type PackageItem struct {
	MetadataURL       *StringWrapper  `xml:"MetadataURL,omitempty"`
	URL               *StringWrapper  `xml:"URL,omitempty"`
	IntegrityDataURL  *StringWrapper  `xml:"IntegrityDataURL,omitempty"`
	IntegrityDataSize *IntegerWrapper `xml:"IntegrityDataSize,omitempty"`
	Digest            *StringWrapper  `xml:"Digest,omitempty"`
	Size              *IntegerWrapper `xml:"Size,omitempty"`
}

type StringWrapper struct {
	String *string `xml:"string"`
}

type IntegerWrapper struct {
	Integer *int64 `xml:"integer"`
}

type ExtendedMetaInfo struct {
	Items []ExtendedMetaInfoItem `xml:"*"` //InstallAssistantPackageIdentifiers
}

type ExtendedMetaInfoItem struct {
	Key  string                              `xml:"key,omitempty"`
	Dict *InstallAssistantPackageIdentifiers `xml:"dict,omitempty"`
}

type InstallAssistantPackageIdentifiers struct {
	Items []InstallAssistantPackageIdentifiersItem `xml:"*"`
}

type InstallAssistantPackageIdentifiersItem struct {
	Key    string  `xml:"key,omitempty"`
	String *string `xml:"string,omitempty"`
}

// Distributions dictionary
type Distributions struct {
	Items []DistributionItem `xml:"*"`
}

// DistributionItem handles a distribution in a specific language
type DistributionItem struct {
	Key    string  `xml:"key,omitempty"`    // e.g., "English"
	String *string `xml:"string,omitempty"` // The URL
}

// --- Existing code ---
var (
	cacheDuration = 24 * time.Hour
	cacheFile     = "catalog.xml.cache"
	format        = flag.String("format", "{{.Version}}: {{.InstallAssistantPkgURL}}", "Template format for output")
	verbose       = flag.Bool("verbose", false, "Verbose output")
	jsonFunc      = func(v interface{}) string {
		a, _ := json.Marshal(v)
		return string(a)
	}
	downloadURLRegex = regexp.MustCompile(`InstallAssistant\.pkg$`)

	//go:embed fetch-url.txt
	fetchURL string
)

// Output is the structure passed to the template
type Output struct {
	Version                string
	InstallAssistantPkgURL string
	AllURLs                []string
	ProductInfo            *ProductInfo
}

func main() {
	flag.Parse()

	// Read the contents of fetch-url.txt from the embedded files.
	fetchURL := strings.TrimSpace(fetchURL)

	// Check if the cache file exists and is recent
	cacheFilePath := filepath.Join(".", cacheFile)
	cacheInfo, err := os.Stat(cacheFilePath)
	useCache := err == nil && time.Since(cacheInfo.ModTime()) < cacheDuration

	var body []byte

	if useCache {
		fmt.Println("Using cached data...")
		body, err = os.ReadFile(cacheFilePath)
		if err != nil {
			log.Printf("Error reading cache file: %v, fetching from URL...", err)
			useCache = false // Fallback to fetching from URL
		}
	}

	if !useCache {
		// Fetch the URL.
		fmt.Println("Fetching data from URL...")
		resp, err := http.Get(fetchURL)
		if err != nil {
			log.Fatalf("error fetching %s: %v", fetchURL, err)
		}
		defer resp.Body.Close()

		// Check the response status code.
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("error fetching %s: status code %d", fetchURL, resp.StatusCode)
		}

		// Determine if the response is gzipped.
		var reader io.Reader = resp.Body
		if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
			gzipReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				log.Fatalf("error creating gzip reader: %v", err)
			}
			defer gzipReader.Close()
			reader = gzipReader
		}

		// Read the response body.
		body, err = io.ReadAll(reader)
		if err != nil {
			log.Fatalf("error reading response body: %v", err)
		}

		// Cache the result to a file.
		err = os.WriteFile(cacheFilePath, body, 0644)
		if err != nil {
			log.Printf("error writing to cache file %s: %v", cacheFilePath, err)
		} else {
			fmt.Printf("Cached response to %s\n", cacheFilePath)
		}
	}

	// Unmarshal the XML.
	var swCatalog SoftwareUpdateCatalog // Renamed to swCatalog
	err = xml.Unmarshal(body, &swCatalog)
	if err != nil {
		log.Fatalf("error unmarshaling XML: %v", err)
	}

	if *verbose {
		fmt.Printf("%+v\n", swCatalog) // Use swCatalog
	}

	// Create the template
	tmpl := template.New("output").Funcs(template.FuncMap{
		"json": jsonFunc,
	})

	tmpl, err = tmpl.Parse(*format)
	if err != nil {
		log.Fatalf("error parsing template: %v", err)
	}

	// Find the "Products" item in the TopLevelDict
	var products *Products
	for _, item := range swCatalog.Dict.Items {
		if item.Key == "Products" && item.Dict != nil {
			products = item.Dict
			break
		}
	}

	if products == nil {
		log.Fatal("No Products found in the catalog")
		return
	}
	fmt.Printf("Catalog contains %d products\n", len(products.ProductList))
	// Iterate through the products
	for _, productWrapper := range products.ProductList {
		//fmt.Printf("wrap: %+v", productWrapper)
		if productWrapper.Dict != nil {
			output := Output{
				AllURLs:     []string{},
				ProductInfo: productWrapper.Dict,
			}

			// Extract Version
			var sharedSupport string
			for _, productItem := range productWrapper.Dict.Items {
				if productItem.Key == "ExtendedMetaInfo" && productItem.ExtendedMetaInfo != nil {
					for _, extendedMetaInfoItem := range productItem.ExtendedMetaInfo.Dict.Items {
						if extendedMetaInfoItem.Key == "InstallAssistantPackageIdentifiers" && extendedMetaInfoItem.Dict != nil {
							for _, idItem := range extendedMetaInfoItem.Dict.Items {
								if idItem.Key == "SharedSupport" && idItem.String != nil {
									sharedSupport = *idItem.String
									break
								}
							}
						}
					}
					break // Added break to stop looping after finding ExtendedMetaInfo
				}
			}

			if sharedSupport != "" {
				output.Version = strings.ReplaceAll(sharedSupport, "com.apple.pkg.InstallAssistant.macOS", "")
			}

			// Extract URLs
			for _, productItem := range productWrapper.Dict.Items {
				if productItem.Key == "Packages" && productItem.Array != nil {
					for _, packageStruct := range productItem.Array.PackageList {
						for _, packageItem := range packageStruct.Items {
							if packageItem.URL != nil && packageItem.URL.String != nil {
								output.AllURLs = append(output.AllURLs, *packageItem.URL.String)
								if downloadURLRegex.MatchString(*packageItem.URL.String) {
									output.InstallAssistantPkgURL = *packageItem.URL.String
								}
							}
						}
					}
					break // Added break to stop looping after finding Packages
				}
			}

			if output.InstallAssistantPkgURL != "" {
				// Execute the template
				err = tmpl.Execute(os.Stdout, output)
				if err != nil {
					log.Printf("error executing template: %v", err)
				}
				fmt.Println() // Add a newline after each output
			}
		}
	}
}
