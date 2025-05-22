package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tmc/misc/vim-jsonrpc/pkg/client"
)

func main() {
	c, err := client.New("tcp", "localhost:8080", "")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	if err := c.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	ctx := context.Background()

	result, err := c.Call(ctx, "echo", "Hello from client!")
	if err != nil {
		log.Printf("Echo call failed: %v", err)
	} else {
		fmt.Printf("Echo result: %v\n", result)
	}

	result, err = c.Call(ctx, "add", []interface{}{5, 3})
	if err != nil {
		log.Printf("Add call failed: %v", err)
	} else {
		fmt.Printf("Add result: %v\n", result)
	}

	result, err = c.Call(ctx, "greet", map[string]interface{}{"name": "JSON-RPC"})
	if err != nil {
		log.Printf("Greet call failed: %v", err)
	} else {
		fmt.Printf("Greet result: %v\n", result)
	}

	c.OnNotification("log", func(params interface{}) {
		fmt.Printf("Received log notification: %v\n", params)
	})

	if err := c.Notify("log", "This is a test notification"); err != nil {
		log.Printf("Failed to send notification: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Client example completed")
}