package main

import (
	"fmt"
	"strings"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Store open documents
var documents = make(map[string]string)

func textDocumentDidOpen(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	uri := params.TextDocument.URI
	documents[uri] = params.TextDocument.Text
	
	// Trigger initial diagnostics
	go publishDiagnostics(context, uri, documents[uri])
	
	return nil
}

func textDocumentDidChange(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := params.TextDocument.URI
	
	// Update document content
	if content, exists := documents[uri]; exists {
		for _, change := range params.ContentChanges {
			// For simplicity, we're assuming full document sync
			if change.Text != "" {
				documents[uri] = change.Text
			}
		}
		
		// Trigger diagnostics on change
		go publishDiagnostics(context, uri, documents[uri])
	}
	
	return nil
}

func textDocumentDidClose(context *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	uri := params.TextDocument.URI
	delete(documents, uri)
	
	// Clear diagnostics for closed file
	context.Notify("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: []protocol.Diagnostic{},
	})
	
	return nil
}

func textDocumentCompletion(context *glsp.Context, params *protocol.CompletionParams) (any, error) {
	// Demo completion items
	completionItems := []protocol.CompletionItem{
		{
			Label:  "println",
			Detail: new(string),
			Kind:   protocol.CompletionItemKindFunction,
			Documentation: &protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: "Prints values to stdout",
			},
			InsertText: new(string),
		},
		{
			Label:  "if",
			Detail: new(string),
			Kind:   protocol.CompletionItemKindKeyword,
			Documentation: &protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: "Conditional statement",
			},
			InsertText: new(string),
		},
		{
			Label:  "for",
			Detail: new(string),
			Kind:   protocol.CompletionItemKindKeyword,
			Documentation: &protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: "Loop statement",
			},
			InsertText: new(string),
		},
		{
			Label:  "func",
			Detail: new(string),
			Kind:   protocol.CompletionItemKindKeyword,
			Documentation: &protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: "Function declaration",
			},
			InsertText: new(string),
		},
	}
	
	// Set values for pointers
	*completionItems[0].Detail = "func println(a ...any)"
	*completionItems[0].InsertText = "println($1)"
	*completionItems[1].Detail = "if statement"
	*completionItems[1].InsertText = "if $1 {\n\t$0\n}"
	*completionItems[2].Detail = "for loop"
	*completionItems[2].InsertText = "for $1 {\n\t$0\n}"
	*completionItems[3].Detail = "function declaration"
	*completionItems[3].InsertText = "func ${1:name}($2) $3 {\n\t$0\n}"
	
	return completionItems, nil
}

func textDocumentHover(context *glsp.Context, params *protocol.HoverParams) (any, error) {
	uri := params.TextDocument.URI
	content, exists := documents[uri]
	if !exists {
		return nil, nil
	}
	
	// Get word at position (simplified)
	lines := strings.Split(content, "\n")
	if int(params.Position.Line) >= len(lines) {
		return nil, nil
	}
	
	line := lines[params.Position.Line]
	
	// Demo: provide hover for specific keywords
	if strings.Contains(line, "println") {
		return protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: "```go\nfunc println(a ...any)\n```\n\nPrints values to stdout",
			},
		}, nil
	}
	
	return nil, nil
}

func textDocumentDiagnostic(context *glsp.Context, params *protocol.DocumentDiagnosticParams) (any, error) {
	uri := params.TextDocument.URI
	content, exists := documents[uri]
	if !exists {
		return protocol.DocumentDiagnosticReport{
			Kind: "full",
			Items: protocol.RelatedFullDocumentDiagnosticReport{
				FullDocumentDiagnosticReport: protocol.FullDocumentDiagnosticReport{
					Items: []protocol.Diagnostic{},
				},
			},
		}, nil
	}
	
	diagnostics := generateDiagnostics(content)
	
	return protocol.DocumentDiagnosticReport{
		Kind: "full",
		Items: protocol.RelatedFullDocumentDiagnosticReport{
			FullDocumentDiagnosticReport: protocol.FullDocumentDiagnosticReport{
				Items: diagnostics,
			},
		},
	}, nil
}

func textDocumentDefinition(context *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	// Demo: return empty array (no definitions found)
	return []protocol.Location{}, nil
}

func textDocumentReferences(context *glsp.Context, params *protocol.ReferenceParams) (any, error) {
	// Demo: return empty array (no references found)
	return []protocol.Location{}, nil
}

func textDocumentCodeAction(context *glsp.Context, params *protocol.CodeActionParams) (any, error) {
	// Demo: return empty array (no code actions)
	return []protocol.CodeAction{}, nil
}

func textDocumentFormatting(context *glsp.Context, params *protocol.DocumentFormattingParams) (any, error) {
	// Demo: return empty array (no formatting changes)
	return []protocol.TextEdit{}, nil
}

func textDocumentSignatureHelp(context *glsp.Context, params *protocol.SignatureHelpParams) (any, error) {
	// Demo signature help
	signatures := []protocol.SignatureInformation{
		{
			Label: "println(a ...any)",
			Documentation: &protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: "Prints values to stdout",
			},
			Parameters: []protocol.ParameterInformation{
				{
					Label: "a ...any",
					Documentation: &protocol.MarkupContent{
						Kind:  protocol.MarkupKindPlainText,
						Value: "Values to print",
					},
				},
			},
		},
	}
	
	activeParameter := uint32(0)
	
	return protocol.SignatureHelp{
		Signatures:      signatures,
		ActiveSignature: 0,
		ActiveParameter: &activeParameter,
	}, nil
}

// Helper functions

func generateDiagnostics(content string) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic
	
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Demo: Check for common issues
		
		// Check for TODO comments
		if strings.Contains(line, "TODO") || strings.Contains(line, "FIXME") {
			start := strings.Index(line, "TODO")
			if start == -1 {
				start = strings.Index(line, "FIXME")
			}
			
			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(i), Character: uint32(start)},
					End:   protocol.Position{Line: uint32(i), Character: uint32(len(line))},
				},
				Severity: protocol.DiagnosticSeverityInformation,
				Message:  "TODO comment found",
				Source:   "go-lsp-server",
			})
		}
		
		// Check for missing package declaration
		if i == 0 && !strings.HasPrefix(strings.TrimSpace(line), "package") {
			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: 0, Character: 0},
				},
				Severity: protocol.DiagnosticSeverityError,
				Message:  "Missing package declaration",
				Source:   "go-lsp-server",
			})
		}
	}
	
	return diagnostics
}

func publishDiagnostics(context *glsp.Context, uri string, content string) {
	diagnostics := generateDiagnostics(content)
	
	context.Notify("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}