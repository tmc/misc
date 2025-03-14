/*
Package gemini provides functionality for interacting with the Google Gemini API.

It includes types and functions for constructing and handling requests
and responses specific to the Gemini API endpoints, focusing on
chat and language model interactions.

This package aims to provide a clean and Go-idiomatic interface
for interacting with the Gemini API within the ant-proxy.

Example usage:

	import "github.com/tmc/misc/ant-proxy/gemini"

	func main() {
		cfg := gemini.Config{
			APIKey: "AIza...",
			Model:  "gemini-pro",
		}
		client := gemini.NewClient(cfg)

		req := gemini.ChatRequest{
			Messages: []gemini.ChatMessage{
				{
					Role:    "user",
					Content: "Hello, Gemini!",
				},
			},
			MaxOutputTokens: 1024,
		}

		resp, err := client.SendMessage(context.Background(), req)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(resp.Content)
	}
*/
package gemini
