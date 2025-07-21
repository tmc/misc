# SPA Testing Examples

This directory contains examples for testing Single Page Applications (SPAs) with chrome-to-har.

## Examples

### 1. React App Tester (`react-app-tester.go`)
Comprehensive testing for React applications with detailed component analysis.

**Usage:**
```bash
go run react-app-tester.go https://my-react-app.com
```

**Features:**
- Detects React version and DevTools
- Analyzes component tree structure
- Checks for React Router and common UI frameworks
- Monitors hydration and performance metrics
- Generates detailed JSON reports

**Output:**
- Console report with pass/fail status
- `spa-test-report.json` with detailed analysis
- Network traffic analysis

### 2. Vue App Tester (`vue-app-tester.go`)
Specialized testing for Vue.js applications with Vuex state management.

**Usage:**
```bash
go run vue-app-tester.go https://my-vue-app.com
```

**Features:**
- Detects Vue version and DevTools
- Extracts Vuex store state
- Analyzes component hierarchy
- Checks for Vue Router and UI frameworks
- Performance and network analysis

**Output:**
- Console report with Vue-specific metrics
- `vue-test-report.json` with component tree and state
- Vuex state snapshots

### 3. Angular App Tester (`angular-app-tester.go`)
Testing framework for Angular applications with services and dependency injection.

**Usage:**
```bash
go run angular-app-tester.go https://my-angular-app.com
```

**Features:**
- Detects Angular version and DevTools
- Identifies Angular components and services
- Checks for Angular Router, Material, and Forms
- Analyzes directive usage
- Performance monitoring

**Output:**
- Console report with Angular-specific analysis
- `angular-test-report.json` with component and service details
- Framework usage statistics

## Shell Script Helpers

### Multi-Framework Testing
```bash
#!/bin/bash
# test-all-spas.sh
BASE_URL="$1"

echo "Testing React features..."
go run react-app-tester.go "$BASE_URL" > react-results.txt

echo "Testing Vue features..."
go run vue-app-tester.go "$BASE_URL" > vue-results.txt

echo "Testing Angular features..."
go run angular-app-tester.go "$BASE_URL" > angular-results.txt

echo "All tests completed. Check *-results.txt files."
```

### Continuous SPA Monitoring
```bash
#!/bin/bash
# spa-monitor.sh
URL="$1"
INTERVAL=300  # 5 minutes

while true; do
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    echo "Testing at $TIMESTAMP"
    
    go run react-app-tester.go "$URL"
    mv spa-test-report.json "spa-report_$TIMESTAMP.json"
    
    sleep $INTERVAL
done
```

## Testing Strategies

### 1. Route Coverage Testing
Test all important routes in your SPA:
```go
testRoutes := []string{
    "/",
    "/dashboard",
    "/profile",
    "/settings",
    "/products",
    "/cart",
    "/checkout",
    "/admin",
}
```

### 2. Component Analysis
Analyze component loading and rendering:
```go
// Check for specific components
chromedp.WaitVisible("app-header", chromedp.ByQuery)
chromedp.WaitVisible("app-sidebar", chromedp.ByQuery)
chromedp.WaitVisible("app-content", chromedp.ByQuery)
```

### 3. State Management Testing
Monitor application state:
```go
// React - Redux state
chromedp.Evaluate(`window.__REDUX_DEVTOOLS_EXTENSION__.store.getState()`, &state)

// Vue - Vuex state
chromedp.Evaluate(`document.querySelector('#app').__vue__.$store.state`, &state)

// Angular - Services (requires custom setup)
chromedp.Evaluate(`window.ng.probe(document.querySelector('app-root')).injector`, &injector)
```

### 4. Performance Monitoring
Track SPA performance metrics:
```go
// Core Web Vitals
chromedp.Evaluate(`
    new Promise((resolve) => {
        new PerformanceObserver((list) => {
            const entries = list.getEntries();
            resolve(entries);
        }).observe({entryTypes: ['navigation', 'paint', 'largest-contentful-paint']});
    })
`, &metrics)
```

## Framework-Specific Tips

### React Applications
- Look for `__REACT_DEVTOOLS_GLOBAL_HOOK__`
- Check for Redux DevTools extension
- Monitor component re-renders with profiler
- Test error boundaries functionality

### Vue Applications
- Check for `__VUE_DEVTOOLS_GLOBAL_HOOK__`
- Extract Vuex state and mutations
- Monitor component lifecycle hooks
- Test Vue Router navigation

### Angular Applications
- Look for `window.ng` and Angular DevTools
- Check for Angular Material components
- Monitor service injection and providers
- Test lazy-loaded modules

## Common SPA Testing Patterns

### 1. Wait for SPA Hydration
```go
chromedp.WaitVisible("body", chromedp.ByQuery)
chromedp.Sleep(2*time.Second)  // Wait for framework initialization
chromedp.WaitVisible("#root, #app, app-root", chromedp.ByQuery)
```

### 2. Handle Client-Side Routing
```go
// Navigate using history API
chromedp.Evaluate(`history.pushState({}, '', '/new-route')`, nil)
chromedp.Evaluate(`window.dispatchEvent(new PopStateEvent('popstate'))`, nil)
```

### 3. Test Dynamic Content Loading
```go
// Wait for AJAX content
chromedp.WaitVisible(".dynamic-content", chromedp.ByQuery)
chromedp.Sleep(1*time.Second)
```

### 4. Monitor Network Requests
```go
// Track SPA API calls
rec := recorder.New()
chromedp.Run(ctx,
    rec.Start(),
    chromedp.Navigate(url),
    chromedp.WaitVisible("body", chromedp.ByQuery),
    chromedp.Sleep(3*time.Second),  // Wait for API calls
    rec.Stop(),
)
```

## Best Practices

1. **Framework Detection**: Always verify the framework is loaded before testing
2. **Timing**: Use appropriate waits for dynamic content
3. **Error Handling**: Test error states and fallbacks
4. **Performance**: Monitor load times and resource usage
5. **State Management**: Test state persistence across navigation
6. **Responsive Design**: Test different viewport sizes
7. **Accessibility**: Check for ARIA attributes and semantic HTML

## Troubleshooting

1. **Framework not detected**: Increase wait times or check for alternative selectors
2. **Component not found**: Verify component names and hierarchy
3. **State extraction fails**: Check for state management library availability
4. **Performance issues**: Monitor resource loading and bundle sizes
5. **Route testing fails**: Verify router configuration and navigation logic

## Advanced Testing

### Custom Test Suites
Create application-specific test suites:
```go
type CustomSPATest struct {
    Name        string
    URL         string
    Validations []ValidationFunc
}

type ValidationFunc func(ctx context.Context) error
```

### Integration with CI/CD
```yaml
# .github/workflows/spa-test.yml
name: SPA Testing
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Run SPA Tests
        run: |
          go run examples/03-spa-testing/react-app-tester.go ${{ secrets.APP_URL }}
          go run examples/03-spa-testing/vue-app-tester.go ${{ secrets.APP_URL }}
```