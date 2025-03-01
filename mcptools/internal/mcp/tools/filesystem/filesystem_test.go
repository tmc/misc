package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/misc/mcptools/internal/mcp"
)

func TestReadFileTool(t *testing.T) {
	dir := t.TempDir()

	// Create test file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello\nworld\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewReadFileTool([]string{dir})

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "valid file",
			path: testFile,
			want: "hello\nworld\n",
		},
		{
			name:    "file not found",
			path:    filepath.Join(dir, "nonexistent.txt"),
			wantErr: true,
		},
		{
			name:    "path not allowed",
			path:    "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "path with ..",
			path:    filepath.Join(dir, "..", "test.txt"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(struct {
				Path string `json:"path"`
			}{Path: tt.path})

			content, err := tool.Handle(context.Background(), args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(content) != 1 {
				t.Errorf("got %d content items, want 1", len(content))
				return
			}
			if content[0].Type != mcp.ContentText {
				t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
			}
			if content[0].Text != tt.want {
				t.Errorf("got text %q, want %q", content[0].Text, tt.want)
			}
		})
	}
}

func TestWriteFileTool(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFileTool([]string{dir})

	tests := []struct {
		name     string
		path     string
		content  string
		wantErr  bool
		validate func(t *testing.T, path string)
	}{
		{
			name:    "valid file",
			path:    filepath.Join(dir, "test.txt"),
			content: "hello\nworld\n",
			validate: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Error(err)
					return
				}
				if string(data) != "hello\nworld\n" {
					t.Errorf("got content %q, want %q", string(data), "hello\nworld\n")
				}
			},
		},
		{
			name:    "create directories",
			path:    filepath.Join(dir, "subdir", "test.txt"),
			content: "test",
			validate: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Error(err)
					return
				}
				if string(data) != "test" {
					t.Errorf("got content %q, want %q", string(data), "test")
				}
			},
		},
		{
			name:     "path not allowed",
			path:     "/etc/passwd",
			content:  "test",
			wantErr:  true,
			validate: func(t *testing.T, path string) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}{Path: tt.path, Content: tt.content})

			content, err := tool.Handle(context.Background(), args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(content) != 1 {
				t.Errorf("got %d content items, want 1", len(content))
				return
			}
			if content[0].Type != mcp.ContentText {
				t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
			}

			tt.validate(t, tt.path)
		})
	}
}

func TestListDirectoryTool(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	tool := NewListDirectoryTool([]string{dir})

	tests := []struct {
		name    string
		path    string
		want    []string
		wantErr bool
	}{
		{
			name: "valid directory",
			path: dir,
			want: []string{
				"[FILE] file1.txt",
				"[DIR]  subdir",
			},
		},
		{
			name:    "directory not found",
			path:    filepath.Join(dir, "nonexistent"),
			wantErr: true,
		},
		{
			name:    "path not allowed",
			path:    "/etc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(struct {
				Path string `json:"path"`
			}{Path: tt.path})

			content, err := tool.Handle(context.Background(), args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(content) != 1 {
				t.Errorf("got %d content items, want 1", len(content))
				return
			}
			if content[0].Type != mcp.ContentText {
				t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
			}

			got := strings.Split(strings.TrimSpace(content[0].Text), "\n")
			if len(got) != len(tt.want) {
				t.Errorf("got %d lines, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSearchFilesTool(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subdir", "file2.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewSearchFilesTool([]string{dir})

	tests := []struct {
		name            string
		path            string
		pattern         string
		excludePatterns []string
		want            []string
		wantErr         bool
	}{
		{
			name:    "find all txt files",
			path:    dir,
			pattern: ".txt",
			want: []string{
				"file1.txt",
				filepath.Join("subdir", "file2.txt"),
			},
		},
		{
			name:            "exclude subdir",
			path:            dir,
			pattern:         ".txt",
			excludePatterns: []string{"subdir"},
			want: []string{
				"file1.txt",
			},
		},
		{
			name:    "path not allowed",
			path:    "/etc",
			pattern: "passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(struct {
				Path            string   `json:"path"`
				Pattern         string   `json:"pattern"`
				ExcludePatterns []string `json:"excludePatterns,omitempty"`
			}{
				Path:            tt.path,
				Pattern:         tt.pattern,
				ExcludePatterns: tt.excludePatterns,
			})

			content, err := tool.Handle(context.Background(), args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(content) != 1 {
				t.Errorf("got %d content items, want 1", len(content))
				return
			}
			if content[0].Type != mcp.ContentText {
				t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
			}

			got := strings.Split(strings.TrimSpace(content[0].Text), "\n")
			if len(got) != len(tt.want) {
				t.Errorf("got %d lines, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetFileInfoTool(t *testing.T) {
	dir := t.TempDir()

	// Create test file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewGetFileInfoTool([]string{dir})

	tests := []struct {
		name    string
		path    string
		check   func(t *testing.T, content string)
		wantErr bool
	}{
		{
			name: "valid file",
			path: testFile,
			check: func(t *testing.T, content string) {
				if !strings.Contains(content, "Name: test.txt") {
					t.Error("missing name")
				}
				if !strings.Contains(content, "Size: 4 bytes") {
					t.Error("missing size")
				}
				if !strings.Contains(content, "IsDir: false") {
					t.Error("missing isdir")
				}
			},
		},
		{
			name:    "file not found",
			path:    filepath.Join(dir, "nonexistent.txt"),
			wantErr: true,
		},
		{
			name:    "path not allowed",
			path:    "/etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(struct {
				Path string `json:"path"`
			}{Path: tt.path})

			content, err := tool.Handle(context.Background(), args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(content) != 1 {
				t.Errorf("got %d content items, want 1", len(content))
				return
			}
			if content[0].Type != mcp.ContentText {
				t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
			}

			tt.check(t, content[0].Text)
		})
	}
}

func TestListAllowedDirectoriesTool(t *testing.T) {
	dirs := []string{"/tmp", "/home/user"}
	tool := NewListAllowedDirectoriesTool(dirs)

	content, err := tool.Handle(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(content) != 1 {
		t.Errorf("got %d content items, want 1", len(content))
		return
	}
	if content[0].Type != mcp.ContentText {
		t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
	}

	got := strings.Split(strings.TrimSpace(content[0].Text), "\n")
	if len(got) != len(dirs) {
		t.Errorf("got %d lines, want %d", len(got), len(dirs))
		return
	}
	for i := range got {
		if got[i] != dirs[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], dirs[i])
		}
	}
}

func TestCopyFileTool(t *testing.T) {
	dir := t.TempDir()

	// Create test file
	srcFile := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create test symlink
	symFile := filepath.Join(dir, "link.txt")
	if err := os.Symlink(srcFile, symFile); err != nil {
		t.Fatal(err)
	}

	// Create test directory
	srcDir := filepath.Join(dir, "srcdir")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewCopyFileTool([]string{dir})

	tests := []struct {
		name    string
		src     string
		dst     string
		check   func(t *testing.T, dst string)
		wantErr bool
	}{
		{
			name: "copy file",
			src:  srcFile,
			dst:  filepath.Join(dir, "dst.txt"),
			check: func(t *testing.T, dst string) {
				data, err := os.ReadFile(dst)
				if err != nil {
					t.Error(err)
					return
				}
				if string(data) != "test" {
					t.Errorf("got content %q, want %q", string(data), "test")
				}
			},
		},
		{
			name: "copy symlink",
			src:  symFile,
			dst:  filepath.Join(dir, "newlink.txt"),
			check: func(t *testing.T, dst string) {
				target, err := os.Readlink(dst)
				if err != nil {
					t.Error(err)
					return
				}
				if target != srcFile {
					t.Errorf("got target %q, want %q", target, srcFile)
				}
			},
		},
		{
			name: "copy directory",
			src:  srcDir,
			dst:  filepath.Join(dir, "dstdir"),
			check: func(t *testing.T, dst string) {
				data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
				if err != nil {
					t.Error(err)
					return
				}
				if string(data) != "test" {
					t.Errorf("got content %q, want %q", string(data), "test")
				}
			},
		},
		{
			name:    "source not found",
			src:     filepath.Join(dir, "nonexistent.txt"),
			dst:     filepath.Join(dir, "dst.txt"),
			wantErr: true,
		},
		{
			name:    "path not allowed",
			src:     "/etc/passwd",
			dst:     filepath.Join(dir, "dst.txt"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(struct {
				Source      string `json:"source"`
				Destination string `json:"destination"`
			}{Source: tt.src, Destination: tt.dst})

			content, err := tool.Handle(context.Background(), args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(content) != 1 {
				t.Errorf("got %d content items, want 1", len(content))
				return
			}
			if content[0].Type != mcp.ContentText {
				t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
			}

			tt.check(t, tt.dst)
		})
	}
}

func TestDeleteFileTool(t *testing.T) {
	dir := t.TempDir()

	// Create test file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create test directory
	testDir := filepath.Join(dir, "testdir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewDeleteFileTool([]string{dir})

	tests := []struct {
		name    string
		path    string
		check   func(t *testing.T, path string)
		wantErr bool
	}{
		{
			name: "delete file",
			path: testFile,
			check: func(t *testing.T, path string) {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Error("file still exists")
				}
			},
		},
		{
			name: "delete directory",
			path: testDir,
			check: func(t *testing.T, path string) {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Error("directory still exists")
				}
			},
		},
		{
			name:    "path not found",
			path:    filepath.Join(dir, "nonexistent.txt"),
			wantErr: true,
		},
		{
			name:    "path not allowed",
			path:    "/etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(struct {
				Path string `json:"path"`
			}{Path: tt.path})

			content, err := tool.Handle(context.Background(), args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(content) != 1 {
				t.Errorf("got %d content items, want 1", len(content))
				return
			}
			if content[0].Type != mcp.ContentText {
				t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
			}

			tt.check(t, tt.path)
		})
	}
}

func TestMoveFileTool(t *testing.T) {
	dir := t.TempDir()

	// Create test file
	srcFile := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(srcFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create test symlink
	symFile := filepath.Join(dir, "link.txt")
	if err := os.Symlink(srcFile, symFile); err != nil {
		t.Fatal(err)
	}

	// Create test directory
	srcDir := filepath.Join(dir, "srcdir")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewMoveFileTool([]string{dir})

	tests := []struct {
		name    string
		src     string
		dst     string
		check   func(t *testing.T, src, dst string)
		wantErr bool
	}{
		{
			name: "move file",
			src:  srcFile,
			dst:  filepath.Join(dir, "dst.txt"),
			check: func(t *testing.T, src, dst string) {
				// Source should be gone
				if _, err := os.Stat(src); !os.IsNotExist(err) {
					t.Error("source file still exists")
				}
				// Destination should have content
				data, err := os.ReadFile(dst)
				if err != nil {
					t.Error(err)
					return
				}
				if string(data) != "test" {
					t.Errorf("got content %q, want %q", string(data), "test")
				}
			},
		},
		{
			name: "move symlink",
			src:  symFile,
			dst:  filepath.Join(dir, "newlink.txt"),
			check: func(t *testing.T, src, dst string) {
				// Source should be gone
				if _, err := os.Stat(src); !os.IsNotExist(err) {
					t.Error("source symlink still exists")
				}
				// Destination should be a symlink to original target
				target, err := os.Readlink(dst)
				if err != nil {
					t.Error(err)
					return
				}
				if target != srcFile {
					t.Errorf("got target %q, want %q", target, srcFile)
				}
			},
		},
		{
			name: "move directory",
			src:  srcDir,
			dst:  filepath.Join(dir, "dstdir"),
			check: func(t *testing.T, src, dst string) {
				// Source should be gone
				if _, err := os.Stat(src); !os.IsNotExist(err) {
					t.Error("source directory still exists")
				}
				// Destination should have content
				data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
				if err != nil {
					t.Error(err)
					return
				}
				if string(data) != "test" {
					t.Errorf("got content %q, want %q", string(data), "test")
				}
			},
		},
		{
			name:    "source not found",
			src:     filepath.Join(dir, "nonexistent.txt"),
			dst:     filepath.Join(dir, "dst.txt"),
			wantErr: true,
		},
		{
			name:    "path not allowed",
			src:     "/etc/passwd",
			dst:     filepath.Join(dir, "dst.txt"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(struct {
				Source      string `json:"source"`
				Destination string `json:"destination"`
			}{Source: tt.src, Destination: tt.dst})

			content, err := tool.Handle(context.Background(), args)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if len(content) != 1 {
				t.Errorf("got %d content items, want 1", len(content))
				return
			}
			if content[0].Type != mcp.ContentText {
				t.Errorf("got type %q, want %q", content[0].Type, mcp.ContentText)
			}

			tt.check(t, tt.src, tt.dst)
		})
	}
}
