// Package filesystem implements MCP tools for filesystem operations.
package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/misc/mcptools/internal/mcp"
)

// ReadFileTool implements the read_file tool.
type ReadFileTool struct {
	allowedDirs []string
}

// NewReadFileTool creates a new read_file tool.
func NewReadFileTool(allowedDirs []string) *ReadFileTool {
	return &ReadFileTool{allowedDirs: allowedDirs}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the complete contents of a file from the file system. " +
		"Handles various text encodings and provides detailed error messages " +
		"if the file cannot be read."
}

func (t *ReadFileTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate path
	cleanPath := filepath.Clean(params.Path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path not allowed")
	}

	// Check if path is in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed")
	}

	// Read file
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", params.Path)
		}
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return []mcp.Content{{
		Type: mcp.ContentText,
		Text: string(data),
	}}, nil
}

// WriteFileTool implements the write_file tool.
type WriteFileTool struct {
	allowedDirs []string
}

// NewWriteFileTool creates a new write_file tool.
func NewWriteFileTool(allowedDirs []string) *WriteFileTool {
	return &WriteFileTool{allowedDirs: allowedDirs}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Create a new file or completely overwrite an existing file with new content. " +
		"Use with caution as it will overwrite existing files without warning. " +
		"Handles text content with proper encoding."
}

func (t *WriteFileTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate path
	cleanPath := filepath.Clean(params.Path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path not allowed")
	}

	// Check if path is in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed")
	}

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
		return nil, fmt.Errorf("error creating directories: %v", err)
	}

	// Write file
	if err := os.WriteFile(cleanPath, []byte(params.Content), 0644); err != nil {
		return nil, fmt.Errorf("error writing file: %v", err)
	}

	return []mcp.Content{{
		Type: mcp.ContentText,
		Text: fmt.Sprintf("wrote %d bytes to %s", len(params.Content), params.Path),
	}}, nil
}

// ListDirectoryTool implements the list_directory tool.
type ListDirectoryTool struct {
	allowedDirs []string
}

// NewListDirectoryTool creates a new list_directory tool.
func NewListDirectoryTool(allowedDirs []string) *ListDirectoryTool {
	return &ListDirectoryTool{allowedDirs: allowedDirs}
}

func (t *ListDirectoryTool) Name() string {
	return "list_directory"
}

func (t *ListDirectoryTool) Description() string {
	return "Get a detailed listing of all files and directories in a specified path. " +
		"Results clearly distinguish between files and directories with [FILE] and [DIR] prefixes."
}

func (t *ListDirectoryTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate path
	cleanPath := filepath.Clean(params.Path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path not allowed")
	}

	// Check if path is in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed")
	}

	// Read directory
	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not found: %s", params.Path)
		}
		return nil, fmt.Errorf("error reading directory: %v", err)
	}

	// Format entries
	var lines []string
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		prefix := "[FILE]"
		if info.IsDir() {
			prefix = "[DIR] "
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, entry.Name()))
	}

	return []mcp.Content{{
		Type: mcp.ContentText,
		Text: strings.Join(lines, "\n"),
	}}, nil
}

// SearchFilesTool implements the search_files tool.
type SearchFilesTool struct {
	allowedDirs []string
}

// NewSearchFilesTool creates a new search_files tool.
func NewSearchFilesTool(allowedDirs []string) *SearchFilesTool {
	return &SearchFilesTool{allowedDirs: allowedDirs}
}

func (t *SearchFilesTool) Name() string {
	return "search_files"
}

func (t *SearchFilesTool) Description() string {
	return "Recursively search for files and directories matching a pattern. " +
		"Searches through all subdirectories from the starting path. " +
		"The search is case-insensitive and matches partial names."
}

func (t *SearchFilesTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	var params struct {
		Path            string   `json:"path"`
		Pattern         string   `json:"pattern"`
		ExcludePatterns []string `json:"excludePatterns,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate path
	cleanPath := filepath.Clean(params.Path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path not allowed")
	}

	// Check if path is in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed")
	}

	// Compile exclude patterns
	var excludes []string
	for _, pattern := range params.ExcludePatterns {
		excludes = append(excludes, strings.ToLower(pattern))
	}

	// Search files
	pattern := strings.ToLower(params.Pattern)
	var matches []string
	err := filepath.Walk(cleanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check excludes
		for _, exclude := range excludes {
			if strings.Contains(strings.ToLower(path), exclude) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Check pattern
		if strings.Contains(strings.ToLower(path), pattern) {
			rel, err := filepath.Rel(cleanPath, path)
			if err != nil {
				return nil
			}
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error searching files: %v", err)
	}

	return []mcp.Content{{
		Type: mcp.ContentText,
		Text: strings.Join(matches, "\n"),
	}}, nil
}

// GetFileInfoTool implements the get_file_info tool.
type GetFileInfoTool struct {
	allowedDirs []string
}

// NewGetFileInfoTool creates a new get_file_info tool.
func NewGetFileInfoTool(allowedDirs []string) *GetFileInfoTool {
	return &GetFileInfoTool{allowedDirs: allowedDirs}
}

func (t *GetFileInfoTool) Name() string {
	return "get_file_info"
}

func (t *GetFileInfoTool) Description() string {
	return "Retrieve detailed metadata about a file or directory. " +
		"Returns comprehensive information including size, creation time, " +
		"last modified time, permissions, and type."
}

func (t *GetFileInfoTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate path
	cleanPath := filepath.Clean(params.Path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path not allowed")
	}

	// Check if path is in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed")
	}

	// Get file info
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", params.Path)
		}
		return nil, fmt.Errorf("error getting file info: %v", err)
	}

	// Format info
	var details []string
	details = append(details, fmt.Sprintf("Name: %s", info.Name()))
	details = append(details, fmt.Sprintf("Size: %d bytes", info.Size()))
	details = append(details, fmt.Sprintf("Mode: %s", info.Mode()))
	details = append(details, fmt.Sprintf("ModTime: %s", info.ModTime().Format(time.RFC3339)))
	details = append(details, fmt.Sprintf("IsDir: %v", info.IsDir()))

	return []mcp.Content{{
		Type: mcp.ContentText,
		Text: strings.Join(details, "\n"),
	}}, nil
}

// ListAllowedDirectoriesTool implements the list_allowed_directories tool.
type ListAllowedDirectoriesTool struct {
	allowedDirs []string
}

// NewListAllowedDirectoriesTool creates a new list_allowed_directories tool.
func NewListAllowedDirectoriesTool(allowedDirs []string) *ListAllowedDirectoriesTool {
	return &ListAllowedDirectoriesTool{allowedDirs: allowedDirs}
}

func (t *ListAllowedDirectoriesTool) Name() string {
	return "list_allowed_directories"
}

func (t *ListAllowedDirectoriesTool) Description() string {
	return "Returns the list of directories that this server is allowed to access."
}

func (t *ListAllowedDirectoriesTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	return []mcp.Content{{
		Type: mcp.ContentText,
		Text: strings.Join(t.allowedDirs, "\n"),
	}}, nil
}

// CopyFileTool implements the copy_file tool.
type CopyFileTool struct {
	allowedDirs []string
}

// NewCopyFileTool creates a new copy_file tool.
func NewCopyFileTool(allowedDirs []string) *CopyFileTool {
	return &CopyFileTool{allowedDirs: allowedDirs}
}

func (t *CopyFileTool) Name() string {
	return "copy_file"
}

func (t *CopyFileTool) Description() string {
	return "Copy a file or directory to a new location. " +
		"Preserves file mode and timestamps. " +
		"For directories, performs a recursive copy."
}

func (t *CopyFileTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	var params struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate paths
	cleanSrc := filepath.Clean(params.Source)
	cleanDst := filepath.Clean(params.Destination)
	if strings.Contains(cleanSrc, "..") || strings.Contains(cleanDst, "..") {
		return nil, fmt.Errorf("path not allowed: contains ..")
	}

	// Check if paths are in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absSrc, err := filepath.Abs(cleanSrc)
		if err != nil {
			continue
		}
		absDst, err := filepath.Abs(cleanDst)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absSrc, absDir) && strings.HasPrefix(absDst, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed: outside allowed directories")
	}

	// Get source info
	info, err := os.Lstat(cleanSrc)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source not found: %s", params.Source)
		}
		return nil, fmt.Errorf("error accessing source: %v", err)
	}

	// Handle based on source type
	if info.Mode()&os.ModeSymlink != 0 {
		// Copy symlink
		target, err := os.Readlink(cleanSrc)
		if err != nil {
			return nil, fmt.Errorf("error reading symlink: %v", err)
		}
		if err := os.Symlink(target, cleanDst); err != nil {
			return nil, fmt.Errorf("error creating symlink: %v", err)
		}
	} else if info.IsDir() {
		// Copy directory recursively
		if err := copyDir(cleanSrc, cleanDst); err != nil {
			return nil, fmt.Errorf("error copying directory: %v", err)
		}
	} else {
		// Copy regular file
		if err := copyFile(cleanSrc, cleanDst); err != nil {
			return nil, fmt.Errorf("error copying file: %v", err)
		}
	}

	return []mcp.Content{{
		Type: mcp.ContentText,
		Text: fmt.Sprintf("copied %s to %s", params.Source, params.Destination),
	}}, nil
}

// DeleteFileTool implements the delete_file tool.
type DeleteFileTool struct {
	allowedDirs []string
}

// NewDeleteFileTool creates a new delete_file tool.
func NewDeleteFileTool(allowedDirs []string) *DeleteFileTool {
	return &DeleteFileTool{allowedDirs: allowedDirs}
}

func (t *DeleteFileTool) Name() string {
	return "delete_file"
}

func (t *DeleteFileTool) Description() string {
	return "Delete a file or directory. " +
		"For directories, performs a recursive delete. " +
		"Use with caution as this operation cannot be undone."
}

func (t *DeleteFileTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate path
	cleanPath := filepath.Clean(params.Path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path not allowed: contains ..")
	}

	// Check if path is in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed: outside allowed directories")
	}

	// Get file info
	info, err := os.Lstat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path not found: %s", params.Path)
		}
		return nil, fmt.Errorf("error accessing path: %v", err)
	}

	// Delete the file or directory
	if err := os.RemoveAll(cleanPath); err != nil {
		return nil, fmt.Errorf("error deleting path: %v", err)
	}

	return []mcp.Content{{
		Type: mcp.ContentText,
		Text: fmt.Sprintf("deleted %s", params.Path),
	}}, nil
}

// MoveFileTool implements the move_file tool.
type MoveFileTool struct {
	allowedDirs []string
}

// NewMoveFileTool creates a new move_file tool.
func NewMoveFileTool(allowedDirs []string) *MoveFileTool {
	return &MoveFileTool{allowedDirs: allowedDirs}
}

func (t *MoveFileTool) Name() string {
	return "move_file"
}

func (t *MoveFileTool) Description() string {
	return "Move a file or directory to a new location. " +
		"Performs an atomic move when possible (same filesystem). " +
		"Falls back to copy+delete when necessary (cross-device)."
}

func (t *MoveFileTool) Handle(ctx context.Context, args json.RawMessage) ([]mcp.Content, error) {
	var params struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %v", err)
	}

	// Clean and validate paths
	cleanSrc := filepath.Clean(params.Source)
	cleanDst := filepath.Clean(params.Destination)
	if strings.Contains(cleanSrc, "..") || strings.Contains(cleanDst, "..") {
		return nil, fmt.Errorf("path not allowed: contains ..")
	}

	// Check if paths are in allowed directories
	allowed := false
	for _, dir := range t.allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absSrc, err := filepath.Abs(cleanSrc)
		if err != nil {
			continue
		}
		absDst, err := filepath.Abs(cleanDst)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absSrc, absDir) && strings.HasPrefix(absDst, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("path not allowed: outside allowed directories")
	}

	// Get source info
	info, err := os.Lstat(cleanSrc)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source not found: %s", params.Source)
		}
		return nil, fmt.Errorf("error accessing source: %v", err)
	}

	// Try atomic move first
	err = os.Rename(cleanSrc, cleanDst)
	if err == nil {
		return []mcp.Content{{
			Type: mcp.ContentText,
			Text: fmt.Sprintf("moved %s to %s", params.Source, params.Destination),
		}}, nil
	}

	// If atomic move failed, try copy+delete
	if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err == syscall.EXDEV {
		// Cross-device move needed
		if info.Mode()&os.ModeSymlink != 0 {
			// Copy symlink
			target, err := os.Readlink(cleanSrc)
			if err != nil {
				return nil, fmt.Errorf("error reading symlink: %v", err)
			}
			if err := os.Symlink(target, cleanDst); err != nil {
				return nil, fmt.Errorf("error creating symlink: %v", err)
			}
		} else if info.IsDir() {
			// Copy directory recursively
			if err := copyDir(cleanSrc, cleanDst); err != nil {
				return nil, fmt.Errorf("error copying directory: %v", err)
			}
		} else {
			// Copy regular file
			if err := copyFile(cleanSrc, cleanDst); err != nil {
				return nil, fmt.Errorf("error copying file: %v", err)
			}
		}

		// Delete source after successful copy
		if err := os.RemoveAll(cleanSrc); err != nil {
			// Try to clean up destination on error
			os.RemoveAll(cleanDst)
			return nil, fmt.Errorf("error removing source after copy: %v", err)
		}

		return []mcp.Content{{
			Type: mcp.ContentText,
			Text: fmt.Sprintf("moved %s to %s (copy+delete)", params.Source, params.Destination),
		}}, nil
	}

	return nil, fmt.Errorf("error moving file: %v", err)
}

// Helper functions for file operations

func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get source file info
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Sync to ensure write is complete
	return dstFile.Sync()
}

func copyDir(src, dst string) error {
	// Get source info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// Copy symlink
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
		} else if info.IsDir() {
			// Recursively copy directory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy regular file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
