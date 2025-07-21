package differential

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/pkg/errors"
)

// CaptureMetadata stores metadata about a capture
type CaptureMetadata struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Timestamp   time.Time         `json:"timestamp"`
	URL         string            `json:"url"`
	UserAgent   string            `json:"user_agent"`
	Viewport    *Viewport         `json:"viewport,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	FilePath    string            `json:"file_path"`
	Checksum    string            `json:"checksum"`
	Size        int64             `json:"size"`
	EntryCount  int               `json:"entry_count"`
	Duration    time.Duration     `json:"duration"`
	Status      CaptureStatus     `json:"status"`
	Description string            `json:"description,omitempty"`
}

// Viewport represents browser viewport dimensions
type Viewport struct {
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

// CaptureStatus represents the status of a capture
type CaptureStatus string

const (
	CaptureStatusPending   CaptureStatus = "pending"
	CaptureStatusRecording CaptureStatus = "recording"
	CaptureStatusCompleted CaptureStatus = "completed"
	CaptureStatusFailed    CaptureStatus = "failed"
)

// CaptureManager manages multiple HAR captures and their metadata
type CaptureManager struct {
	workDir      string
	captures     map[string]*CaptureMetadata
	verbose      bool
	metadataFile string
}

// NewCaptureManager creates a new capture manager
func NewCaptureManager(workDir string, verbose bool) (*CaptureManager, error) {
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, errors.Wrap(err, "creating work directory")
	}

	cm := &CaptureManager{
		workDir:      workDir,
		captures:     make(map[string]*CaptureMetadata),
		verbose:      verbose,
		metadataFile: filepath.Join(workDir, "captures.json"),
	}

	// Load existing captures
	if err := cm.loadMetadata(); err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "loading existing captures")
		}
	}

	return cm, nil
}

// CreateCapture creates a new capture with metadata
func (cm *CaptureManager) CreateCapture(name, url, description string, labels map[string]string) (*CaptureMetadata, error) {
	id := cm.generateCaptureID(name)

	metadata := &CaptureMetadata{
		ID:          id,
		Name:        name,
		Timestamp:   time.Now(),
		URL:         url,
		Labels:      labels,
		FilePath:    filepath.Join(cm.workDir, fmt.Sprintf("%s.har", id)),
		Status:      CaptureStatusPending,
		Description: description,
	}

	cm.captures[id] = metadata

	if err := cm.saveMetadata(); err != nil {
		return nil, errors.Wrap(err, "saving metadata")
	}

	if cm.verbose {
		fmt.Printf("Created capture: %s (ID: %s)\n", name, id)
	}

	return metadata, nil
}

// StartCapture marks a capture as recording
func (cm *CaptureManager) StartCapture(id string) error {
	capture, exists := cm.captures[id]
	if !exists {
		return fmt.Errorf("capture not found: %s", id)
	}

	capture.Status = CaptureStatusRecording
	capture.Timestamp = time.Now()

	return cm.saveMetadata()
}

// CompleteCapture marks a capture as completed and updates its metadata
func (cm *CaptureManager) CompleteCapture(id string, harData *har.HAR) error {
	capture, exists := cm.captures[id]
	if !exists {
		return fmt.Errorf("capture not found: %s", id)
	}

	// Calculate duration
	capture.Duration = time.Since(capture.Timestamp)

	// Update metadata from HAR data
	if harData != nil && harData.Log != nil {
		capture.EntryCount = len(harData.Log.Entries)

		// Extract user agent if available
		if len(harData.Log.Entries) > 0 {
			for _, header := range harData.Log.Entries[0].Request.Headers {
				if header.Name == "User-Agent" {
					capture.UserAgent = header.Value
					break
				}
			}
		}
	}

	// Write HAR file
	if err := cm.writeHARFile(capture.FilePath, harData); err != nil {
		capture.Status = CaptureStatusFailed
		return errors.Wrap(err, "writing HAR file")
	}

	// Calculate file size and checksum
	if err := cm.updateFileMetadata(capture); err != nil {
		return errors.Wrap(err, "updating file metadata")
	}

	capture.Status = CaptureStatusCompleted

	if cm.verbose {
		fmt.Printf("Completed capture: %s (%d entries, %d bytes)\n",
			capture.Name, capture.EntryCount, capture.Size)
	}

	return cm.saveMetadata()
}

// GetCapture retrieves capture metadata by ID
func (cm *CaptureManager) GetCapture(id string) (*CaptureMetadata, error) {
	capture, exists := cm.captures[id]
	if !exists {
		return nil, fmt.Errorf("capture not found: %s", id)
	}
	return capture, nil
}

// ListCaptures returns all capture metadata
func (cm *CaptureManager) ListCaptures() []*CaptureMetadata {
	captures := make([]*CaptureMetadata, 0, len(cm.captures))
	for _, capture := range cm.captures {
		captures = append(captures, capture)
	}

	// Sort by timestamp (newest first)
	sort.Slice(captures, func(i, j int) bool {
		return captures[i].Timestamp.After(captures[j].Timestamp)
	})

	return captures
}

// FindCapturesByLabel finds captures with specific label values
func (cm *CaptureManager) FindCapturesByLabel(key, value string) []*CaptureMetadata {
	var matches []*CaptureMetadata

	for _, capture := range cm.captures {
		if capture.Labels != nil {
			if labelValue, exists := capture.Labels[key]; exists && labelValue == value {
				matches = append(matches, capture)
			}
		}
	}

	return matches
}

// DeleteCapture removes a capture and its HAR file
func (cm *CaptureManager) DeleteCapture(id string) error {
	capture, exists := cm.captures[id]
	if !exists {
		return fmt.Errorf("capture not found: %s", id)
	}

	// Remove HAR file
	if err := os.Remove(capture.FilePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "removing HAR file")
	}

	// Remove from metadata
	delete(cm.captures, id)

	if cm.verbose {
		fmt.Printf("Deleted capture: %s\n", capture.Name)
	}

	return cm.saveMetadata()
}

// LoadHAR loads HAR data from a capture
func (cm *CaptureManager) LoadHAR(id string) (*har.HAR, error) {
	capture, exists := cm.captures[id]
	if !exists {
		return nil, fmt.Errorf("capture not found: %s", id)
	}

	return cm.loadHARFile(capture.FilePath)
}

// generateCaptureID generates a unique ID for a capture
func (cm *CaptureManager) generateCaptureID(name string) string {
	timestamp := time.Now().Format("20060102-150405")
	hash := md5.Sum([]byte(name + timestamp))
	return fmt.Sprintf("%s-%x", timestamp, hash[:4])
}

// loadMetadata loads capture metadata from disk
func (cm *CaptureManager) loadMetadata() error {
	data, err := os.ReadFile(cm.metadataFile)
	if err != nil {
		return err
	}

	var captures map[string]*CaptureMetadata
	if err := json.Unmarshal(data, &captures); err != nil {
		return errors.Wrap(err, "unmarshaling metadata")
	}

	cm.captures = captures
	return nil
}

// saveMetadata saves capture metadata to disk
func (cm *CaptureManager) saveMetadata() error {
	data, err := json.MarshalIndent(cm.captures, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling metadata")
	}

	if err := os.WriteFile(cm.metadataFile, data, 0644); err != nil {
		return errors.Wrap(err, "writing metadata file")
	}

	return nil
}

// writeHARFile writes HAR data to a file
func (cm *CaptureManager) writeHARFile(filename string, harData *har.HAR) error {
	data, err := json.MarshalIndent(harData, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling HAR data")
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return errors.Wrap(err, "writing HAR file")
	}

	return nil
}

// loadHARFile loads HAR data from a file
func (cm *CaptureManager) loadHARFile(filename string) (*har.HAR, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "reading HAR file")
	}

	var harData har.HAR
	if err := json.Unmarshal(data, &harData); err != nil {
		return nil, errors.Wrap(err, "unmarshaling HAR data")
	}

	return &harData, nil
}

// updateFileMetadata updates file size and checksum for a capture
func (cm *CaptureManager) updateFileMetadata(capture *CaptureMetadata) error {
	file, err := os.Open(capture.FilePath)
	if err != nil {
		return errors.Wrap(err, "opening HAR file")
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return errors.Wrap(err, "getting file stats")
	}
	capture.Size = stat.Size()

	// Calculate checksum
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return errors.Wrap(err, "calculating checksum")
	}
	capture.Checksum = fmt.Sprintf("%x", hash.Sum(nil))

	return nil
}

// Cleanup removes temporary files and data
func (cm *CaptureManager) Cleanup() error {
	// This could be extended to clean up old captures, temporary files, etc.
	return nil
}
