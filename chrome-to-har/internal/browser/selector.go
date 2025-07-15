package browser

import (
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"
)

// SelectorEngine provides different selector strategies similar to Playwright
type SelectorEngine string

const (
	// CSS is the default CSS selector engine
	CSS SelectorEngine = "css"
	// XPath selects elements using XPath
	XPath SelectorEngine = "xpath"
	// Text selects elements by text content
	Text SelectorEngine = "text"
	// Role selects elements by ARIA role
	Role SelectorEngine = "role"
	// TestID selects elements by data-testid attribute
	TestID SelectorEngine = "data-testid"
)

// Selector represents a selector with its engine
type Selector struct {
	Engine SelectorEngine
	Value  string
}

// ParseSelector parses a selector string with optional engine prefix
func ParseSelector(selector string) Selector {
	// Check for engine prefix
	if strings.HasPrefix(selector, "css=") {
		return Selector{Engine: CSS, Value: strings.TrimPrefix(selector, "css=")}
	}
	if strings.HasPrefix(selector, "xpath=") {
		return Selector{Engine: XPath, Value: strings.TrimPrefix(selector, "xpath=")}
	}
	if strings.HasPrefix(selector, "text=") {
		return Selector{Engine: Text, Value: strings.TrimPrefix(selector, "text=")}
	}
	if strings.HasPrefix(selector, "role=") {
		return Selector{Engine: Role, Value: strings.TrimPrefix(selector, "role=")}
	}
	if strings.HasPrefix(selector, "data-testid=") {
		return Selector{Engine: TestID, Value: strings.TrimPrefix(selector, "data-testid=")}
	}

	// Default to CSS selector
	return Selector{Engine: CSS, Value: selector}
}

// ToChromedpSelector converts to chromedp selector
func (s Selector) ToChromedpSelector() (string, chromedp.QueryOption) {
	switch s.Engine {
	case XPath:
		return s.Value, chromedp.BySearch
	case Text:
		// Convert text selector to XPath
		return fmt.Sprintf("//*[contains(text(), '%s')]", s.Value), chromedp.BySearch
	case Role:
		// Convert role selector to CSS
		return fmt.Sprintf("[role='%s']", s.Value), chromedp.ByQuery
	case TestID:
		// Convert test-id selector to CSS
		return fmt.Sprintf("[data-testid='%s']", s.Value), chromedp.ByQuery
	default:
		return s.Value, chromedp.ByQuery
	}
}

// BuildSelector builds a complex selector from parts
type SelectorBuilder struct {
	parts []string
}

// NewSelectorBuilder creates a new selector builder
func NewSelectorBuilder() *SelectorBuilder {
	return &SelectorBuilder{}
}

// Add adds a selector part
func (sb *SelectorBuilder) Add(selector string) *SelectorBuilder {
	sb.parts = append(sb.parts, selector)
	return sb
}

// HasText adds a text filter
func (sb *SelectorBuilder) HasText(text string) *SelectorBuilder {
	sb.parts = append(sb.parts, fmt.Sprintf(":has-text('%s')", text))
	return sb
}

// Nth selects the nth matching element
func (sb *SelectorBuilder) Nth(index int) *SelectorBuilder {
	sb.parts = append(sb.parts, fmt.Sprintf(":nth(%d)", index))
	return sb
}

// First selects the first matching element
func (sb *SelectorBuilder) First() *SelectorBuilder {
	return sb.Nth(0)
}

// Last selects the last matching element
func (sb *SelectorBuilder) Last() *SelectorBuilder {
	sb.parts = append(sb.parts, ":last")
	return sb
}

// Visible selects only visible elements
func (sb *SelectorBuilder) Visible() *SelectorBuilder {
	sb.parts = append(sb.parts, ":visible")
	return sb
}

// Build returns the final selector
func (sb *SelectorBuilder) Build() string {
	return strings.Join(sb.parts, " ")
}

// LocatorOptions provides options for element location
type LocatorOptions struct {
	HasText     string
	HasSelector string
	Index       int
	Exact       bool
	Timeout     int
}

// Locator provides a way to find elements with retry logic
type Locator struct {
	page     *Page
	selector Selector
	options  LocatorOptions
}

// Locator creates a new element locator
func (p *Page) Locator(selector string, opts ...LocatorOption) *Locator {
	loc := &Locator{
		page:     p,
		selector: ParseSelector(selector),
		options:  LocatorOptions{},
	}

	for _, opt := range opts {
		opt(&loc.options)
	}

	return loc
}

// LocatorOption configures a locator
type LocatorOption func(*LocatorOptions)

// WithText filters by text content
func WithText(text string) LocatorOption {
	return func(o *LocatorOptions) {
		o.HasText = text
	}
}

// WithSelector filters by child selector
func WithSelector(selector string) LocatorOption {
	return func(o *LocatorOptions) {
		o.HasSelector = selector
	}
}

// WithIndex selects specific index
func WithIndex(index int) LocatorOption {
	return func(o *LocatorOptions) {
		o.Index = index
	}
}

// Click clicks the element found by the locator
func (l *Locator) Click(opts ...ClickOption) error {
	el, err := l.Element()
	if err != nil {
		return err
	}
	return el.Click(opts...)
}

// Type types into the element found by the locator
func (l *Locator) Type(text string, opts ...TypeOption) error {
	el, err := l.Element()
	if err != nil {
		return err
	}
	return el.Type(text, opts...)
}

// GetText gets text from the element found by the locator
func (l *Locator) GetText() (string, error) {
	el, err := l.Element()
	if err != nil {
		return "", err
	}
	return el.GetText()
}

// Element finds the element matching the locator
func (l *Locator) Element() (*ElementHandle, error) {
	selector, _ := l.selector.ToChromedpSelector()

	// Apply additional filters if needed
	if l.options.HasText != "" {
		// Modify selector to include text filter
		if l.selector.Engine == CSS {
			selector = fmt.Sprintf("%s:contains('%s')", selector, l.options.HasText)
		}
	}

	// Find all matching elements
	elements, err := l.page.QuerySelectorAll(selector)
	if err != nil {
		return nil, err
	}

	if len(elements) == 0 {
		return nil, fmt.Errorf("no element found matching %s", selector)
	}

	// Apply index filter
	if l.options.Index >= 0 && l.options.Index < len(elements) {
		return elements[l.options.Index], nil
	}

	return elements[0], nil
}

// Elements finds all elements matching the locator
func (l *Locator) Elements() ([]*ElementHandle, error) {
	selector, _ := l.selector.ToChromedpSelector()
	return l.page.QuerySelectorAll(selector)
}

// Count returns the number of matching elements
func (l *Locator) Count() (int, error) {
	elements, err := l.Elements()
	if err != nil {
		return 0, err
	}
	return len(elements), nil
}

// IsVisible checks if the element is visible
func (l *Locator) IsVisible() (bool, error) {
	el, err := l.Element()
	if err != nil {
		return false, nil // Element not found means not visible
	}
	return el.IsVisible()
}

// WaitFor waits for the element to match a condition
func (l *Locator) WaitFor(opts ...WaitOption) error {
	selector, _ := l.selector.ToChromedpSelector()
	return l.page.WaitForSelector(selector, opts...)
}
