//go:build !unix

package llmperf

import (
	"fmt"
	"os"
	"os/exec"
)

func openRuntimeTemplate(path string) (*os.File, error) {
	return nil, fmt.Errorf("runtime template worker requires unix process groups")
}

func runRuntimeTemplate(cmd *exec.Cmd) error {
	return fmt.Errorf("runtime template worker requires unix process groups")
}
