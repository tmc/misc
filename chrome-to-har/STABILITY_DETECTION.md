# Enhanced Stability Detection

The chrome-to-har project now includes a comprehensive stability detection system that can reliably determine when dynamic web pages have finished loading all their content.

## Features

### Network Idle Detection
- **Configurable threshold**: Set the maximum number of concurrent network requests that constitute "idle"
- **Configurable timeout**: Set how long to wait at the idle threshold before considering the page stable
- **Monitoring window**: Define the time window for monitoring network activity

### DOM Stability Detection
- **Mutation monitoring**: Tracks DOM modifications in real-time using MutationObserver
- **Stability threshold**: Configure the maximum number of mutations allowed for stability
- **Timeout configuration**: Set how long to wait for DOM stability

### Resource Loading Completion
- **Images**: Wait for all images to load completely
- **Fonts**: Wait for all web fonts to be ready
- **Stylesheets**: Wait for all CSS resources to be processed
- **Scripts**: Wait for all JavaScript resources to load

### JavaScript Execution Completion
- **Animation frames**: Wait for the next animation frame to complete
- **Idle callbacks**: Wait for browser idle time using requestIdleCallback
- **Custom execution checks**: Define custom JavaScript conditions for stability

### Configurable Timeouts and Retries
- **Maximum stability wait**: Set overall timeout for stability detection
- **Retry attempts**: Configure number of retry attempts on failure
- **Retry delays**: Set delays between retry attempts

### Custom Stability Checks
- **JavaScript expressions**: Define custom JavaScript conditions that must be true
- **Per-check timeouts**: Set individual timeouts for each custom check
- **Named checks**: Give descriptive names to custom stability checks

## Command Line Usage

### Basic Stability Detection
```bash
# Enable enhanced stability detection
chrome-to-har -wait-for-stability -url https://example.com

# Use legacy stability detection (simple timeout)
chrome-to-har -wait-stable -url https://example.com
```

### Network Idle Configuration
```bash
# Wait for complete network idle (0 requests)
chrome-to-har -wait-for-stability -network-idle-timeout 500 -url https://example.com

# Allow up to 2 concurrent requests before considering idle
chrome-to-har -wait-for-stability -network-idle-timeout 1000 -url https://example.com
```

### DOM Stability Configuration
```bash
# Wait 1 second for DOM stability
chrome-to-har -wait-for-stability -dom-stable-timeout 1000 -url https://example.com
```

### Resource Loading Configuration
```bash
# Wait for all resource types
chrome-to-har -wait-for-stability -wait-for-images -wait-for-fonts -wait-for-stylesheets -wait-for-scripts -url https://example.com

# Only wait for images and stylesheets
chrome-to-har -wait-for-stability -wait-for-images -wait-for-stylesheets -url https://example.com
```

### Timeout and Retry Configuration
```bash
# Set overall timeout to 60 seconds with 5 retry attempts
chrome-to-har -wait-for-stability -stable-timeout 60 -stability-retries 5 -url https://example.com

# Set resource loading timeout to 15 seconds
chrome-to-har -wait-for-stability -resource-timeout 15 -url https://example.com
```

## Programmatic Usage

### Basic API Usage
```go
// Create a page
page, err := browser.NewPage()
if err != nil {
    log.Fatal(err)
}

// Configure stability detection
page.ConfigureStability(
    browser.WithNetworkIdleThreshold(0),
    browser.WithNetworkIdleTimeout(500*time.Millisecond),
    browser.WithDOMStableTimeout(1*time.Second),
    browser.WithResourceWaiting(true, true, true, true),
    browser.WithMaxStabilityWait(30*time.Second),
    browser.WithVerboseLogging(true),
)

// Navigate and wait for stability
if err := page.Navigate("https://example.com"); err != nil {
    log.Fatal(err)
}

ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

if err := page.WaitForStability(ctx, nil); err != nil {
    log.Printf("Stability detection failed: %v", err)
}
```

### Custom Stability Checks
```go
// Add custom stability checks
page.ConfigureStability(
    browser.WithCustomCheck("spa-ready", "window.appReady === true", 10*time.Second),
    browser.WithCustomCheck("data-loaded", "document.querySelectorAll('.data-item').length > 0", 5*time.Second),
    browser.WithVerboseLogging(true),
)
```

### Different Load States
```go
// Wait for different load states
ctx := context.Background()

// DOM content loaded
err := page.WaitForLoadState(ctx, browser.LoadStateDOMContentLoaded)

// Network idle (0 requests)
err = page.WaitForLoadState(ctx, browser.LoadStateNetworkIdle0)

// Network idle (max 2 requests)
err = page.WaitForLoadState(ctx, browser.LoadStateNetworkIdle2)
```

### Monitoring Stability Metrics
```go
// Get stability metrics
metrics := page.GetStabilityMetrics()
if metrics != nil {
    log.Printf("Network requests: %d", metrics.NetworkRequests)
    log.Printf("DOM modifications: %d", metrics.DOMModifications)
    log.Printf("Pending requests: %d", len(metrics.PendingRequests))
    log.Printf("Loaded resources: %d", len(metrics.LoadedResources))
}
```

## Advanced Configuration

### Single Page Application (SPA) Detection
```go
// Configure for SPA stability detection
page.ConfigureStability(
    browser.WithNetworkIdleThreshold(2),                  // Allow some background requests
    browser.WithNetworkIdleTimeout(1*time.Second),       // Wait longer for network idle
    browser.WithDOMStableTimeout(2*time.Second),         // Wait longer for DOM stability
    browser.WithResourceWaiting(true, true, true, true), // Wait for all resources
    browser.WithMaxStabilityWait(60*time.Second),        // Allow more time for SPAs
    browser.WithCustomCheck("spa-ready", "window.appReady === true", 15*time.Second),
    browser.WithVerboseLogging(true),
)
```

### Complex Dynamic Pages
```go
// Configure for complex dynamic pages
page.ConfigureStability(
    browser.WithNetworkIdleThreshold(1),           // Allow minimal background activity
    browser.WithNetworkIdleTimeout(2*time.Second), // Wait longer for network idle
    browser.WithDOMStableTimeout(3*time.Second),   // Wait longer for DOM stability
    browser.WithResourceWaiting(true, true, true, true),
    browser.WithMaxStabilityWait(90*time.Second),  // Allow more time
    browser.WithCustomCheck("content-loaded", "document.querySelectorAll('.content').length > 0", 10*time.Second),
    browser.WithCustomCheck("ads-loaded", "document.querySelectorAll('.ad').length > 0", 5*time.Second),
    browser.WithVerboseLogging(true),
)
```

## Default Configuration

The default stability configuration includes:
- **Network idle threshold**: 0 (no network requests)
- **Network idle timeout**: 500ms
- **DOM stable timeout**: 500ms
- **Resource loading timeout**: 10 seconds
- **Max stability wait**: 30 seconds
- **Retry attempts**: 3
- **Retry delay**: 1 second
- **Wait for all resource types**: true
- **Wait for animation frames**: true
- **Wait for idle callbacks**: true

## Best Practices

1. **Start with defaults**: The default configuration works well for most websites
2. **Enable verbose logging**: Use `-verbose` flag during development to understand timing
3. **Adjust timeouts gradually**: Start with conservative timeouts and adjust based on your needs
4. **Use custom checks for SPAs**: Single page applications often need custom stability checks
5. **Monitor metrics**: Use stability metrics to understand page loading behavior
6. **Consider retry logic**: Enable retries for flaky network conditions
7. **Test with different page types**: Static pages, SPAs, and dynamic content may need different configurations

## Troubleshooting

### Common Issues

1. **Stability detection timeout**: Increase `MaxStabilityWait` or adjust individual timeouts
2. **Network never idle**: Increase `NetworkIdleThreshold` to allow some background activity
3. **DOM never stable**: Increase `DOMStableTimeout` for pages with continuous animations
4. **Resource loading timeout**: Increase `ResourceTimeout` for pages with large assets
5. **Custom checks failing**: Review JavaScript expressions and adjust timeouts

### Debug Information

Enable verbose logging to see detailed information about:
- Network request tracking
- DOM mutation monitoring
- Resource loading progress
- Custom check execution
- Timeout and retry behavior

```bash
chrome-to-har -wait-for-stability -verbose -url https://example.com
```

This enhanced stability detection system provides robust and configurable page loading detection suitable for modern web applications with dynamic content loading patterns.