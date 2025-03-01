package validate

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitTagExists checks if a git tag exists in the repository
func GitTagExists(tag string) error {
	cmd := exec.Command("git", "tag", "-l", tag)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error checking git tag: %w", err)
	}

	if strings.TrimSpace(string(output)) != tag {
		return fmt.Errorf("git tag %s not found. Hint: create tag with: git tag %s", tag, tag)
	}
	
	return nil
}

// GitWorkingDirClean checks if the git working directory is clean
func GitWorkingDirClean() error {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error checking git status: %w", err)
	}

	if len(strings.TrimSpace(string(output))) > 0 {
		return fmt.Errorf("git working directory is not clean. Commit or stash changes first")
	}
	
	return nil
}