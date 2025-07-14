package browser

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// ElementHandle represents a handle to a DOM element
type ElementHandle struct {
	ctx      context.Context
	nodeID   cdp.NodeID
	objectID runtime.RemoteObjectID
	page     *Page
}

// QuerySelector finds the first element matching the selector
func (p *Page) QuerySelector(selector string) (*ElementHandle, error) {
	var nodes []*cdp.Node
	if err := chromedp.Run(p.ctx,
		chromedp.Nodes(selector, &nodes, chromedp.ByQuery),
	); err != nil {
		return nil, errors.Wrapf(err, "querying selector %s", selector)
	}

	if len(nodes) == 0 {
		return nil, nil // No element found
	}

	node := nodes[0]
	
	// Get the remote object for the node
	objID, err := dom.ResolveNode().WithNodeID(node.NodeID).Do(p.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "resolving node")
	}

	return &ElementHandle{
		ctx:      p.ctx,
		nodeID:   node.NodeID,
		objectID: objID.ObjectID,
		page:     p,
	}, nil
}

// QuerySelectorAll finds all elements matching the selector
func (p *Page) QuerySelectorAll(selector string) ([]*ElementHandle, error) {
	var nodes []*cdp.Node
	if err := chromedp.Run(p.ctx,
		chromedp.Nodes(selector, &nodes, chromedp.ByQueryAll),
	); err != nil {
		return nil, errors.Wrapf(err, "querying selector %s", selector)
	}

	elements := make([]*ElementHandle, 0, len(nodes))
	for _, node := range nodes {
		objID, err := dom.ResolveNode().WithNodeID(node.NodeID).Do(p.ctx)
		if err != nil {
			continue
		}

		elements = append(elements, &ElementHandle{
			ctx:      p.ctx,
			nodeID:   node.NodeID,
			objectID: objID.ObjectID,
			page:     p,
		})
	}

	return elements, nil
}

// Click clicks the element
func (e *ElementHandle) Click(opts ...ClickOption) error {
	options := &ClickOptions{
		Button: "left",
		Count:  1,
	}

	for _, opt := range opts {
		opt(options)
	}

	return chromedp.Run(e.ctx,
		chromedp.MouseClickNode(e.nodeID),
	)
}

// Type types text into the element
func (e *ElementHandle) Type(text string, opts ...TypeOption) error {
	options := &TypeOptions{}

	for _, opt := range opts {
		opt(options)
	}

	// Focus the element first
	if err := e.Focus(); err != nil {
		return err
	}

	// Clear and type
	return chromedp.Run(e.ctx,
		chromedp.SendKeys(e.nodeID, text, chromedp.ByNodeID),
	)
}

// Clear clears the element's value
func (e *ElementHandle) Clear() error {
	return chromedp.Run(e.ctx,
		chromedp.Clear(e.nodeID, chromedp.ByNodeID),
	)
}

// Focus focuses the element
func (e *ElementHandle) Focus() error {
	return chromedp.Run(e.ctx,
		chromedp.Focus(e.nodeID, chromedp.ByNodeID),
	)
}

// GetText gets the text content
func (e *ElementHandle) GetText() (string, error) {
	var text string
	if err := chromedp.Run(e.ctx,
		chromedp.Text(e.nodeID, &text, chromedp.ByNodeID),
	); err != nil {
		return "", errors.Wrap(err, "getting text")
	}
	return text, nil
}

// GetAttribute gets an attribute value
func (e *ElementHandle) GetAttribute(name string) (string, error) {
	var value string
	var exists bool
	if err := chromedp.Run(e.ctx,
		chromedp.AttributeValue(e.nodeID, name, &value, &exists, chromedp.ByNodeID),
	); err != nil {
		return "", errors.Wrap(err, "getting attribute")
	}
	
	if !exists {
		return "", nil
	}
	
	return value, nil
}

// SetAttribute sets an attribute value
func (e *ElementHandle) SetAttribute(name, value string) error {
	return chromedp.Run(e.ctx,
		chromedp.SetAttributeValue(e.nodeID, name, value, chromedp.ByNodeID),
	)
}

// GetProperty gets a JavaScript property value
func (e *ElementHandle) GetProperty(property string) (interface{}, error) {
	var result interface{}
	err := chromedp.Run(e.ctx,
		chromedp.Evaluate(
			fmt.Sprintf(
				`(() => { const el = document.querySelector('[data-nodeid="%d"]'); return el ? el.%s : null; })()`,
				e.nodeID, property,
			),
			&result,
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "getting property")
	}
	return result, nil
}

// IsVisible checks if the element is visible
func (e *ElementHandle) IsVisible() (bool, error) {
	var visible bool
	err := runtime.CallFunctionOn(`
		function() {
			const style = window.getComputedStyle(this);
			return style.display !== 'none' && 
			       style.visibility !== 'hidden' && 
			       style.opacity !== '0';
		}
	`).WithObjectID(e.objectID).Do(e.ctx)
	
	if err != nil {
		return false, errors.Wrap(err, "checking visibility")
	}
	
	return visible, nil
}

// ScrollIntoView scrolls the element into view
func (e *ElementHandle) ScrollIntoView() error {
	return runtime.CallFunctionOn(`
		function() { this.scrollIntoView({behavior: 'smooth', block: 'center'}); }
	`).WithObjectID(e.objectID).Do(e.ctx)
}

// Hover hovers over the element
func (e *ElementHandle) Hover() error {
	// Get element position
	box, err := e.GetBoundingBox()
	if err != nil {
		return err
	}

	// Move mouse to center of element
	centerX := box.X + box.Width/2
	centerY := box.Y + box.Height/2

	return chromedp.Run(e.ctx,
		chromedp.MouseEvent("mousemove", centerX, centerY),
	)
}

// GetBoundingBox gets the element's bounding box
func (e *ElementHandle) GetBoundingBox() (*BoundingBox, error) {
	var box BoundingBox
	
	err := runtime.CallFunctionOn(`
		function() {
			const rect = this.getBoundingClientRect();
			return {
				x: rect.x,
				y: rect.y,
				width: rect.width,
				height: rect.height
			};
		}
	`).WithObjectID(e.objectID).Do(e.ctx)
	
	if err != nil {
		return nil, errors.Wrap(err, "getting bounding box")
	}
	
	return &box, nil
}

// BoundingBox represents element dimensions
type BoundingBox struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// Screenshot takes a screenshot of the element
func (e *ElementHandle) Screenshot(opts ...ScreenshotOption) ([]byte, error) {
	options := &ScreenshotOptions{
		Type:    "png",
		Quality: 90,
	}

	for _, opt := range opts {
		opt(options)
	}

	var buf []byte
	if err := chromedp.Run(e.ctx,
		chromedp.Screenshot(e.nodeID, &buf, chromedp.ByNodeID),
	); err != nil {
		return nil, errors.Wrap(err, "taking element screenshot")
	}

	return buf, nil
}

// WaitForSelector waits for a child selector within this element
func (e *ElementHandle) WaitForSelector(selector string, opts ...WaitOption) (*ElementHandle, error) {
	options := &WaitOptions{
		State:   "visible",
		Timeout: 30000,
	}

	for _, opt := range opts {
		opt(options)
	}

	// Build a selector that's relative to this element
	// This is a simplified version - real implementation would need proper selector handling
	fullSelector := fmt.Sprintf(`[data-nodeid="%d"] %s`, e.nodeID, selector)
	
	if err := e.page.WaitForSelector(fullSelector, opts...); err != nil {
		return nil, err
	}

	return e.page.QuerySelector(fullSelector)
}

// Evaluate evaluates JavaScript in the context of this element
func (e *ElementHandle) Evaluate(expression string, result interface{}) error {
	return runtime.CallFunctionOn(
		fmt.Sprintf("function() { return (%s); }", expression),
	).WithObjectID(e.objectID).Do(e.ctx)
}