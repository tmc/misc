package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tmc/misc/mcptools/internal/mcp"
	"github.com/tmc/misc/mcptools/internal/mcp/scripttest"
)

func TestScripts(t *testing.T) {
	// Create test directory
	dir := t.TempDir()
	testdata := filepath.Join(dir, "testdata")
	if err := os.MkdirAll(testdata, 0755); err != nil {
		t.Fatal(err)
	}

	// Create server
	srv := mcp.NewServer("filesystem-server", "1.0.0")

	// Register tools
	tools := []mcp.Tool{
		NewReadFileTool([]string{dir}),
		NewWriteFileTool([]string{dir}),
		NewListDirectoryTool([]string{dir}),
		NewSearchFilesTool([]string{dir}),
		NewGetFileInfoTool([]string{dir}),
		NewListAllowedDirectoriesTool([]string{dir}),
		NewCopyFileTool([]string{dir}),
		NewDeleteFileTool([]string{dir}),
		NewMoveFileTool([]string{dir}),
	}
	for _, tool := range tools {
		if err := srv.RegisterTool(tool); err != nil {
			t.Fatal(err)
		}
	}

	// Find test scripts
	scripts, err := scripttest.FindTestScripts("testdata")
	if err != nil {
		t.Fatal(err)
	}

	// Run each script
	for _, script := range scripts {
		t.Run(filepath.Base(script), func(t *testing.T) {
			// Create a clean test directory for each script
			if err := os.RemoveAll(testdata); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(testdata, 0755); err != nil {
				t.Fatal(err)
			}

			// Run script
			scripttest.TestScript(t, func(msg []byte) ([]byte, error) {
				return srv.Handle(context.Background(), msg)
			}, script)
		})
	}
}
