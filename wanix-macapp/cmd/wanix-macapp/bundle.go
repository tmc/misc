package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmc/macgo/codesign"
)

func buildBundle(appPath, name, assetsDir, rcWASM string) error {
	if err := os.RemoveAll(appPath); err != nil {
		return fmt.Errorf("remove old app: %w", err)
	}
	contents := filepath.Join(appPath, "Contents")
	macos := filepath.Join(contents, "MacOS")
	resources := filepath.Join(contents, "Resources", "wanix")
	if err := os.MkdirAll(macos, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(resources, 0755); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := copyFile(filepath.Join(macos, name), exe, 0755); err != nil {
		return fmt.Errorf("copy executable: %w", err)
	}
	if err := copyTree(resources, assetsDir); err != nil {
		return fmt.Errorf("copy wanix assets: %w", err)
	}
	if rcWASM != "" {
		if err := copyFile(filepath.Join(resources, "rc.wasm"), rcWASM, 0644); err != nil {
			return fmt.Errorf("copy rc wasm: %w", err)
		}
	}
	if err := writeInfoPlist(filepath.Join(contents, "Info.plist"), name); err != nil {
		return err
	}
	if err := codesignBundle(appPath); err != nil {
		return err
	}
	fmt.Println(appPath)
	return nil
}

func writeInfoPlist(path, name string) error {
	bundleID := "dev.tmc.wanix." + sanitizeBundlePart(name)
	text := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>%s</string>
  <key>CFBundleIdentifier</key>
  <string>%s</string>
  <key>CFBundleName</key>
  <string>%s</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>0.1.0</string>
  <key>CFBundleVersion</key>
  <string>0.1.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
`, xmlEscape(name), xmlEscape(bundleID), xmlEscape(name))
	return os.WriteFile(path, []byte(text), 0644)
}

func codesignBundle(appPath string) error {
	cmd := exec.Command("codesign", "--force", "--sign", "-", appPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codesign: %w", err)
	}
	return codesign.VerifySignature(appPath)
}

func copyTree(dst, src string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().Type() != 0 {
			return nil
		}
		return copyFile(target, path, info.Mode().Perm())
	})
}

func copyFile(dst, src string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func sanitizeBundlePart(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "app"
	}
	if out[0] >= '0' && out[0] <= '9' {
		return "app-" + out
	}
	return out
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
