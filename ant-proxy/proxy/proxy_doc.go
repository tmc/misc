/*
Package proxy implements the core proxy logic for ant-proxy.

It handles routing incoming requests, transforming them as needed,
dispatching them to different backend providers (like Anthropic or Gemini),
and then handling and transforming the responses back to the client.

The proxy package is designed to be extensible, allowing for the addition
of new backend providers and transformation logic.

Key components include:

  - Request routing: Determines which backend provider to use based on configuration or request analysis.
  - Request transformation: Adapts incoming requests to the format expected by the backend provider.
  - Provider dispatch: Sends the transformed request to the selected backend provider.
  - Response handling: Receives responses from the backend provider.
  - Response transformation: Adapts backend responses back to the format expected by the original client (if necessary).

Example usage (conceptual - requires further implementation):

	import (
		"github.com/tmc/misc/ant-proxy/anthropic"
		"github.com/tmc/misc/ant-proxy/gemini"
		"github.com/tmc/misc/ant-proxy/proxy"
	)

	func main() {
		anthropicClient := anthropic.NewClient(anthropic.Config{APIKey: "..."})
		geminiClient := gemini.NewClient(gemini.Config{APIKey: "..."})

		p := proxy.NewProxy(
			proxy.WithProvider("anthropic", anthropicClient),
			proxy.WithProvider("gemini", geminiClient),
			// ... other options ...
		)

		// ... handle incoming request and use proxy to process it ...
		// response, err := p.HandleRequest(ctx, incomingRequest)
		// ...
	}
*/
package proxy
