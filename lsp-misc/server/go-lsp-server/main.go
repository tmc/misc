package main

import (
	"github.com/tliron/commonlog"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"

	// Must include a backend implementation
	_ "github.com/tliron/commonlog/simple"
)

const lsName = "Demo Go Language Server"

var (
	version string = "0.0.1"
	handler protocol.Handler
)

func main() {
	// This increases logging verbosity (optional)
	commonlog.Configure(1, nil)

	handler = protocol.Handler{
		Initialize:               initialize,
		Initialized:              initialized,
		Shutdown:                 shutdown,
		SetTrace:                 setTrace,
		TextDocumentCompletion:   textDocumentCompletion,
		TextDocumentDidOpen:      textDocumentDidOpen,
		TextDocumentDidChange:    textDocumentDidChange,
		TextDocumentDidClose:     textDocumentDidClose,
		TextDocumentDiagnostic:   textDocumentDiagnostic,
		TextDocumentHover:        textDocumentHover,
		TextDocumentDefinition:   textDocumentDefinition,
		TextDocumentReferences:   textDocumentReferences,
		TextDocumentCodeAction:   textDocumentCodeAction,
		TextDocumentFormatting:   textDocumentFormatting,
		TextDocumentSignatureHelp: textDocumentSignatureHelp,
	}

	server := server.NewServer(&handler, lsName, false)
	server.RunStdio()
}

func initialize(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
	capabilities := handler.CreateServerCapabilities()

	// Advertise supported capabilities
	capabilities.CompletionProvider = &protocol.CompletionOptions{
		TriggerCharacters: []string{".", ":"},
	}
	capabilities.HoverProvider = true
	capabilities.DefinitionProvider = true
	capabilities.ReferencesProvider = true
	capabilities.DocumentFormattingProvider = true
	capabilities.CodeActionProvider = true
	capabilities.SignatureHelpProvider = &protocol.SignatureHelpOptions{
		TriggerCharacters: []string{"(", ","},
	}
	capabilities.DiagnosticProvider = &protocol.DiagnosticOptions{
		Identifier:            "go-lsp-server",
		InterFileDependencies: false,
		WorkspaceDiagnostics:  false,
	}

	// Text document sync options
	capabilities.TextDocumentSync = protocol.TextDocumentSyncKindIncremental

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &version,
		},
	}, nil
}

func initialized(context *glsp.Context) error {
	return nil
}

func shutdown(context *glsp.Context) error {
	return nil
}

func setTrace(context *glsp.Context, params *protocol.SetTraceParams) error {
	return nil
}