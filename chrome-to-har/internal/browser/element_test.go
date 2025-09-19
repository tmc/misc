package browser_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// TestElementQuery tests element querying
func TestElementQuery(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Query single element
	element, err := page.QuerySelector("#title")
	if err != nil {
		t.Errorf("Failed to query selector: %v", err)
		return // Don't continue if query failed
	}
	if element == nil {
		t.Error("Element not found")
		return // Don't continue if element is nil
	}

	// Get element text
	text, err := element.GetText()
	if err != nil {
		t.Errorf("Failed to get element text: %v", err)
		return
	}
	if text != "Test Page" {
		t.Errorf("Unexpected element text: got %s, want Test Page", text)
	}

	// Query non-existent element
	nonExistent, err := page.QuerySelector("#does-not-exist")
	if err != nil {
		t.Errorf("Error querying non-existent element: %v", err)
	}
	if nonExistent != nil {
		t.Error("Non-existent element should be nil")
	}
}

// TestElementQueryAll tests querying multiple elements
func TestElementQueryAll(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Navigate to page with multiple elements
	err = page.Navigate("data:text/html,<div class='item'>1</div><div class='item'>2</div><div class='item'>3</div>")
	if err != nil {
		t.Fatal(err)
	}

	// Query all elements
	elements, err := page.QuerySelectorAll(".item")
	if err != nil {
		t.Errorf("Failed to query all selectors: %v", err)
	}

	if len(elements) != 3 {
		t.Errorf("Expected 3 elements, got %d", len(elements))
	}

	// Check each element's text
	for i, element := range elements {
		text, err := element.GetText()
		if err != nil {
			t.Errorf("Failed to get text for element %d: %v", i, err)
		}
		expectedText := string(rune('1' + i))
		if text != expectedText {
			t.Errorf("Element %d has wrong text: got %s, want %s", i, text, expectedText)
		}
	}
}

// TestElementInteractions tests element interaction methods
func TestElementInteractions(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Get button element
	button, err := page.QuerySelector("#clickme")
	if err != nil {
		t.Fatal(err)
	}

	// Click button
	err = button.Click()
	if err != nil {
		t.Errorf("Failed to click element: %v", err)
	}

	// Verify click result
	result, err := page.QuerySelector("#result")
	if err != nil {
		t.Fatal(err)
	}

	// Check visibility (simplified check without full implementation)
	style, err := result.GetAttribute("style")
	if err != nil {
		t.Errorf("Failed to get style attribute: %v", err)
	}
	if strings.Contains(style, "display:none") || strings.Contains(style, "display: none") {
		t.Error("Result should be visible after click")
	}

	// Get input element
	input, err := page.QuerySelector("#textinput")
	if err != nil {
		t.Fatal(err)
	}

	// Clear and type text
	err = input.Clear()
	if err != nil {
		t.Errorf("Failed to clear input: %v", err)
	}

	testText := "Element typing test"
	err = input.Type(testText)
	if err != nil {
		t.Errorf("Failed to type in element: %v", err)
	}

	// Verify typed text
	value, err := input.GetAttribute("value")
	if err != nil {
		t.Errorf("Failed to get value attribute: %v", err)
	}
	if value != testText {
		t.Errorf("Unexpected input value: got %s, want %s", value, testText)
	}
}

// TestElementAttributes tests attribute manipulation
func TestElementAttributes(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	element, err := page.QuerySelector("#title")
	if err != nil {
		t.Fatal(err)
	}

	// Get existing attribute
	id, err := element.GetAttribute("id")
	if err != nil {
		t.Errorf("Failed to get id attribute: %v", err)
	}
	if id != "title" {
		t.Errorf("Unexpected id: got %s, want title", id)
	}

	// Get non-existent attribute
	nonExistent, err := element.GetAttribute("data-test")
	if err != nil {
		t.Errorf("Error getting non-existent attribute: %v", err)
	}
	if nonExistent != "" {
		t.Errorf("Non-existent attribute should be empty, got: %s", nonExistent)
	}

	// Set attribute
	err = element.SetAttribute("data-test", "test-value")
	if err != nil {
		t.Errorf("Failed to set attribute: %v", err)
	}

	// Verify attribute was set
	testValue, err := element.GetAttribute("data-test")
	if err != nil {
		t.Errorf("Failed to get set attribute: %v", err)
	}
	if testValue != "test-value" {
		t.Errorf("Attribute value mismatch: got %s, want test-value", testValue)
	}
}

// TestElementFocus tests element focus functionality
func TestElementFocus(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	input, err := page.QuerySelector("#textinput")
	if err != nil {
		t.Fatal(err)
	}

	// Focus element
	err = input.Focus()
	if err != nil {
		t.Errorf("Failed to focus element: %v", err)
	}

	// Verify element is focused
	var isFocused bool
	err = page.Evaluate(`document.activeElement.id === 'textinput'`, &isFocused)
	if err != nil {
		t.Errorf("Failed to check focus: %v", err)
	}
	if !isFocused {
		t.Error("Element should be focused")
	}
}

// TestElementScreenshot tests element screenshot
func TestElementScreenshot(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	element, err := page.QuerySelector("#title")
	if err != nil {
		t.Fatal(err)
	}

	// Take element screenshot
	screenshot, err := element.Screenshot()
	if err != nil {
		t.Errorf("Failed to take element screenshot: %v", err)
	}

	if len(screenshot) == 0 {
		t.Error("Element screenshot is empty")
	}

	// Verify it's a PNG
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47}
	if len(screenshot) < 4 || string(screenshot[:4]) != string(pngHeader) {
		t.Error("Element screenshot is not a valid PNG")
	}
}

// TestElementWaitForChild tests waiting for child elements
func TestElementWaitForChild(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Page that adds child element dynamically
	err = page.Navigate("data:text/html,<div id='parent'></div><script>setTimeout(() => { document.getElementById('parent').innerHTML = '<span id=\"child\">Child</span>'; }, 500);</script>")
	if err != nil {
		t.Fatal(err)
	}

	// Get parent element
	parent, err := page.QuerySelector("#parent")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for child to appear
	child, err := parent.WaitForSelector("#child", browser.WaitWithTimeout(2*time.Second))
	if err != nil {
		t.Errorf("Failed to wait for child selector: %v", err)
	}

	if child == nil {
		t.Error("Child element not found")
	}

	// Verify child text
	if child != nil {
		text, err := child.GetText()
		if err != nil {
			t.Errorf("Failed to get child text: %v", err)
		}
		if text != "Child" {
			t.Errorf("Unexpected child text: got %s, want Child", text)
		}
	}
}

// TestElementHover tests element hover
func TestElementHover(t *testing.T) {
	t.Parallel()
	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Create page with hover effect
	err = page.Navigate(`data:text/html,
		<style>
			#hoverable { width: 100px; height: 100px; background: red; }
			#hoverable:hover { background: blue; }
		</style>
		<div id='hoverable' onmouseover='this.textContent="Hovered"'>Hover me</div>
	`)
	if err != nil {
		t.Fatal(err)
	}

	element, err := page.QuerySelector("#hoverable")
	if err != nil {
		t.Fatal(err)
	}

	// Get initial text
	initialText, err := element.GetText()
	if err != nil {
		t.Fatal(err)
	}

	// Hover over element
	err = element.Hover()
	if err != nil {
		t.Errorf("Failed to hover over element: %v", err)
	}

	// Small delay for hover effect
	time.Sleep(100 * time.Millisecond)

	// Get text after hover
	hoverText, err := element.GetText()
	if err != nil {
		t.Fatal(err)
	}

	// Text should have changed
	if initialText == hoverText {
		t.Error("Hover did not change element text")
	}
	if hoverText != "Hovered" {
		t.Errorf("Unexpected hover text: got %s, want Hovered", hoverText)
	}
}

// TestElementScrollIntoView tests scrolling element into view
func TestElementScrollIntoView(t *testing.T) {
	t.Parallel()
	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Create page with scrollable content
	err = page.Navigate(`data:text/html,
		<div style="height: 2000px;">
			<div id="top">Top</div>
			<div id="bottom" style="position: absolute; bottom: 0;">Bottom</div>
		</div>
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Get bottom element
	bottom, err := page.QuerySelector("#bottom")
	if err != nil {
		t.Fatal(err)
	}

	// Check if initially visible in viewport
	var initiallyVisible bool
	err = page.Evaluate(`
		(() => {
			const el = document.getElementById('bottom');
			const rect = el.getBoundingClientRect();
			return rect.top >= 0 && rect.bottom <= window.innerHeight;
		})()
	`, &initiallyVisible)
	if err != nil {
		t.Fatal(err)
	}

	if initiallyVisible {
		t.Skip("Element already visible, cannot test scroll")
	}

	// Scroll into view
	err = bottom.ScrollIntoView()
	if err != nil {
		t.Errorf("Failed to scroll element into view: %v", err)
	}

	// Small delay for scroll
	time.Sleep(500 * time.Millisecond)

	// Check if now visible
	var nowVisible bool
	err = page.Evaluate(`
		(() => {
			const el = document.getElementById('bottom');
			const rect = el.getBoundingClientRect();
			return rect.top >= 0 && rect.bottom <= window.innerHeight;
		})()
	`, &nowVisible)
	if err != nil {
		t.Fatal(err)
	}

	if !nowVisible {
		t.Error("Element should be visible after scrolling into view")
	}
}
