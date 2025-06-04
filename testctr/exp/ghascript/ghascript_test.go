package ghascript_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tmc/misc/testctr/exp/ghascript"
	"rsc.io/script"
	"rsc.io/script/scripttest"
)

func TestGHAScript(t *testing.T) {
	t.Parallel()
	// Create engine with GitHub Actions commands and conditions
	engine := &script.Engine{
		Cmds:  ghascript.DefaultCmds(t),
		Conds: ghascript.DefaultConds(),
		Quiet: !testing.Verbose(),
	}

	// Run tests in testdata directory
	testdata := "testdata"
	files, err := os.ReadDir(testdata)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".txt" {
			continue
		}

		name := file.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// Read script file
			scriptPath := filepath.Join(testdata, name)
			scriptFile, err := os.Open(scriptPath)
			if err != nil {
				t.Fatal(err)
			}
			defer scriptFile.Close()

			// Create initial state
			state, err := script.NewState(ctx, testdata, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Run the script
			scripttest.Run(t, engine, state, name, scriptFile)
		})
	}
}

func TestWorkflowTest(t *testing.T) {
	t.Parallel()
	
	// Test the Test function with custom options
	ghascript.Test(t,
		ghascript.WithEvents("push"),
		ghascript.WithTimeout(5*time.Minute),
		ghascript.WithWorkflowsDir("testdata"),
	)
}

func TestWorkflowTestSequential(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sequential test in short mode")
	}
	
	// Test sequential execution
	ghascript.Test(t,
		ghascript.WithEvents("push", "pull_request"),
		ghascript.WithTimeout(2*time.Minute),
		ghascript.WithWorkflowsDir("testdata"),
		ghascript.WithSequential(),
	)
}