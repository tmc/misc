package visual

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// BaselineManager handles baseline image management
type BaselineManager struct {
	verbose bool
}

// NewBaselineManager creates a new baseline manager
func NewBaselineManager(verbose bool) *BaselineManager {
	return &BaselineManager{
		verbose: verbose,
	}
}

// SaveBaseline saves a baseline image with metadata
func (bm *BaselineManager) SaveBaseline(testName string, screenshot []byte, metadata *BaselineMetadata, config *VisualTestConfig) error {
	if err := bm.ensureDirectoryExists(config.BaselineDir); err != nil {
		return errors.Wrap(err, "ensuring baseline directory exists")
	}

	// Generate file paths
	imageFile := bm.getBaselineImagePath(testName, config)
	metadataFile := bm.getBaselineMetadataPath(testName, config)

	// Calculate checksums
	if metadata != nil {
		metadata.ChecksumMD5 = fmt.Sprintf("%x", md5.Sum(screenshot))
		metadata.ChecksumSHA256 = fmt.Sprintf("%x", sha256.Sum256(screenshot))
		metadata.FileSize = int64(len(screenshot))
		metadata.UpdatedAt = time.Now()
		
		// Get image dimensions
		if dims, err := GetScreenshotDimensions(screenshot); err == nil {
			metadata.ImageDimensions = *dims
		}
	}

	// Save the image
	if err := bm.saveFile(imageFile, screenshot); err != nil {
		return errors.Wrap(err, "saving baseline image")
	}

	// Save metadata if provided
	if metadata != nil {
		if err := bm.saveMetadata(metadataFile, metadata); err != nil {
			// Clean up image file if metadata save fails
			os.Remove(imageFile)
			return errors.Wrap(err, "saving baseline metadata")
		}
	}

	if bm.verbose {
		fmt.Printf("Saved baseline: %s\n", imageFile)
	}

	return nil
}

// LoadBaseline loads a baseline image and its metadata
func (bm *BaselineManager) LoadBaseline(testName string, config *VisualTestConfig) ([]byte, *BaselineMetadata, error) {
	imageFile := bm.getBaselineImagePath(testName, config)
	metadataFile := bm.getBaselineMetadataPath(testName, config)

	// Load the image
	screenshot, err := bm.loadFile(imageFile)
	if err != nil {
		return nil, nil, errors.Wrap(err, "loading baseline image")
	}

	// Load metadata if it exists
	var metadata *BaselineMetadata
	if bm.fileExists(metadataFile) {
		metadata, err = bm.loadMetadata(metadataFile)
		if err != nil {
			if bm.verbose {
				fmt.Printf("Warning: failed to load metadata for %s: %v\n", testName, err)
			}
			// Continue without metadata rather than failing
		}
	}

	return screenshot, metadata, nil
}

// GetBaselineMetadata gets metadata for a baseline
func (bm *BaselineManager) GetBaselineMetadata(testName string, config *VisualTestConfig) (*BaselineMetadata, error) {
	metadataFile := bm.getBaselineMetadataPath(testName, config)
	
	if !bm.fileExists(metadataFile) {
		return nil, errors.New("baseline metadata not found")
	}

	return bm.loadMetadata(metadataFile)
}

// UpdateBaseline updates an existing baseline
func (bm *BaselineManager) UpdateBaseline(testName string, screenshot []byte, metadata *BaselineMetadata, config *VisualTestConfig) error {
	// Check if baseline exists
	if !bm.BaselineExists(testName, config) {
		return errors.New("baseline does not exist")
	}

	// Load existing metadata to preserve history
	existingMetadata, err := bm.GetBaselineMetadata(testName, config)
	if err == nil && existingMetadata != nil && metadata != nil {
		// Preserve creation time
		metadata.CreatedAt = existingMetadata.CreatedAt
		// Increment version
		metadata.Version = bm.incrementVersion(existingMetadata.Version)
	}

	return bm.SaveBaseline(testName, screenshot, metadata, config)
}

// DeleteBaseline deletes a baseline and its metadata
func (bm *BaselineManager) DeleteBaseline(testName string, config *VisualTestConfig) error {
	imageFile := bm.getBaselineImagePath(testName, config)
	metadataFile := bm.getBaselineMetadataPath(testName, config)

	// Delete image file
	if bm.fileExists(imageFile) {
		if err := os.Remove(imageFile); err != nil {
			return errors.Wrap(err, "deleting baseline image")
		}
	}

	// Delete metadata file
	if bm.fileExists(metadataFile) {
		if err := os.Remove(metadataFile); err != nil {
			return errors.Wrap(err, "deleting baseline metadata")
		}
	}

	if bm.verbose {
		fmt.Printf("Deleted baseline: %s\n", testName)
	}

	return nil
}

// BaselineExists checks if a baseline exists
func (bm *BaselineManager) BaselineExists(testName string, config *VisualTestConfig) bool {
	imageFile := bm.getBaselineImagePath(testName, config)
	return bm.fileExists(imageFile)
}

// ListBaselines lists all available baselines
func (bm *BaselineManager) ListBaselines(config *VisualTestConfig) ([]string, error) {
	if !bm.fileExists(config.BaselineDir) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(config.BaselineDir)
	if err != nil {
		return nil, errors.Wrap(err, "reading baseline directory")
	}

	var baselines []string
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") {
				// Remove extension to get test name
				testName := strings.TrimSuffix(name, filepath.Ext(name))
				baselines = append(baselines, testName)
			}
		}
	}

	sort.Strings(baselines)
	return baselines, nil
}

// GetBaselineInfo gets detailed information about all baselines
func (bm *BaselineManager) GetBaselineInfo(config *VisualTestConfig) ([]*BaselineInfo, error) {
	baselines, err := bm.ListBaselines(config)
	if err != nil {
		return nil, errors.Wrap(err, "listing baselines")
	}

	var info []*BaselineInfo
	for _, testName := range baselines {
		baselineInfo := &BaselineInfo{
			TestName: testName,
		}

		// Get image file info
		imageFile := bm.getBaselineImagePath(testName, config)
		if stat, err := os.Stat(imageFile); err == nil {
			baselineInfo.ImageFile = imageFile
			baselineInfo.ImageSize = stat.Size()
			baselineInfo.ImageModTime = stat.ModTime()
		}

		// Get metadata if available
		metadata, err := bm.GetBaselineMetadata(testName, config)
		if err == nil {
			baselineInfo.Metadata = metadata
		}

		info = append(info, baselineInfo)
	}

	return info, nil
}

// CleanupBaselines removes old or unused baselines
func (bm *BaselineManager) CleanupBaselines(config *VisualTestConfig, olderThan time.Time) error {
	baselines, err := bm.ListBaselines(config)
	if err != nil {
		return errors.Wrap(err, "listing baselines")
	}

	cleaned := 0
	for _, testName := range baselines {
		imageFile := bm.getBaselineImagePath(testName, config)
		
		if stat, err := os.Stat(imageFile); err == nil {
			if stat.ModTime().Before(olderThan) {
				if err := bm.DeleteBaseline(testName, config); err != nil {
					if bm.verbose {
						fmt.Printf("Warning: failed to delete baseline %s: %v\n", testName, err)
					}
				} else {
					cleaned++
				}
			}
		}
	}

	if bm.verbose {
		fmt.Printf("Cleaned up %d baselines\n", cleaned)
	}

	return nil
}

// ExportBaselines exports baselines to a tar archive
func (bm *BaselineManager) ExportBaselines(config *VisualTestConfig, archivePath string) error {
	// This would implement tar/zip export functionality
	// For now, return not implemented
	return errors.New("export functionality not implemented yet")
}

// ImportBaselines imports baselines from a tar archive
func (bm *BaselineManager) ImportBaselines(config *VisualTestConfig, archivePath string) error {
	// This would implement tar/zip import functionality
	// For now, return not implemented
	return errors.New("import functionality not implemented yet")
}

// CompareBaselines compares two baselines
func (bm *BaselineManager) CompareBaselines(testName1, testName2 string, config *VisualTestConfig) (*BaselineComparison, error) {
	// Load both baselines
	_, metadata1, err := bm.LoadBaseline(testName1, config)
	if err != nil {
		return nil, errors.Wrapf(err, "loading baseline %s", testName1)
	}

	_, metadata2, err := bm.LoadBaseline(testName2, config)
	if err != nil {
		return nil, errors.Wrapf(err, "loading baseline %s", testName2)
	}

	// Compare checksums for quick comparison
	identical := false
	if metadata1 != nil && metadata2 != nil {
		identical = metadata1.ChecksumSHA256 == metadata2.ChecksumSHA256
	}

	comparison := &BaselineComparison{
		TestName1: testName1,
		TestName2: testName2,
		Identical: identical,
		Metadata1: metadata1,
		Metadata2: metadata2,
	}

	// If not identical, calculate difference
	if !identical {
		// This would use the image comparison engine
		// For now, just set basic info
		comparison.DifferencePercentage = 0.0 // Would be calculated by comparison engine
	}

	return comparison, nil
}

// ValidateBaseline validates a baseline image and metadata
func (bm *BaselineManager) ValidateBaseline(testName string, config *VisualTestConfig) (*BaselineValidation, error) {
	validation := &BaselineValidation{
		TestName: testName,
		Valid:    true,
		Issues:   []string{},
	}

	// Check if image exists
	imageFile := bm.getBaselineImagePath(testName, config)
	if !bm.fileExists(imageFile) {
		validation.Valid = false
		validation.Issues = append(validation.Issues, "baseline image not found")
		return validation, nil
	}

	// Check if image is readable
	screenshot, err := bm.loadFile(imageFile)
	if err != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, fmt.Sprintf("cannot read baseline image: %v", err))
		return validation, nil
	}

	// Check image dimensions
	if dims, err := GetScreenshotDimensions(screenshot); err != nil {
		validation.Valid = false
		validation.Issues = append(validation.Issues, fmt.Sprintf("cannot determine image dimensions: %v", err))
	} else {
		validation.ImageDimensions = dims
	}

	// Check metadata if it exists
	metadataFile := bm.getBaselineMetadataPath(testName, config)
	if bm.fileExists(metadataFile) {
		metadata, err := bm.loadMetadata(metadataFile)
		if err != nil {
			validation.Issues = append(validation.Issues, fmt.Sprintf("cannot read metadata: %v", err))
		} else {
			validation.Metadata = metadata
			
			// Validate checksums
			actualMD5 := fmt.Sprintf("%x", md5.Sum(screenshot))
			actualSHA256 := fmt.Sprintf("%x", sha256.Sum256(screenshot))
			
			if metadata.ChecksumMD5 != "" && metadata.ChecksumMD5 != actualMD5 {
				validation.Issues = append(validation.Issues, "MD5 checksum mismatch")
			}
			
			if metadata.ChecksumSHA256 != "" && metadata.ChecksumSHA256 != actualSHA256 {
				validation.Issues = append(validation.Issues, "SHA256 checksum mismatch")
			}
		}
	}

	if len(validation.Issues) > 0 {
		validation.Valid = false
	}

	return validation, nil
}

// Helper methods

func (bm *BaselineManager) getBaselineImagePath(testName string, config *VisualTestConfig) string {
	filename := fmt.Sprintf("%s.%s", testName, config.Format)
	return filepath.Join(config.BaselineDir, filename)
}

func (bm *BaselineManager) getBaselineMetadataPath(testName string, config *VisualTestConfig) string {
	filename := fmt.Sprintf("%s.json", testName)
	return filepath.Join(config.BaselineDir, filename)
}

func (bm *BaselineManager) ensureDirectoryExists(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func (bm *BaselineManager) fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (bm *BaselineManager) saveFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func (bm *BaselineManager) loadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (bm *BaselineManager) saveMetadata(path string, metadata *BaselineMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling metadata")
	}
	return bm.saveFile(path, data)
}

func (bm *BaselineManager) loadMetadata(path string) (*BaselineMetadata, error) {
	data, err := bm.loadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "loading metadata file")
	}

	var metadata BaselineMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, errors.Wrap(err, "unmarshaling metadata")
	}

	return &metadata, nil
}

func (bm *BaselineManager) incrementVersion(version string) string {
	if version == "" {
		return "1.0.0"
	}
	
	// Simple version increment (would be more sophisticated in real implementation)
	return version + ".1"
}

// Supporting types

// BaselineInfo contains information about a baseline
type BaselineInfo struct {
	TestName      string
	ImageFile     string
	ImageSize     int64
	ImageModTime  time.Time
	Metadata      *BaselineMetadata
}

// BaselineComparison contains the result of comparing two baselines
type BaselineComparison struct {
	TestName1            string
	TestName2            string
	Identical            bool
	DifferencePercentage float64
	Metadata1            *BaselineMetadata
	Metadata2            *BaselineMetadata
}

// BaselineValidation contains the result of validating a baseline
type BaselineValidation struct {
	TestName        string
	Valid           bool
	Issues          []string
	ImageDimensions *ImageDimensions
	Metadata        *BaselineMetadata
}