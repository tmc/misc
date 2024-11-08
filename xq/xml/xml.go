package xml

import (
    "encoding/xml"
    "fmt"
    "io"
    "strings"

    "github.com/antchfx/htmlquery"
    "github.com/antchfx/xmlquery"
    "github.com/antchfx/xpath"
    "github.com/fatih/color"
)

type Node struct {
    *xmlquery.Node
}

func Parse(r io.Reader) (*Node, error) {
    doc, err := xmlquery.Parse(r)
    if err != nil {
        return nil, err
    }
    return &Node{doc}, nil
}

func ParseHTML(r io.Reader) (*Node, error) {
    doc, err := htmlquery.Parse(r)
    if err != nil {
        return nil, err
    }
    return &Node{(*xmlquery.Node)(doc)}, nil
}

func StreamParse(r io.Reader) (*Node, error) {
    doc, err := xmlquery.ParseStream(r)
    if err != nil {
        return nil, err
    }
    return &Node{doc}, nil
}

func Format(n interface{}, indent string) string {
    switch node := n.(type) {
    case *Node:
        return formatNode(node.Node, indent)
    case []*Node:
        var result strings.Builder
        for _, n := range node {
            result.WriteString(formatNode(n.Node, indent))
            result.WriteString("\n")
        }
        return result.String()
    default:
        return fmt.Sprintf("Unsupported type: %T", n)
    }
}

func formatNode(n *xmlquery.Node, indent string) string {
    var buf strings.Builder
    enc := xml.NewEncoder(&buf)
    enc.Indent("", indent)
    if err := enc.Encode(n); err != nil {
        return fmt.Sprintf("Error formatting XML: %v", err)
    }
    return buf.String()
}

func XPathQuery(n interface{}, query string) ([]*Node, error) {
    var root *xmlquery.Node
    switch node := n.(type) {
    case *Node:
        root = node.Node
    default:
        return nil, fmt.Errorf("Unsupported type for XPath query: %T", n)
    }

    expr, err := xpath.Compile(query)
    if err != nil {
        return nil, err
    }

    iter := expr.Evaluate(xmlquery.CreateXPathNavigator(root)).(*xpath.NodeIterator)
    var results []*Node
    for iter.MoveNext() {
        results = append(results, &Node{iter.Current().(*xmlquery.Node)})
    }
    return results, nil
}

func Colorize(input string) string {
    tagColor := color.New(color.FgYellow).SprintFunc()
    attrColor := color.New(color.FgGreen).SprintFunc()
    valueColor := color.New(color.FgCyan).SprintFunc()

    lines := strings.Split(input, "\n")
    for i, line := range lines {
        line = strings.ReplaceAll(line, "<", "\u001B[33m<")
        line = strings.ReplaceAll(line, ">", ">\u001B[0m")
        line = strings.ReplaceAll(line, "=", "\u001B[32m=\u001B[0m")
        line = strings.ReplaceAll(line, "\"", "\u001B[36m\"\u001B[0m")
        lines[i] = line
    }
    return strings.Join(lines, "\n")
}

func ToJSON(n interface{}) interface{} {
    switch node := n.(type) {
    case *Node:
        return nodeToJSON(node.Node)
    case []*Node:
        result := make([]interface{}, len(node))
        for i, n := range node {
            result[i] = nodeToJSON(n.Node)
        }
        return result
    default:
        return fmt.Sprintf("Unsupported type: %T", n)
    }
}

func nodeToJSON(n *xmlquery.Node) interface{} {
    result := make(map[string]interface{})

    if n.Type == xmlquery.ElementNode {
        for _, attr := range n.Attr {
            result["@"+attr.Name.Local] = attr.Value
        }
    }

    if n.FirstChild != nil {
        childContent := make(map[string]interface{})
        for child := n.FirstChild; child != nil; child = child.NextSibling {
            if child.Type == xmlquery.TextNode {
                text := strings.TrimSpace(child.Data)
                if text != "" {
                    result["#text"] = text
                }
            } else if child.Type == xmlquery.ElementNode {
                childJSON := nodeToJSON(child)
                if existing, ok := childContent[child.Data]; ok {
                    switch existing := existing.(type) {
                    case []interface{}:
                        childContent[child.Data] = append(existing, childJSON)
                    default:
                        childContent[child.Data] = []interface{}{existing, childJSON}
                    }
                } else {
                    childContent[child.Data] = childJSON
                }
            }
        }
        for k, v := range childContent {
            result[k] = v
        }
    }

    return result
}

