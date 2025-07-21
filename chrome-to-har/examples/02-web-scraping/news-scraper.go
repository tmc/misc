// News website scraper example
// This example shows how to scrape news articles from a website
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
	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type Article struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Author      string `json:"author,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run news-scraper.go <news-site-url>")
		fmt.Println("Example: go run news-scraper.go https://news.ycombinator.com")
		os.Exit(1)
	}

	url := os.Args[1]

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create Chrome browser with options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; NewsBot/1.0)"),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Create recorder to capture network traffic
	rec := recorder.New()

	var articles []Article

	// Navigate and scrape
	err := chromedp.Run(chromeCtx,
		rec.Start(),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for dynamic content
		
		// Extract articles based on common news site patterns
		chromedp.Evaluate(`
			(function() {
				const articles = [];
				
				// Try different common selectors for news articles
				const selectors = [
					'article', 
					'.story', 
					'.article', 
					'.post',
					'.news-item',
					'[class*="story"]',
					'[class*="article"]'
				];
				
				let elements = [];
				for (const selector of selectors) {
					elements = document.querySelectorAll(selector);
					if (elements.length > 0) break;
				}
				
				elements.forEach(el => {
					// Extract title
					let title = '';
					const titleSelectors = ['h1', 'h2', 'h3', '.title', '.headline', 'a'];
					for (const sel of titleSelectors) {
						const titleEl = el.querySelector(sel);
						if (titleEl && titleEl.textContent.trim()) {
							title = titleEl.textContent.trim();
							break;
						}
					}
					
					// Extract URL
					let url = '';
					const linkEl = el.querySelector('a');
					if (linkEl) {
						url = linkEl.href;
					}
					
					// Extract description
					let description = '';
					const descSelectors = ['.summary', '.description', '.excerpt', 'p'];
					for (const sel of descSelectors) {
						const descEl = el.querySelector(sel);
						if (descEl && descEl.textContent.trim()) {
							description = descEl.textContent.trim();
							break;
						}
					}
					
					if (title && url) {
						articles.push({
							title: title,
							url: url,
							description: description
						});
					}
				});
				
				return articles;
			})()
		`, &articles),
		
		rec.Stop(),
	)

	if err != nil {
		log.Fatal(err)
	}

	// Output results
	fmt.Printf("Found %d articles from %s\n\n", len(articles), url)
	
	for i, article := range articles {
		fmt.Printf("Article %d:\n", i+1)
		fmt.Printf("Title: %s\n", article.Title)
		fmt.Printf("URL: %s\n", article.URL)
		if article.Description != "" {
			fmt.Printf("Description: %s\n", truncateString(article.Description, 200))
		}
		fmt.Println(strings.Repeat("-", 50))
	}

	// Save as JSON
	jsonData, err := json.MarshalIndent(articles, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("articles.json", jsonData, 0644)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nArticles saved to articles.json\n")

	// Optionally save HAR file for analysis
	harData, err := rec.HAR()
	if err == nil {
		err = os.WriteFile("scraping.har", []byte(harData), 0644)
		if err == nil {
			fmt.Printf("Network traffic saved to scraping.har\n")
		}
	}
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}