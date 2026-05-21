// Package webkithost hosts Wanix browser assets in a WebKit view.
package webkithost

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// Assets resolves wanix://runtime URLs to bytes.
type Assets struct {
	Dir    string
	RCPath string
}

// Asset is one response served to WebKit.
type Asset struct {
	Data []byte
	MIME string
}

// Open returns the asset for path. The path is rooted at wanix://runtime/.
func (a Assets) Open(path string) (Asset, error) {
	name, ok := cleanAssetPath(path)
	if !ok {
		return Asset{}, fmt.Errorf("open %s: invalid asset path", path)
	}
	if name == "" || name == "/" {
		name = "index.html"
	}
	if name == "index.html" {
		return Asset{
			Data: []byte(BootstrapHTML),
			MIME: "text/html; charset=utf-8",
		}, nil
	}
	if name == "rc.wasm" && a.RCPath != "" {
		data, err := os.ReadFile(a.RCPath)
		if err != nil {
			return Asset{}, fmt.Errorf("open %s: %w", name, err)
		}
		return Asset{Data: data, MIME: MIME(name)}, nil
	}
	if a.Dir == "" {
		return Asset{}, fmt.Errorf("open %s: asset directory not configured", name)
	}
	filename := filepath.Join(a.Dir, filepath.FromSlash(name))
	data, err := os.ReadFile(filename)
	if err != nil {
		return Asset{}, fmt.Errorf("open %s: %w", name, err)
	}
	return Asset{Data: data, MIME: MIME(name)}, nil
}

func cleanAssetPath(path string) (string, bool) {
	path = strings.TrimPrefix(path, "/")
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." {
		return "", true
	}
	if path == ".." || strings.HasPrefix(path, "../") {
		return "", false
	}
	return path, true
}

// MIME returns the content type WebKit should see for name.
func MIME(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".wasm":
		return "application/wasm"
	case ".js", ".mjs":
		return "text/javascript"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	}
	if typ := mime.TypeByExtension(filepath.Ext(name)); typ != "" {
		return typ
	}
	return "application/octet-stream"
}
