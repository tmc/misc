package browser

import (
	"fmt"
	"regexp"
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
	// TextPartial selects elements by partial text content
	TextPartial SelectorEngine = "text*"
	// TextRegex selects elements by regex text match
	TextRegex SelectorEngine = "text-regex"
	// Role selects elements by ARIA role
	Role SelectorEngine = "role"
	// TestID selects elements by data-testid attribute
	TestID SelectorEngine = "data-testid"
	// Compound combines multiple selectors
	Compound SelectorEngine = "compound"
)

// Selector represents a selector with its engine
type Selector struct {
	Engine     SelectorEngine
	Value      string
	Modifiers  SelectorModifiers
	Subqueries []Selector // For compound selectors
	LogicalOp  LogicalOperator
}

// SelectorModifiers contains additional selector options
type SelectorModifiers struct {
	CaseInsensitive bool
	Exact           bool
	Normalize       bool // Normalize whitespace in text matching
}

// LogicalOperator defines how compound selectors are combined
type LogicalOperator string

const (
	OpAnd LogicalOperator = "and"
	OpOr  LogicalOperator = "or"
	OpNot LogicalOperator = "not"
)

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
	if strings.HasPrefix(selector, "text*=") {
		return Selector{Engine: TextPartial, Value: strings.TrimPrefix(selector, "text*=")}
	}
	if strings.HasPrefix(selector, "text-regex=") {
		return Selector{Engine: TextRegex, Value: strings.TrimPrefix(selector, "text-regex=")}
	}
	if strings.HasPrefix(selector, "role=") {
		return Selector{Engine: Role, Value: strings.TrimPrefix(selector, "role=")}
	}
	if strings.HasPrefix(selector, "data-testid=") {
		return Selector{Engine: TestID, Value: strings.TrimPrefix(selector, "data-testid=")}
	}

	// Check for compound selector indicators
	if strings.Contains(selector, " >> ") || strings.Contains(selector, " & ") || strings.Contains(selector, " | ") {
		return parseCompoundSelector(selector)
	}

	// Default to CSS selector
	return Selector{Engine: CSS, Value: selector}
}

// parseCompoundSelector parses compound selectors with logical operators
func parseCompoundSelector(selector string) Selector {
	// Handle chained selectors first (>>)
	if strings.Contains(selector, " >> ") {
		parts := strings.Split(selector, " >> ")
		subqueries := make([]Selector, len(parts))
		for i, part := range parts {
			subqueries[i] = ParseSelector(strings.TrimSpace(part))
		}
		return Selector{
			Engine:     Compound,
			Subqueries: subqueries,
			LogicalOp:  OpAnd,
		}
	}

	// Handle AND operator (&)
	if strings.Contains(selector, " & ") {
		parts := strings.Split(selector, " & ")
		subqueries := make([]Selector, len(parts))
		for i, part := range parts {
			subqueries[i] = ParseSelector(strings.TrimSpace(part))
		}
		return Selector{
			Engine:     Compound,
			Subqueries: subqueries,
			LogicalOp:  OpAnd,
		}
	}

	// Handle OR operator (|)
	if strings.Contains(selector, " | ") {
		parts := strings.Split(selector, " | ")
		subqueries := make([]Selector, len(parts))
		for i, part := range parts {
			subqueries[i] = ParseSelector(strings.TrimSpace(part))
		}
		return Selector{
			Engine:     Compound,
			Subqueries: subqueries,
			LogicalOp:  OpOr,
		}
	}

	return Selector{Engine: CSS, Value: selector}
}

// ToChromedpSelector converts to chromedp selector
func (s Selector) ToChromedpSelector() (string, chromedp.QueryOption) {
	switch s.Engine {
	case XPath:
		return s.Value, chromedp.BySearch
	case Text:
		// Convert text selector to XPath with exact match
		if s.Modifiers.Normalize {
			return fmt.Sprintf("//*[normalize-space(text())='%s']", normalizeWhitespace(s.Value)), chromedp.BySearch
		}
		return fmt.Sprintf("//*[text()='%s']", s.Value), chromedp.BySearch
	case TextPartial:
		// Convert partial text selector to XPath
		if s.Modifiers.CaseInsensitive {
			return fmt.Sprintf("//*[contains(translate(text(), 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'), '%s')]",
				strings.ToLower(s.Value)), chromedp.BySearch
		}
		return fmt.Sprintf("//*[contains(text(), '%s')]", s.Value), chromedp.BySearch
	case TextRegex:
		// For regex, we'll need to use JavaScript evaluation
		// This returns an XPath that will be post-processed
		return fmt.Sprintf("regex:%s", s.Value), chromedp.ByJSPath
	case Role:
		// Enhanced role selector with hierarchy support
		return buildRoleSelector(s.Value), chromedp.ByQuery
	case TestID:
		// Convert test-id selector to CSS
		return fmt.Sprintf("[data-testid='%s']", s.Value), chromedp.ByQuery
	case Compound:
		// Handle compound selectors
		return buildCompoundSelector(s), chromedp.ByJSPath
	default:
		return s.Value, chromedp.ByQuery
	}
}

// normalizeWhitespace normalizes whitespace in text
func normalizeWhitespace(text string) string {
	// Replace multiple whitespaces with single space
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(text, " "))
}

// buildRoleSelector builds an enhanced role selector
func buildRoleSelector(role string) string {
	// Support for common ARIA roles with proper attributes
	roleMap := map[string]string{
		"button":        "[role='button'], button",
		"link":          "[role='link'], a[href]",
		"textbox":       "[role='textbox'], input[type='text'], input[type='email'], input[type='password'], textarea",
		"checkbox":      "[role='checkbox'], input[type='checkbox']",
		"radio":         "[role='radio'], input[type='radio']",
		"combobox":      "[role='combobox'], select",
		"heading":       "[role='heading'], h1, h2, h3, h4, h5, h6",
		"img":           "[role='img'], img",
		"list":          "[role='list'], ul, ol",
		"listitem":      "[role='listitem'], li",
		"navigation":    "[role='navigation'], nav",
		"main":          "[role='main'], main",
		"complementary": "[role='complementary'], aside",
		"contentinfo":   "[role='contentinfo'], footer",
		"banner":        "[role='banner'], header",
	}

	if mapped, ok := roleMap[strings.ToLower(role)]; ok {
		return mapped
	}

	// Default to explicit role attribute
	return fmt.Sprintf("[role='%s']", role)
}

// buildCompoundSelector builds a compound selector for JavaScript evaluation
func buildCompoundSelector(s Selector) string {
	// This will be evaluated in JavaScript context
	// Return a special format that the browser package can interpret
	parts := make([]string, len(s.Subqueries))
	for i, sub := range s.Subqueries {
		selector, _ := sub.ToChromedpSelector()
		parts[i] = selector
	}

	switch s.LogicalOp {
	case OpAnd:
		return fmt.Sprintf("compound:and:%s", strings.Join(parts, "|||"))
	case OpOr:
		return fmt.Sprintf("compound:or:%s", strings.Join(parts, "|||"))
	case OpNot:
		return fmt.Sprintf("compound:not:%s", parts[0])
	default:
		return fmt.Sprintf("compound:and:%s", strings.Join(parts, "|||"))
	}
}

// AdvancedSelector provides a fluent API for building complex selectors
type AdvancedSelector struct {
	base      Selector
	filters   []SelectorFilter
	axes      []XPathAxis
	functions []XPathFunction
}

// SelectorFilter represents a filter to apply to elements
type SelectorFilter struct {
	Type  string
	Value string
}

// XPathAxis represents XPath axes for navigation
type XPathAxis struct {
	Axis     string
	NodeTest string
}

// XPathFunction represents XPath functions
type XPathFunction struct {
	Name string
	Args []string
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

// NewAdvancedSelector creates a new advanced selector builder
func NewAdvancedSelector(base string) *AdvancedSelector {
	return &AdvancedSelector{
		base: ParseSelector(base),
	}
}

// WithRole filters by ARIA role
func (as *AdvancedSelector) WithRole(role string) *AdvancedSelector {
	as.filters = append(as.filters, SelectorFilter{
		Type:  "role",
		Value: role,
	})
	return as
}

// WithText filters by exact text
func (as *AdvancedSelector) WithText(text string, exact bool) *AdvancedSelector {
	filterType := "text"
	if !exact {
		filterType = "text-contains"
	}
	as.filters = append(as.filters, SelectorFilter{
		Type:  filterType,
		Value: text,
	})
	return as
}

// WithTextRegex filters by regex pattern
func (as *AdvancedSelector) WithTextRegex(pattern string) *AdvancedSelector {
	as.filters = append(as.filters, SelectorFilter{
		Type:  "text-regex",
		Value: pattern,
	})
	return as
}

// Ancestor adds ancestor axis
func (as *AdvancedSelector) Ancestor(nodeTest string) *AdvancedSelector {
	as.axes = append(as.axes, XPathAxis{
		Axis:     "ancestor",
		NodeTest: nodeTest,
	})
	return as
}

// Descendant adds descendant axis
func (as *AdvancedSelector) Descendant(nodeTest string) *AdvancedSelector {
	as.axes = append(as.axes, XPathAxis{
		Axis:     "descendant",
		NodeTest: nodeTest,
	})
	return as
}

// Following adds following axis
func (as *AdvancedSelector) Following(nodeTest string) *AdvancedSelector {
	as.axes = append(as.axes, XPathAxis{
		Axis:     "following",
		NodeTest: nodeTest,
	})
	return as
}

// Preceding adds preceding axis
func (as *AdvancedSelector) Preceding(nodeTest string) *AdvancedSelector {
	as.axes = append(as.axes, XPathAxis{
		Axis:     "preceding",
		NodeTest: nodeTest,
	})
	return as
}

// Contains adds XPath contains function
func (as *AdvancedSelector) Contains(haystack, needle string) *AdvancedSelector {
	as.functions = append(as.functions, XPathFunction{
		Name: "contains",
		Args: []string{haystack, needle},
	})
	return as
}

// StartsWith adds XPath starts-with function
func (as *AdvancedSelector) StartsWith(text, prefix string) *AdvancedSelector {
	as.functions = append(as.functions, XPathFunction{
		Name: "starts-with",
		Args: []string{text, prefix},
	})
	return as
}

// ToSelector builds the final selector
func (as *AdvancedSelector) ToSelector() Selector {
	// Build XPath if we have axes or functions
	if len(as.axes) > 0 || len(as.functions) > 0 {
		return as.buildXPathSelector()
	}

	// Otherwise build a compound selector if we have filters
	if len(as.filters) > 0 {
		return as.buildCompoundSelector()
	}

	return as.base
}

// buildXPathSelector builds an XPath selector from advanced options
func (as *AdvancedSelector) buildXPathSelector() Selector {
	var xpath strings.Builder

	// Start with base selector converted to XPath
	baseXPath := as.convertToXPath(as.base)
	xpath.WriteString(baseXPath)

	// Add axes
	for _, axis := range as.axes {
		xpath.WriteString("/")
		xpath.WriteString(axis.Axis)
		xpath.WriteString("::")
		xpath.WriteString(axis.NodeTest)
	}

	// Add filters as predicates
	if len(as.filters) > 0 || len(as.functions) > 0 {
		xpath.WriteString("[")
		predicates := []string{}

		for _, filter := range as.filters {
			switch filter.Type {
			case "text":
				predicates = append(predicates, fmt.Sprintf("text()='%s'", filter.Value))
			case "text-contains":
				predicates = append(predicates, fmt.Sprintf("contains(text(), '%s')", filter.Value))
			case "role":
				predicates = append(predicates, fmt.Sprintf("@role='%s'", filter.Value))
			}
		}

		for _, fn := range as.functions {
			args := strings.Join(fn.Args, ", ")
			predicates = append(predicates, fmt.Sprintf("%s(%s)", fn.Name, args))
		}

		xpath.WriteString(strings.Join(predicates, " and "))
		xpath.WriteString("]")
	}

	return Selector{
		Engine: XPath,
		Value:  xpath.String(),
	}
}

// buildCompoundSelector builds a compound selector from filters
func (as *AdvancedSelector) buildCompoundSelector() Selector {
	subqueries := []Selector{as.base}

	for _, filter := range as.filters {
		switch filter.Type {
		case "text":
			subqueries = append(subqueries, Selector{
				Engine: Text,
				Value:  filter.Value,
			})
		case "text-contains":
			subqueries = append(subqueries, Selector{
				Engine: TextPartial,
				Value:  filter.Value,
			})
		case "text-regex":
			subqueries = append(subqueries, Selector{
				Engine: TextRegex,
				Value:  filter.Value,
			})
		case "role":
			subqueries = append(subqueries, Selector{
				Engine: Role,
				Value:  filter.Value,
			})
		}
	}

	if len(subqueries) == 1 {
		return subqueries[0]
	}

	return Selector{
		Engine:     Compound,
		Subqueries: subqueries,
		LogicalOp:  OpAnd,
	}
}

// convertToXPath converts a selector to XPath format
func (as *AdvancedSelector) convertToXPath(s Selector) string {
	switch s.Engine {
	case CSS:
		// Simple CSS to XPath conversion
		if strings.HasPrefix(s.Value, "#") {
			return fmt.Sprintf("//*[@id='%s']", strings.TrimPrefix(s.Value, "#"))
		}
		if strings.HasPrefix(s.Value, ".") {
			return fmt.Sprintf("//*[contains(@class, '%s')]", strings.TrimPrefix(s.Value, "."))
		}
		return "//*" // Fallback
	case XPath:
		return s.Value
	default:
		return "//*"
	}
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

// GetByRole locates elements by ARIA role
func (p *Page) GetByRole(role string, opts ...LocatorOption) *Locator {
	return p.Locator(fmt.Sprintf("role=%s", role), opts...)
}

// GetByText locates elements by text content
func (p *Page) GetByText(text string, opts ...LocatorOption) *Locator {
	options := &LocatorOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.Exact {
		return p.Locator(fmt.Sprintf("text=%s", text), opts...)
	}
	return p.Locator(fmt.Sprintf("text*=%s", text), opts...)
}

// GetByTestId locates elements by data-testid attribute
func (p *Page) GetByTestId(testId string, opts ...LocatorOption) *Locator {
	return p.Locator(fmt.Sprintf("data-testid=%s", testId), opts...)
}

// GetByLabel locates form elements by associated label text
func (p *Page) GetByLabel(labelText string, opts ...LocatorOption) *Locator {
	// This creates a compound selector that finds inputs associated with labels
	return p.Locator(fmt.Sprintf("xpath=//label[contains(text(), '%s')]//input | //input[@id=(//label[contains(text(), '%s')]/@for)]", labelText, labelText), opts...)
}

// GetByPlaceholder locates input elements by placeholder text
func (p *Page) GetByPlaceholder(placeholder string, opts ...LocatorOption) *Locator {
	return p.Locator(fmt.Sprintf("[placeholder='%s']", placeholder), opts...)
}

// GetByAltText locates images by alt text
func (p *Page) GetByAltText(altText string, opts ...LocatorOption) *Locator {
	return p.Locator(fmt.Sprintf("img[alt='%s']", altText), opts...)
}

// GetByTitle locates elements by title attribute
func (p *Page) GetByTitle(title string, opts ...LocatorOption) *Locator {
	return p.Locator(fmt.Sprintf("[title='%s']", title), opts...)
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

// WithExact enables exact text matching
func WithExact() LocatorOption {
	return func(o *LocatorOptions) {
		o.Exact = true
	}
}

// WithLocatorTimeout sets custom timeout for locator
func WithLocatorTimeout(timeout int) LocatorOption {
	return func(o *LocatorOptions) {
		o.Timeout = timeout
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

// Filter filters the locator by additional criteria
func (l *Locator) Filter(opts ...LocatorOption) *Locator {
	newLoc := &Locator{
		page:     l.page,
		selector: l.selector,
		options:  l.options,
	}

	for _, opt := range opts {
		opt(&newLoc.options)
	}

	return newLoc
}

// And creates a compound locator with AND logic
func (l *Locator) And(selector string) *Locator {
	newSelector := ParseSelector(selector)
	return &Locator{
		page: l.page,
		selector: Selector{
			Engine:     Compound,
			Subqueries: []Selector{l.selector, newSelector},
			LogicalOp:  OpAnd,
		},
		options: l.options,
	}
}

// Or creates a compound locator with OR logic
func (l *Locator) Or(selector string) *Locator {
	newSelector := ParseSelector(selector)
	return &Locator{
		page: l.page,
		selector: Selector{
			Engine:     Compound,
			Subqueries: []Selector{l.selector, newSelector},
			LogicalOp:  OpOr,
		},
		options: l.options,
	}
}

// HasText filters by text content
func (l *Locator) HasText(text string) *Locator {
	newLoc := &Locator{
		page:     l.page,
		selector: l.selector,
		options:  l.options,
	}
	newLoc.options.HasText = text
	return newLoc
}

// HasClass filters by CSS class
func (l *Locator) HasClass(className string) *Locator {
	return l.And(fmt.Sprintf(".%s", className))
}

// HasAttribute filters by attribute presence or value
func (l *Locator) HasAttribute(name string, value ...string) *Locator {
	if len(value) > 0 {
		return l.And(fmt.Sprintf("[%s='%s']", name, value[0]))
	}
	return l.And(fmt.Sprintf("[%s]", name))
}

// Parent gets the parent element
func (l *Locator) Parent() *Locator {
	return &Locator{
		page: l.page,
		selector: Selector{
			Engine: XPath,
			Value:  l.buildXPathWithAxis("parent"),
		},
		options: l.options,
	}
}

// buildXPathWithAxis builds XPath with axis navigation
func (l *Locator) buildXPathWithAxis(axis string) string {
	baseSelector, _ := l.selector.ToChromedpSelector()
	if l.selector.Engine == XPath {
		return fmt.Sprintf("(%s)/%s::*", baseSelector, axis)
	}
	// Convert CSS to simple XPath first
	return fmt.Sprintf("(//*[contains(@class, '%s') or @id='%s'])/%s::*", baseSelector, baseSelector, axis)
}
