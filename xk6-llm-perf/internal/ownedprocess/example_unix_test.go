//go:build unix

package ownedprocess_test

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/tmc/misc/xk6-llm-perf/internal/ownedprocess"
)

func ExampleRun() {
	cmd := exec.CommandContext(context.Background(), "echo", "hello")
	var output bytes.Buffer
	cmd.Stdout = &output
	if err := ownedprocess.Run(cmd); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(output.String())
	// Output: hello
}
