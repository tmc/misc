# Web Scraping Examples

This directory contains comprehensive web scraping examples using chrome-to-har and churl.

## Examples

### 1. News Scraper (`news-scraper.go`)
Scrapes news articles from news websites with intelligent content detection.

**Usage:**
```bash
go run news-scraper.go https://news.ycombinator.com
go run news-scraper.go https://reddit.com/r/technology
```

**Features:**
- Detects articles using multiple common selectors
- Extracts title, URL, description, author, and publication date
- Outputs JSON and HAR files
- Handles dynamic content loading

### 2. E-commerce Scraper (`ecommerce-scraper.go`)
Scrapes product information from e-commerce sites.

**Usage:**
```bash
go run ecommerce-scraper.go "https://example-store.com/search?q=laptop"
```

**Features:**
- Extracts product name, price, rating, description, and images
- Handles lazy loading and infinite scroll
- Anti-detection measures
- Saves results as JSON

### 3. Social Media Scraper (`social-media-scraper.go`)
Scrapes social media posts and engagement metrics.

**Usage:**
```bash
go run social-media-scraper.go "https://twitter.com/search?q=golang"
```

**Features:**
- Extracts posts, authors, timestamps, and engagement metrics
- Identifies hashtags and mentions
- Handles infinite scroll
- Respects rate limits

## Shell Script Helpers

### Batch News Scraping
```bash
#!/bin/bash
# batch-news-scrape.sh
declare -a sites=(
    "https://news.ycombinator.com"
    "https://reddit.com/r/technology"
    "https://techcrunch.com"
)

for site in "${sites[@]}"; do
    echo "Scraping $site..."
    go run news-scraper.go "$site"
    sleep 5  # Be respectful
done
```

### Product Price Monitoring
```bash
#!/bin/bash
# price-monitor.sh
PRODUCT_URL="$1"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

go run ecommerce-scraper.go "$PRODUCT_URL"
mv products.json "products_$TIMESTAMP.json"
echo "Price data saved to products_$TIMESTAMP.json"
```

## Best Practices

### 1. Respect Rate Limits
```go
// Add delays between requests
chromedp.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
```

### 2. Handle Dynamic Content
```go
// Wait for content to load
chromedp.WaitVisible("selector", chromedp.ByQuery)
chromedp.Sleep(2*time.Second)

// Handle infinite scroll
chromedp.Evaluate(`
    window.scrollTo(0, document.body.scrollHeight);
`, nil)
```

### 3. Anti-Detection Measures
```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"),
    chromedp.Flag("disable-blink-features", "AutomationControlled"),
    chromedp.Flag("disable-extensions", false),
)
```

### 4. Error Handling
```go
// Implement retry logic
for attempts := 0; attempts < 3; attempts++ {
    err := chromedp.Run(ctx, actions...)
    if err == nil {
        break
    }
    if attempts < 2 {
        time.Sleep(time.Duration(attempts+1) * time.Second)
    }
}
```

### 5. Data Validation
```go
// Validate extracted data
if title := strings.TrimSpace(title); title != "" && len(title) > 3 {
    // Process valid title
}
```

## Legal and Ethical Considerations

1. **Robots.txt**: Always check and respect robots.txt files
2. **Rate Limiting**: Implement delays between requests
3. **Terms of Service**: Review and comply with website terms
4. **Personal Data**: Be careful with personal information
5. **Attribution**: Credit data sources appropriately

## Advanced Techniques

### 1. Handling JavaScript-heavy Sites
```go
// Wait for specific elements
chromedp.WaitVisible("[data-testid='content']", chromedp.ByQuery)

// Execute custom JavaScript
chromedp.Evaluate(`
    // Custom logic to trigger content loading
    document.querySelector('.load-more').click();
`, nil)
```

### 2. Session Management
```go
// Use cookies for authentication
chromedp.ActionFunc(func(ctx context.Context) error {
    return network.SetCookie("session", "token").Do(ctx)
})
```

### 3. Proxy Support
```go
// Use churl with proxy
cmd := exec.Command("churl", "--proxy", "http://proxy:8080", url)
```

## Troubleshooting

1. **Element not found**: Add explicit waits
2. **Dynamic content**: Increase sleep durations
3. **Rate limiting**: Implement exponential backoff
4. **Memory issues**: Process data in batches
5. **Anti-bot measures**: Update user agents and headers

## Performance Tips

1. **Parallel processing**: Run multiple scrapers concurrently
2. **Caching**: Cache static content and selectors
3. **Efficient selectors**: Use specific, fast CSS selectors
4. **Resource filtering**: Block unnecessary resources (images, ads)
5. **Headless mode**: Use headless Chrome for better performance