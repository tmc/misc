package termmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"golang.org/x/term"
)

// TermRenderer renders markdown for terminal output
type TermRenderer struct {
	width     int
	indent    int
	listLevel int
}

// NewTermRenderer creates a new terminal renderer
func NewTermRenderer() *TermRenderer {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}
	return &TermRenderer{
		width:  width,
		indent: 0,
	}
}

// RenderMarkdown converts markdown to terminal-friendly text
func RenderMarkdown(md string) (string, error) {
	tr := NewTermRenderer()
	markdown := goldmark.New(
		goldmark.WithRenderer(
			renderer.NewRenderer(
				renderer.WithNodeRenderers(
					util.Prioritized(tr, 1000),
				),
			),
		),
	)

	var buf bytes.Buffer
	if err := markdown.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RegisterFuncs implements renderer.NodeRenderer
func (r *TermRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDocument, r.renderDocument)
	reg.Register(ast.KindHeading, r.renderHeading)
	reg.Register(ast.KindParagraph, r.renderParagraph)
	reg.Register(ast.KindTextBlock, r.renderTextBlock)
	reg.Register(ast.KindText, r.renderText)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
	reg.Register(ast.KindList, r.renderList)
	reg.Register(ast.KindListItem, r.renderListItem)
	reg.Register(ast.KindEmphasis, r.renderEmphasis)
	reg.Register(ast.KindLink, r.renderLink)
}

func (r *TermRenderer) renderDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		heading := node.(*ast.Heading)
		fmt.Fprintf(w, "\n%s ", strings.Repeat("#", heading.Level))
	} else {
		fmt.Fprintf(w, "\n\n")
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if r.indent > 0 {
			fmt.Fprintf(w, "%s", strings.Repeat(" ", r.indent))
		}
	} else {
		fmt.Fprintf(w, "\n\n")
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderTextBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		fmt.Fprintf(w, "\n")
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		text := node.(*ast.Text)
		content := string(text.Segment.Value(source))
		if text.SoftLineBreak() {
			content = strings.TrimRight(content, " \t\n")
			content += " "
		}
		fmt.Fprintf(w, "%s", content)
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		fmt.Fprintf(w, "\n")
		r.writeIndentedCode(w, source, node)
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		fmt.Fprintf(w, "\n")
		fenced := node.(*ast.FencedCodeBlock)
		if lang := string(fenced.Language(source)); lang != "" {
			fmt.Fprintf(w, "$ # %s\n", lang)
		}
		r.writeIndentedCode(w, source, node)
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) writeIndentedCode(w io.Writer, source []byte, node ast.Node) {
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		fmt.Fprintf(w, "    %s", string(line.Value(source)))
	}
	fmt.Fprintf(w, "\n")
}

func (r *TermRenderer) renderList(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.listLevel++
		r.indent += 2
	} else {
		r.listLevel--
		r.indent -= 2
		fmt.Fprintf(w, "\n")
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderListItem(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		fmt.Fprintf(w, "%s• ", strings.Repeat(" ", r.indent-2))
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderEmphasis(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		fmt.Fprintf(w, "_")
	} else {
		fmt.Fprintf(w, "_")
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		link := node.(*ast.Link)
		fmt.Fprintf(w, "[")
		r.renderChildren(w, source, node)
		fmt.Fprintf(w, "](%s)", string(link.Destination))
		return ast.WalkSkipChildren, nil
	}
	return ast.WalkContinue, nil
}

func (r *TermRenderer) renderChildren(w util.BufWriter, source []byte, node ast.Node) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		// Get the renderer function for this node type
		if renderer, ok := r.getRenderer(child.Kind()); ok {
			renderer(w, source, child, true)
			if child.HasChildren() {
				r.renderChildren(w, source, child)
			}
			renderer(w, source, child, false)
		} else {
			// Fallback for unknown node types
			if text := child.Text(source); len(text) > 0 {
				w.Write(text)
			}
		}
	}
}

func (r *TermRenderer) getRenderer(kind ast.NodeKind) (renderer.NodeRendererFunc, bool) {
	switch kind {
	case ast.KindText:
		return r.renderText, true
	case ast.KindEmphasis:
		return r.renderEmphasis, true
	case ast.KindLink:
		return r.renderLink, true
	case ast.KindParagraph:
		return r.renderParagraph, true
	case ast.KindHeading:
		return r.renderHeading, true
	case ast.KindList:
		return r.renderList, true
	case ast.KindListItem:
		return r.renderListItem, true
	case ast.KindCodeBlock:
		return r.renderCodeBlock, true
	case ast.KindFencedCodeBlock:
		return r.renderFencedCodeBlock, true
	default:
		return nil, false
	}
}
