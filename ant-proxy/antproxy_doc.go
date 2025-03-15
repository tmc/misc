/*
Package main is the entry point for the ant-proxy application.

It sets up and runs the proxy server, configuring it to listen for
incoming Anthropic API requests and forward them to other providers
like Google Gemini, OpenAI, or Ollama, based on the proxy's configuration.

The main package is responsible for:

  - Loading configuration from environment variables or configuration files.
  - Initializing backend provider clients (Anthropic, Gemini, etc.).
  - Creating and configuring the proxy instance.
  - Setting up HTTP handlers to receive Anthropic API requests.
  - Starting the HTTP server to listen for incoming connections.

Example usage (running the proxy):

go run . -anthropic-api-key sk-ant-... -gemini-api-key AIza... -listen-address :8080

This will start the ant-proxy server, listening on port 8080, configured
with Anthropic and Gemini API keys. Incoming requests to the proxy will be
routed and processed according to the proxy's logic.
*/
package main
