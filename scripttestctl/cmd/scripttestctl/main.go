package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

type ScripttestController struct {
	engine *script.Engine
	env    []string
	cgpt   *CGPTIntegration
}

func NewScripttestController() *ScripttestController {
	return &ScripttestController{
		engine: &script.Engine{
			Conds: scripttest.DefaultConds(),
			Cmds:  scripttest.DefaultCmds(),
			Quiet: false,
		},
		env:  os.Environ(),
		cgpt: NewCGPTIntegration(),
	}
}

func (sc *ScripttestController) RunTests(pattern string) error {
	ctx := context.Background()
	scripttest.Test(nil, ctx, sc.engine, sc.env, pattern)
	return nil
}

func (sc *ScripttestController) GenerateTest(description, outputFile string) error {
	testContent, err := sc.cgpt.GenerateTest(description)
	if err != nil {
		return fmt.Errorf("failed to generate test: %v", err)
	}

	err = os.WriteFile(outputFile, []byte(testContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write test file: %v", err)
	}

	fmt.Printf("Test generated and saved to %s\n", outputFile)
	return nil
}

func main() {
	sc := NewScripttestController()

	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: scripttestctl <command> [args...]")
		os.Exit(1)
	}

	switch args[0] {
	case "run":
		if len(args) < 2 {
			fmt.Println("Usage: scripttestctl run <test_pattern>")
			os.Exit(1)
		}
		err := sc.RunTests(args[1])
		if err != nil {
			fmt.Printf("Error running tests: %v\n", err)
			os.Exit(1)
		}
	case "generate":
		if len(args) < 3 {
			fmt.Println("Usage: scripttestctl generate <description> <output_file>")
			os.Exit(1)
		}
		err := sc.GenerateTest(args[1], args[2])
		if err != nil {
			fmt.Printf("Error generating test: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", args[0])
		os.Exit(1)
	}
}
