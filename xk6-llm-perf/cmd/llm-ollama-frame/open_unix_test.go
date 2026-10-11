//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRegularInputAdmission(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "input.json")
	if err := os.WriteFile(regular, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := openRegular(regular)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	link := filepath.Join(root, "link")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(root, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, fifo, root} {
		done := make(chan error, 1)
		go func() {
			file, err := openRegular(path)
			if file != nil {
				file.Close()
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("nonregular input accepted")
			}
		case <-time.After(time.Second):
			t.Fatal("input admission blocked")
		}
	}
}
