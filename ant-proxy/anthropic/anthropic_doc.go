/*
Package anthropic provides functionality for interacting with the Anthropic API.

It includes types and functions for constructing and handling requests
and responses specific to the Anthropic API endpoints, particularly
the /v1/messages endpoint used for chat-like interactions.

This package aims to abstract away the details of the Anthropic API,
providing a Go-idiomatic interface for use within the ant-proxy.

Example usage:

	import "github.com/tmc/misc/ant-proxy/anthropic"

	func main() {
		cfg := anthropic.Config{
			APIKey: "sk-ant-...",
			APIVersion: "2023-06-01",
			Model: "claude-3-sonnet-20250219",
		}
		client := anthropic.NewClient(cfg)

		req := anthropic.MessageRequest{
			Messages: []anthropic.Message{
				{
					Role:    "user",
					Content: "Hello, Claude!",
				},
			},
			MaxTokens: 1024,
		}

		resp, err := client.SendMessage(context.Background(), req)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(resp.Content)
	}
*/
package anthropic
