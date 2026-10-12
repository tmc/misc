//go:build unix

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSnapshotFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if f, err := openSnapshot(path); err == nil {
		f.Close()
		t.Fatal("FIFO accepted")
	}
}
func TestRendererValidation(t *testing.T) {
	dir := t.TempDir()
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(dir, "image.png")
	if err := os.WriteFile(data, pngData.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	renderer := filepath.Join(dir, "renderer")
	if err := os.WriteFile(renderer, []byte("#!/bin/sh\ncat > \"$0.svg\"\ncat '"+data+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if b, err := renderPNG(renderer, []byte("<svg/>")); err != nil || !bytes.Equal(b, pngData.Bytes()) {
		t.Fatalf("valid renderer: %v", err)
	}
	if got, err := os.ReadFile(renderer + ".svg"); err != nil || string(got) != "<svg/>" {
		t.Fatalf("renderer input = %q, error %v", got, err)
	}
	if err := os.WriteFile(renderer, []byte("#!/bin/sh\ncat > \"$0.svg\"\nprintf 'not PNG'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := renderPNG(renderer, []byte("<svg><path/></svg>")); err == nil || err.Error() != "renderer did not produce PNG" {
		t.Fatalf("invalid PNG: %v", err)
	}
	if got, err := os.ReadFile(renderer + ".svg"); err != nil || string(got) != "<svg><path/></svg>" {
		t.Fatalf("invalid renderer input = %q, error %v", got, err)
	}
}
func TestRendererBufferBound(t *testing.T) {
	b := boundedBuffer{limit: 2}
	if _, err := b.Write([]byte("abc")); err == nil {
		t.Fatal("oversized output accepted")
	}
}

func TestRendererCopyBounds(t *testing.T) {
	for _, size := range []int{2, 3} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			b := boundedBuffer{limit: 2}
			// Hide WriterTo so io.Copy must choose the destination copy path.
			r := struct{ io.Reader }{bytes.NewReader(bytes.Repeat([]byte("x"), size))}
			_, err := io.Copy(&b, r)
			if (err == nil) != (size == 2) || b.Len() > b.limit {
				t.Fatalf("size=%d retained=%d error=%v", size, b.Len(), err)
			}
		})
	}
}

func TestRendererPipeBounds(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		for _, size := range []int{2, 3} {
			t.Run(fmt.Sprintf("%s/%d", stream, size), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				script := "printf xx"
				if size == 3 {
					script = "printf xxx"
				}
				if stream == "stderr" {
					script += " >&2"
				}
				cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
				out, stderr := boundedBuffer{limit: 2}, boundedBuffer{limit: 2}
				cmd.Stdout, cmd.Stderr = &out, &stderr
				err := runRenderer(cmd)
				if (err == nil) != (size == 2) || out.Len() > 2 || stderr.Len() > 2 {
					t.Fatalf("stdout=%d stderr=%d error=%v", out.Len(), stderr.Len(), err)
				}
				if size == 2 && out.Len()+stderr.Len() != 2 {
					t.Fatal("valid bounded output lost")
				}
			})
		}
	}
}

func TestRendererSuccessfulLeaderCleanup(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	for _, redirected := range []bool{false, true} {
		t.Run(fmt.Sprint(redirected), func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "survivor")
			child := fmt.Sprintf("import os,time;os.write(int(__import__('sys').argv[1]),b'ready');os.close(int(__import__('sys').argv[1]));time.sleep(.8);open(%q,'w').write('survived')", marker)
			redirection := ""
			if redirected {
				redirection = ",stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL"
			}
			source := fmt.Sprintf("#!%s\nimport os,subprocess,sys,base64\nr,w=os.pipe()\nsubprocess.Popen([%q,'-c',%q,str(w)],pass_fds=(w,)%s)\nos.close(w);os.read(r,5);os.close(r)\nsys.stdin.buffer.read()\nsys.stderr.write('diagnostic')\nsys.stdout.buffer.write(base64.b64decode(%q))\n", python, python, child, redirection, base64.StdEncoding.EncodeToString(data.Bytes()))
			path := filepath.Join(dir, "renderer")
			if err := os.WriteFile(path, []byte(source), 0700); err != nil {
				t.Fatal(err)
			}
			got, err := renderPNG(path, bytes.Repeat([]byte("x"), 1<<20))
			if err != nil || !bytes.Equal(got, data.Bytes()) {
				t.Fatalf("PNG: %v", err)
			}
			time.Sleep(time.Second)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("renderer descendant survived leader exit")
			}
		})
	}
}
func TestRendererCancellationAndLargeIO(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			script := "cat; head -c 131072 /dev/zero; printf diagnostic >&2"
			if timeout {
				script = "sleep 30"
			}
			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
			cmd.Stdin = bytes.NewReader(bytes.Repeat([]byte("x"), 1<<20))
			out := boundedBuffer{limit: 2 << 20}
			stderr := boundedBuffer{limit: 64 << 10}
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			start := time.Now()
			err := runRenderer(cmd)
			if timeout {
				if err == nil || time.Since(start) > 2*time.Second {
					t.Fatal("cancellation exceeded budget", err)
				}
			} else if err != nil || out.Len() != (1<<20)+131072 || stderr.String() != "diagnostic" {
				t.Fatal("large IO", out.Len(), stderr.String(), err)
			}
		})
	}
}
