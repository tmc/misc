//go:build unix

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/tmc/misc/xk6-llm-perf/internal/ownedprocess"
)

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *boundedBuffer) Len() int       { return b.buffer.Len() }
func (b *boundedBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *boundedBuffer) String() string { return b.buffer.String() }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("PNG renderer output exceeds limit")
	}
	return b.buffer.Write(p)
}
func renderPNG(path string, svg []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--format=png")
	cmd.Stdin = bytes.NewReader(svg)
	var out boundedBuffer
	out.limit = 64 * 1024 * 1024
	var stderr boundedBuffer
	stderr.limit = 64 * 1024
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := runRenderer(cmd)
	if err != nil {
		return nil, fmt.Errorf("render PNG: %w: %s", err, stderr.String())
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		return nil, fmt.Errorf("renderer did not produce PNG")
	}
	config, err := png.DecodeConfig(bytes.NewReader(out.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	if config.Width > 4096 || config.Height > 4096 {
		return nil, fmt.Errorf("PNG dimensions exceed limit")
	}
	if _, err := png.Decode(bytes.NewReader(out.Bytes())); err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	return out.Bytes(), nil
}

func runRenderer(cmd *exec.Cmd) error {
	err := ownedprocess.Run(cmd)
	if errors.Is(err, ownedprocess.ErrCleanup) {
		return fmt.Errorf("PNG renderer pipe cleanup exceeded budget")
	}
	return err
}

func openSnapshot(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("run snapshot must be regular file")
	}
	return f, nil
}
