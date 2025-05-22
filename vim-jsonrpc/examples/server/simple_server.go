package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tmc/misc/vim-jsonrpc/pkg/server"
)

func main() {
	s, err := server.New("stdio", "", "")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	s.RegisterHandler("echo", handleEcho)
	s.RegisterHandler("add", handleAdd)
	s.RegisterHandler("greet", handleGreet)
	s.RegisterHandler("vim.buffer.get_lines", handleGetLines)
	s.RegisterHandler("vim.command", handleCommand)

	fmt.Fprintf(os.Stderr, "JSON-RPC server started on stdio\n")
	if err := s.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleEcho(ctx context.Context, params interface{}) (interface{}, error) {
	return params, nil
}

func handleAdd(ctx context.Context, params interface{}) (interface{}, error) {
	if params == nil {
		return nil, fmt.Errorf("missing parameters")
	}
	
	switch p := params.(type) {
	case []interface{}:
		if len(p) != 2 {
			return nil, fmt.Errorf("expected 2 parameters, got %d", len(p))
		}
		
		a, ok1 := p[0].(float64)
		b, ok2 := p[1].(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("parameters must be numbers")
		}
		
		return a + b, nil
	default:
		return nil, fmt.Errorf("invalid parameter format")
	}
}

func handleGreet(ctx context.Context, params interface{}) (interface{}, error) {
	if params == nil {
		return "Hello, World!", nil
	}
	
	switch p := params.(type) {
	case string:
		return fmt.Sprintf("Hello, %s!", p), nil
	case map[string]interface{}:
		if name, ok := p["name"].(string); ok {
			return fmt.Sprintf("Hello, %s!", name), nil
		}
		return "Hello, World!", nil
	default:
		return "Hello, World!", nil
	}
}

func handleGetLines(ctx context.Context, params interface{}) (interface{}, error) {
	return []string{
		"Line 1: This is a mock buffer",
		"Line 2: Simulating Vim buffer content",
		"Line 3: JSON-RPC communication working!",
	}, nil
}

func handleCommand(ctx context.Context, params interface{}) (interface{}, error) {
	if params == nil {
		return nil, fmt.Errorf("missing command parameter")
	}
	
	switch p := params.(type) {
	case string:
		fmt.Fprintf(os.Stderr, "Executing command: %s\n", p)
		return fmt.Sprintf("Executed: %s", p), nil
	case []interface{}:
		if len(p) > 0 {
			if cmd, ok := p[0].(string); ok {
				fmt.Fprintf(os.Stderr, "Executing command: %s\n", cmd)
				return fmt.Sprintf("Executed: %s", cmd), nil
			}
		}
		return nil, fmt.Errorf("invalid command format")
	default:
		return nil, fmt.Errorf("command must be a string")
	}
}