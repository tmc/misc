package main

import (
	"bytes"
	"fmt"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig"
)

// TemplateCacheEntry represents a cached template
type TemplateCacheEntry struct {
	Template *template.Template
	LastUsed int64 // Unix timestamp
}

// TemplateEngineLite is a streamlined template engine for WebAssembly
type TemplateEngineLite struct {
	// Template cache to avoid reparsing
	cache     map[string]*TemplateCacheEntry
	cacheLock sync.RWMutex

	// Maximum number of templates to keep in cache
	maxCacheSize int

	// Custom template functions
	customFuncs template.FuncMap
}

// NewTemplateEngineLite creates a new lightweight template engine
func NewTemplateEngineLite() *TemplateEngineLite {
	engine := &TemplateEngineLite{
		cache:        make(map[string]*TemplateCacheEntry),
		maxCacheSize: 50, // Reasonable default for WebAssembly
	}

	// Register custom functions
	engine.customFuncs = template.FuncMap{
		// Add custom functions here
		"methodExtension": func(method interface{}, path string) interface{} {
			// Simplified implementation
			return nil
		},
		"messageExtension": func(message interface{}, path string) interface{} {
			// Simplified implementation
			return nil
		},
		"fieldExtension": func(field interface{}, path string) interface{} {
			// Simplified implementation
			return nil
		},
		"fieldByName": func(message interface{}, name string) interface{} {
			// Simplified implementation
			return nil
		},
	}

	return engine
}

// Execute executes a template with the given data
func (e *TemplateEngineLite) Execute(templateName, templateContent string, data interface{}) (string, error) {
	// Check cache first (read lock)
	e.cacheLock.RLock()
	entry, found := e.cache[templateContent]
	e.cacheLock.RUnlock()

	var tmpl *template.Template
	var err error

	if found {
		tmpl = entry.Template
		// Update last used time with write lock
		e.cacheLock.Lock()
		entry.LastUsed = currentTimeUnix()
		e.cacheLock.Unlock()
	} else {
		// Parse template (outside of lock)
		tmpl, err = e.parseTemplate(templateName, templateContent)
		if err != nil {
			return "", fmt.Errorf("template parse error: %w", err)
		}

		// Update cache with write lock
		e.cacheLock.Lock()
		// Check cache size and evict if necessary
		if len(e.cache) >= e.maxCacheSize {
			e.evictOldestTemplate()
		}
		e.cache[templateContent] = &TemplateCacheEntry{
			Template: tmpl,
			LastUsed: currentTimeUnix(),
		}
		e.cacheLock.Unlock()
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// parseTemplate parses a template with standard and custom functions
func (e *TemplateEngineLite) parseTemplate(name, content string) (*template.Template, error) {
	// Create template with both sprig and custom functions
	tmpl := template.New(name).
		Funcs(sprig.TxtFuncMap()).
		Funcs(e.customFuncs)

	// Parse the template
	parsed, err := tmpl.Parse(content)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}

// evictOldestTemplate removes the least recently used template from cache
func (e *TemplateEngineLite) evictOldestTemplate() {
	var oldestKey string
	var oldestTime int64 = 1<<63 - 1 // Max int64

	for key, entry := range e.cache {
		if entry.LastUsed < oldestTime {
			oldestTime = entry.LastUsed
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(e.cache, oldestKey)
	}
}

// currentTimeUnix returns the current Unix timestamp
func currentTimeUnix() int64 {
	// In a real implementation, this would return time.Now().Unix()
	// For WebAssembly compatibility, we can use js.Global().Get("Date").Call("now").Int()
	// For this example, we'll just return a dummy value
	return 0
}

// ClearCache clears the template cache
func (e *TemplateEngineLite) ClearCache() {
	e.cacheLock.Lock()
	defer e.cacheLock.Unlock()
	e.cache = make(map[string]*TemplateCacheEntry)
}

// SetMaxCacheSize sets the maximum number of templates to keep in cache
func (e *TemplateEngineLite) SetMaxCacheSize(size int) {
	if size < 1 {
		size = 1
	}
	e.cacheLock.Lock()
	defer e.cacheLock.Unlock()
	e.maxCacheSize = size

	// Evict templates if necessary
	for len(e.cache) > e.maxCacheSize {
		e.evictOldestTemplate()
	}
}

// AddCustomFunction adds a custom function to the template engine
func (e *TemplateEngineLite) AddCustomFunction(name string, fn interface{}) {
	e.cacheLock.Lock()
	defer e.cacheLock.Unlock()
	e.customFuncs[name] = fn
	
	// Clear cache since functions have changed
	e.cache = make(map[string]*TemplateCacheEntry)
}