const path = require('path');
const { workspace, ExtensionContext } = require('vscode');
const { LanguageClient, TransportKind } = require('vscode-languageclient/node');

function activate(context) {
  // Path to the bash LSP server script
  const serverPath = path.join(__dirname, '..', 'bash-lsp-server.sh');
  
  // Options for running the server
  const serverOptions = {
    run: {
      command: serverPath,
      transport: TransportKind.stdio
    },
    debug: {
      command: serverPath,
      transport: TransportKind.stdio
    }
  };

  // Options to control the language client
  const clientOptions = {
    // Register the server for plain text documents
    documentSelector: [{ scheme: 'file', language: 'plaintext' }]
  };

  // Create the language client and start it
  const client = new LanguageClient(
    'demoLspClient',
    'Demo LSP Client',
    serverOptions,
    clientOptions
  );

  // Start the client and add it to the disposables
  const disposable = client.start();
  context.subscriptions.push(disposable);
}

function deactivate() {}

module.exports = { activate, deactivate };