// E-commerce product scraper example
// This example shows how to scrape product information from e-commerce sites
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type Product struct {
	Name        string  `json:"name"`
	Price       string  `json:"price"`
	Description string  `json:"description,omitempty"`
	ImageURL    string  `json:"image_url,omitempty"`
	Rating      float64 `json:"rating,omitempty"`
	URL         string  `json:"url"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ecommerce-scraper.go <product-search-url>")
		fmt.Println("Example: go run ecommerce-scraper.go 'https://example-store.com/search?q=laptop'")
		os.Exit(1)
	}

	url := os.Args[1]

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create Chrome browser with e-commerce friendly options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-extensions", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Create recorder
	rec := recorder.New()

	var products []Product

	// Navigate and scrape
	err := chromedp.Run(chromeCtx,
		rec.Start(),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for dynamic content and lazy loading
		
		// Scroll to load more products
		chromedp.Evaluate(`
			window.scrollTo(0, document.body.scrollHeight);
		`, nil),
		chromedp.Sleep(2*time.Second),
		
		// Extract products
		chromedp.Evaluate(`
			(function() {
				const products = [];
				
				// Common e-commerce product selectors
				const selectors = [
					'[data-testid*="product"]',
					'.product',
					'.product-item',
					'[class*="product"]',
					'[class*="item"]',
					'.search-result',
					'[data-cy*="product"]'
				];
				
				let elements = [];
				for (const selector of selectors) {
					elements = document.querySelectorAll(selector);
					if (elements.length > 0) break;
				}
				
				console.log('Found', elements.length, 'product elements');
				
				elements.forEach(el => {
					// Extract product name
					let name = '';
					const nameSelectors = [
						'h1', 'h2', 'h3', 'h4',
						'.product-title',
						'.product-name',
						'[class*="title"]',
						'[class*="name"]',
						'a[title]'
					];
					
					for (const sel of nameSelectors) {
						const nameEl = el.querySelector(sel);
						if (nameEl && nameEl.textContent.trim()) {
							name = nameEl.textContent.trim();
							break;
						}
					}
					
					// Extract price
					let price = '';
					const priceSelectors = [
						'[class*="price"]',
						'[data-testid*="price"]',
						'.cost',
						'.amount',
						'[class*="cost"]'
					];
					
					for (const sel of priceSelectors) {
						const priceEl = el.querySelector(sel);
						if (priceEl && priceEl.textContent.trim()) {
							price = priceEl.textContent.trim();
							break;
						}
					}
					
					// Extract product URL
					let url = '';
					const linkEl = el.querySelector('a[href]');
					if (linkEl) {
						url = linkEl.href;
					}
					
					// Extract image URL
					let imageUrl = '';
					const imgEl = el.querySelector('img');
					if (imgEl) {
						imageUrl = imgEl.src || imgEl.dataset.src;
					}
					
					// Extract rating
					let rating = 0;
					const ratingSelectors = [
						'[class*="rating"]',
						'[class*="stars"]',
						'[data-testid*="rating"]'
					];
					
					for (const sel of ratingSelectors) {
						const ratingEl = el.querySelector(sel);
						if (ratingEl) {
							const ratingText = ratingEl.textContent || ratingEl.getAttribute('aria-label') || '';
							const ratingMatch = ratingText.match(/(\d+\.?\d*)/);
							if (ratingMatch) {
								rating = parseFloat(ratingMatch[1]);
								break;
							}
						}
					}
					
					// Extract description
					let description = '';
					const descSelectors = [
						'.product-description',
						'.description',
						'[class*="description"]',
						'p'
					];
					
					for (const sel of descSelectors) {
						const descEl = el.querySelector(sel);
						if (descEl && descEl.textContent.trim()) {
							description = descEl.textContent.trim();
							break;
						}
					}
					
					if (name && price) {
						products.push({
							name: name,
							price: price,
							url: url,
							image_url: imageUrl,
							rating: rating,
							description: description
						});
					}
				});
				
				return products;
			})()
		`, &products),
		
		rec.Stop(),
	)

	if err != nil {
		log.Fatal(err)
	}

	// Output results
	fmt.Printf("Found %d products from %s\n\n", len(products), url)
	
	for i, product := range products {
		fmt.Printf("Product %d:\n", i+1)
		fmt.Printf("Name: %s\n", product.Name)
		fmt.Printf("Price: %s\n", product.Price)
		if product.Rating > 0 {
			fmt.Printf("Rating: %.1f/5\n", product.Rating)
		}
		if product.Description != "" {
			fmt.Printf("Description: %s\n", truncateString(product.Description, 150))
		}
		if product.URL != "" {
			fmt.Printf("URL: %s\n", product.URL)
		}
		fmt.Println(strings.Repeat("-", 50))
	}

	// Save as JSON
	jsonData, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("products.json", jsonData, 0644)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nProducts saved to products.json\n")

	// Save HAR file for analysis
	harData, err := rec.HAR()
	if err == nil {
		err = os.WriteFile("ecommerce-scraping.har", []byte(harData), 0644)
		if err == nil {
			fmt.Printf("Network traffic saved to ecommerce-scraping.har\n")
		}
	}
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}