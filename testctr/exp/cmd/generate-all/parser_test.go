package main

import (
	"context"
	"path/filepath"
	"testing"

	"generate-all/parser"
	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestParseModules(t *testing.T) {
	engine := &script.Engine{
		Cmds: map[string]script.Cmd{
			"parse-tc-module": createParseTCModuleCommand(),
		},
	}
	
	// Add default script commands
	for name, cmd := range script.DefaultCmds() {
		engine.Cmds[name] = cmd
	}
	
	scripttest.Test(t, context.Background(), engine, nil, "testdata/*.txt")
}

// createParseTCModuleCommand creates the parse-tc-module command for scripttest
func createParseTCModuleCommand() script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "parse testcontainers module and generate testctr module",
			Args:    "-module name -out dir",
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			var moduleName, outputPath string
			
			// Parse arguments
			for i, arg := range args {
				switch arg {
				case "-module":
					if i+1 < len(args) {
						moduleName = args[i+1]
					}
				case "-out":
					if i+1 < len(args) {
						outputPath = args[i+1]
					}
				}
			}
			
			if moduleName == "" || outputPath == "" {
				return nil, script.ErrUsage
			}
			
			// Use our parser package to generate the module files
			fullOutputPath := filepath.Join(s.Getwd(), outputPath)
			err := parser.GenerateModuleFiles(moduleName, fullOutputPath)
			if err != nil {
				return nil, err
			}
			
			// Print debug information
			s.Logf("Generated files for module %s in %s", moduleName, fullOutputPath)
			
			return nil, nil
		},
	)
}