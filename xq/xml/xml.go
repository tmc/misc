package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Node struct {
	Type        string
	Data        string
	Attr        []xml.Attr
	FirstChild  *Node
	NextSibling *Node
}

func Parse(r io.Reader) (*Node, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return parseXML(data)
}

func ParseHTML(r io.Reader) (*Node, error) {
	start := time.Now()
	defer func() {
		log.Printf("ParseHTML took %v", time.Since(start))
	}()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	node := convertHTMLNode(doc)
	log.Printf("Parsed HTML structure: %+v", node)

	// Find the body content
	body := findBody(node)
	if body != nil {
		return body, nil
	}

	// If no body is found, return the original node
	return node, nil
}

func findBody(n *Node) *Node {
	if n.Type == "Element" && strings.ToLower(n.Data) == "body" {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if result := findBody(child); result != nil {
			return result
		}
	}
	return nil
}

func convertHTMLNode(n *html.Node) *Node {
	node := &Node{
		Type: nodeTypeToString(n.Type),
		Data: n.Data,
	}

	for _, attr := range n.Attr {
		node.Attr = append(node.Attr, xml.Attr{Name: xml.Name{Local: attr.Key}, Value: attr.Val})
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		child := convertHTMLNode(c)
		if child.Type != "Document" && child.Type != "Element" && strings.TrimSpace(child.Data) == "" {
			continue // Skip empty text nodes
		}
		if node.FirstChild == nil {
			node.FirstChild = child
		} else {
			lastChild := node.FirstChild
			for lastChild.NextSibling != nil {
				lastChild = lastChild.NextSibling
			}
			lastChild.NextSibling = child
		}
	}

	return node
}

func nodeTypeToString(t html.NodeType) string {
	switch t {
	case html.ErrorNode:
		return "Error"
	case html.TextNode:
		return "Text"
	case html.DocumentNode:
		return "Document"
	case html.ElementNode:
		return "Element"
	case html.CommentNode:
		return "Comment"
	case html.DoctypeNode:
		return "Doctype"
	default:
		return "Unknown"
	}
}

func parseXML(data []byte) (*Node, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var stack []*Node
	var root *Node

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			node := &Node{Type: "Element", Data: t.Name.Local, Attr: t.Attr}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if parent.FirstChild == nil {
					parent.FirstChild = node
				} else {
					sibling := parent.FirstChild
					for sibling.NextSibling != nil {
						sibling = sibling.NextSibling
					}
					sibling.NextSibling = node
				}
			} else {
				root = node
			}
			stack = append(stack, node)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			node := &Node{Type: "Text", Data: string(t)}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				if parent.FirstChild == nil {
					parent.FirstChild = node
				} else {
					sibling := parent.FirstChild
					for sibling.NextSibling != nil {
						sibling = sibling.NextSibling
					}
					sibling.NextSibling = node
				}
			}
		}
	}

	return root, nil
}

func Format(n interface{}, indent string) string {
	switch node := n.(type) {
	case *Node:
		return strings.TrimSpace(formatNodeContent(node, indent, 0))
	default:
		return fmt.Sprintf("Unsupported type: %T", n)
	}
}

func formatNodeContent(n *Node, indent string, depth int) string {
	if n == nil {
		return ""
	}

	var buf strings.Builder

	// Skip the "body" tag itself
	if n.Type == "Element" && strings.ToLower(n.Data) == "body" {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			buf.WriteString(formatNode(child, indent, depth))
		}
	} else {
		buf.WriteString(formatNode(n, indent, depth))
	}

	return buf.String()
}

func formatNode(n *Node, indent string, depth int) string {
	if n == nil {
		return ""
	}

	var buf strings.Builder
	padding := strings.Repeat(indent, depth)

	switch n.Type {
	case "Element":
		buf.WriteString(fmt.Sprintf("%s<%s", padding, n.Data))
		for _, attr := range n.Attr {
			buf.WriteString(fmt.Sprintf(` %s="%s"`, attr.Name.Local, attr.Value))
		}
		if n.FirstChild == nil {
			buf.WriteString("/>")
		} else {
			buf.WriteString(">\n")
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				buf.WriteString(formatNode(child, indent, depth+1))
			}
			buf.WriteString(fmt.Sprintf("%s</%s>", padding, n.Data))
		}
	case "Text":
		text := strings.TrimSpace(n.Data)
		if text != "" {
			buf.WriteString(fmt.Sprintf("%s%s", padding, text))
		}
	case "Document":
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			buf.WriteString(formatNode(child, indent, depth))
		}
		return buf.String() // Don't add an extra newline for the document node
	}
	buf.WriteString("\n")

	return buf.String()
}

func XPathQuery(n interface{}, query string) ([]*Node, error) {
	// Implement XPath query logic here
	return nil, fmt.Errorf("XPath query not implemented")
}

func Colorize(input string) string {
	// Implement colorization logic here
	return input
}

func ToJSON(n interface{}) interface{} {
	// Implement JSON conversion logic here
	return n
}
