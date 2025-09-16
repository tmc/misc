package visual

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"github.com/pkg/errors"
)

// ImageComparison handles image comparison operations
type ImageComparison struct {
	verbose bool
}

// NewImageComparison creates a new image comparison instance
func NewImageComparison(verbose bool) *ImageComparison {
	return &ImageComparison{
		verbose: verbose,
	}
}

// CompareImages compares two images and returns detailed comparison results
func (ic *ImageComparison) CompareImages(baseline, actual image.Image, config *VisualTestConfig) (*ComparisonResult, error) {
	if baseline == nil || actual == nil {
		return nil, errors.New("baseline and actual images cannot be nil")
	}

	// Check if images have the same dimensions
	baselineBounds := baseline.Bounds()
	actualBounds := actual.Bounds()

	if !baselineBounds.Eq(actualBounds) {
		return &ComparisonResult{
			Passed:               false,
			DiffPixels:           0,
			TotalPixels:          0,
			DifferencePercentage: 100.0,
			MatchingScore:        0.0,
			Metadata: map[string]interface{}{
				"error":         "Image dimensions do not match",
				"baseline_size": fmt.Sprintf("%dx%d", baselineBounds.Dx(), baselineBounds.Dy()),
				"actual_size":   fmt.Sprintf("%dx%d", actualBounds.Dx(), actualBounds.Dy()),
			},
		}, nil
	}

	width := baselineBounds.Dx()
	height := baselineBounds.Dy()
	totalPixels := width * height

	// Create diff image
	diffImage := image.NewRGBA(baselineBounds)

	// Comparison statistics
	var diffPixels int
	var totalColorDiff float64
	var regions []DiffRegion

	// Pixel-by-pixel comparison
	for y := baselineBounds.Min.Y; y < baselineBounds.Max.Y; y++ {
		for x := baselineBounds.Min.X; x < baselineBounds.Max.X; x++ {
			baselineColor := baseline.At(x, y)
			actualColor := actual.At(x, y)

			// Calculate color difference
			diff := ic.calculateColorDifference(baselineColor, actualColor)
			totalColorDiff += diff

			// Determine if pixel is different based on threshold
			isDifferent := diff > config.Threshold

			if config.EnableFuzzyMatching {
				isDifferent = diff > config.FuzzyThreshold
			}

			if config.IgnoreAntialiasing {
				isDifferent = isDifferent && !ic.isAntialiasedPixel(baseline, actual, x, y)
			}

			if isDifferent {
				diffPixels++
				// Highlight difference in diff image
				diffImage.Set(x, y, color.RGBA{255, 0, 0, 255}) // Red for differences
			} else {
				// Keep original color but make it slightly transparent
				r, g, b, _ := actualColor.RGBA()
				diffImage.Set(x, y, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 128})
			}
		}
	}

	// Calculate overall statistics
	differencePercentage := (float64(diffPixels) / float64(totalPixels)) * 100
	matchingScore := 1.0 - (differencePercentage / 100.0)

	// Find difference regions
	if diffPixels > 0 {
		regions = ic.findDiffRegions(diffImage, baselineBounds)
	}

	// Determine if comparison passed
	passed := diffPixels <= config.MaxDiffPixels && differencePercentage <= (config.Threshold*100)

	if ic.verbose {
		fmt.Printf("Image comparison: %d different pixels out of %d total (%.2f%%), score: %.2f\n",
			diffPixels, totalPixels, differencePercentage, matchingScore)
	}

	return &ComparisonResult{
		Passed:               passed,
		DiffPixels:           diffPixels,
		TotalPixels:          totalPixels,
		DifferencePercentage: differencePercentage,
		DiffImage:            diffImage,
		MatchingScore:        matchingScore,
		Regions:              regions,
		Metadata: map[string]interface{}{
			"threshold":           config.Threshold,
			"fuzzy_matching":      config.EnableFuzzyMatching,
			"ignore_antialiasing": config.IgnoreAntialiasing,
			"max_diff_pixels":     config.MaxDiffPixels,
			"average_color_diff":  totalColorDiff / float64(totalPixels),
			"total_color_diff":    totalColorDiff,
		},
	}, nil
}

// CompareScreenshots compares two screenshots given as byte arrays
func (ic *ImageComparison) CompareScreenshots(baselineData, actualData []byte, config *VisualTestConfig) (*ComparisonResult, error) {
	// Decode baseline image
	baseline, _, err := image.Decode(bytes.NewReader(baselineData))
	if err != nil {
		return nil, errors.Wrap(err, "decoding baseline image")
	}

	// Decode actual image
	actual, _, err := image.Decode(bytes.NewReader(actualData))
	if err != nil {
		return nil, errors.Wrap(err, "decoding actual image")
	}

	return ic.CompareImages(baseline, actual, config)
}

// calculateColorDifference calculates the difference between two colors
func (ic *ImageComparison) calculateColorDifference(c1, c2 color.Color) float64 {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	// Convert to 0-255 range
	r1, g1, b1, a1 = r1>>8, g1>>8, b1>>8, a1>>8
	r2, g2, b2, a2 = r2>>8, g2>>8, b2>>8, a2>>8

	// Calculate Euclidean distance in RGBA space
	dr := float64(r1) - float64(r2)
	dg := float64(g1) - float64(g2)
	db := float64(b1) - float64(b2)
	da := float64(a1) - float64(a2)

	// Normalize to 0-1 range
	distance := math.Sqrt(dr*dr+dg*dg+db*db+da*da) / (255 * 2)

	return distance
}

// isAntialiasedPixel detects if a pixel difference is due to antialiasing
func (ic *ImageComparison) isAntialiasedPixel(baseline, actual image.Image, x, y int) bool {
	bounds := baseline.Bounds()

	// Check surrounding pixels for antialiasing patterns
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}

			nx, ny := x+dx, y+dy
			if nx >= bounds.Min.X && nx < bounds.Max.X && ny >= bounds.Min.Y && ny < bounds.Max.Y {
				baselineNeighbor := baseline.At(nx, ny)
				actualCenter := actual.At(x, y)

				// If the actual center pixel matches a neighboring baseline pixel,
				// this might be antialiasing
				if ic.calculateColorDifference(baselineNeighbor, actualCenter) < 0.1 {
					return true
				}
			}
		}
	}

	return false
}

// findDiffRegions finds regions of differences in the diff image
func (ic *ImageComparison) findDiffRegions(diffImage *image.RGBA, bounds image.Rectangle) []DiffRegion {
	var regions []DiffRegion
	visited := make(map[string]bool)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			key := fmt.Sprintf("%d,%d", x, y)
			if visited[key] {
				continue
			}

			// Check if this pixel is different (red)
			r, _, _, _ := diffImage.At(x, y).RGBA()
			if r > 32768 { // Red channel is high
				// Start a new region
				region := ic.expandRegion(diffImage, bounds, x, y, visited)
				if region.Width > 0 && region.Height > 0 {
					regions = append(regions, region)
				}
			}
		}
	}

	return regions
}

// expandRegion expands a difference region using flood fill
func (ic *ImageComparison) expandRegion(diffImage *image.RGBA, bounds image.Rectangle, startX, startY int, visited map[string]bool) DiffRegion {
	minX, minY := startX, startY
	maxX, maxY := startX, startY

	// Simple flood fill to find connected different pixels
	queue := []image.Point{{startX, startY}}

	for len(queue) > 0 {
		point := queue[0]
		queue = queue[1:]

		x, y := point.X, point.Y
		key := fmt.Sprintf("%d,%d", x, y)

		if visited[key] || x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
			continue
		}

		// Check if this pixel is different (red)
		r, _, _, _ := diffImage.At(x, y).RGBA()
		if r <= 32768 { // Not red
			continue
		}

		visited[key] = true

		// Update bounds
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}

		// Add neighbors to queue
		neighbors := []image.Point{
			{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1},
		}

		for _, neighbor := range neighbors {
			queue = append(queue, neighbor)
		}
	}

	return DiffRegion{
		X:      minX,
		Y:      minY,
		Width:  maxX - minX + 1,
		Height: maxY - minY + 1,
		Score:  1.0, // Could be calculated based on intensity of differences
	}
}

// GenerateDiffImage creates a visual diff image highlighting differences
func (ic *ImageComparison) GenerateDiffImage(baseline, actual image.Image, config *VisualTestConfig) (image.Image, error) {
	comparison, err := ic.CompareImages(baseline, actual, config)
	if err != nil {
		return nil, errors.Wrap(err, "comparing images")
	}

	return comparison.DiffImage, nil
}

// GenerateSideBySideImage creates a side-by-side comparison image
func (ic *ImageComparison) GenerateSideBySideImage(baseline, actual image.Image, diffImage image.Image) (image.Image, error) {
	baselineBounds := baseline.Bounds()
	actualBounds := actual.Bounds()
	diffBounds := diffImage.Bounds()

	// Calculate dimensions for side-by-side layout
	width := baselineBounds.Dx() + actualBounds.Dx() + diffBounds.Dx() + 20 // 10px padding between images
	height := baselineBounds.Dy()
	if actualBounds.Dy() > height {
		height = actualBounds.Dy()
	}
	if diffBounds.Dy() > height {
		height = diffBounds.Dy()
	}

	// Create result image
	result := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with white background
	draw.Draw(result, result.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)

	// Draw baseline image
	draw.Draw(result, image.Rect(0, 0, baselineBounds.Dx(), baselineBounds.Dy()), baseline, baselineBounds.Min, draw.Src)

	// Draw actual image
	actualX := baselineBounds.Dx() + 10
	draw.Draw(result, image.Rect(actualX, 0, actualX+actualBounds.Dx(), actualBounds.Dy()), actual, actualBounds.Min, draw.Src)

	// Draw diff image
	diffX := actualX + actualBounds.Dx() + 10
	draw.Draw(result, image.Rect(diffX, 0, diffX+diffBounds.Dx(), diffBounds.Dy()), diffImage, diffBounds.Min, draw.Src)

	return result, nil
}

// GenerateAnnotatedImage creates an annotated image with difference regions highlighted
func (ic *ImageComparison) GenerateAnnotatedImage(actual image.Image, regions []DiffRegion) (image.Image, error) {
	bounds := actual.Bounds()
	result := image.NewRGBA(bounds)

	// Copy actual image
	draw.Draw(result, bounds, actual, bounds.Min, draw.Src)

	// Draw region annotations
	for i, region := range regions {
		// Draw red rectangle around region
		ic.drawRect(result, region.X, region.Y, region.Width, region.Height, color.RGBA{255, 0, 0, 255})

		// Add region number
		// (In a real implementation, this would use proper text rendering)
		// For now, just mark the top-left corner
		result.Set(region.X, region.Y, color.RGBA{255, 255, 0, 255})

		if ic.verbose {
			fmt.Printf("Region %d: (%d,%d) %dx%d, score: %.2f\n", i+1, region.X, region.Y, region.Width, region.Height, region.Score)
		}
	}

	return result, nil
}

// drawRect draws a rectangle outline on the image
func (ic *ImageComparison) drawRect(img *image.RGBA, x, y, width, height int, col color.RGBA) {
	bounds := img.Bounds()

	// Draw top and bottom lines
	for i := x; i < x+width && i < bounds.Max.X; i++ {
		if y >= bounds.Min.Y && y < bounds.Max.Y {
			img.Set(i, y, col)
		}
		if y+height-1 >= bounds.Min.Y && y+height-1 < bounds.Max.Y {
			img.Set(i, y+height-1, col)
		}
	}

	// Draw left and right lines
	for i := y; i < y+height && i < bounds.Max.Y; i++ {
		if x >= bounds.Min.X && x < bounds.Max.X {
			img.Set(x, i, col)
		}
		if x+width-1 >= bounds.Min.X && x+width-1 < bounds.Max.X {
			img.Set(x+width-1, i, col)
		}
	}
}

// SaveDiffImage saves a diff image to a file
func (ic *ImageComparison) SaveDiffImage(diffImage image.Image, filename string) error {
	var buf bytes.Buffer

	if err := png.Encode(&buf, diffImage); err != nil {
		return errors.Wrap(err, "encoding diff image")
	}

	return ic.saveImageFile(filename, buf.Bytes())
}

// saveImageFile saves image data to a file
func (ic *ImageComparison) saveImageFile(filename string, data []byte) error {
	// In a real implementation, this would save to the filesystem
	// For now, just return success
	if ic.verbose {
		fmt.Printf("Would save diff image to: %s (%d bytes)\n", filename, len(data))
	}
	return nil
}

// CalculateStructuralSimilarity calculates structural similarity between images
func (ic *ImageComparison) CalculateStructuralSimilarity(baseline, actual image.Image) (float64, error) {
	// This would implement SSIM (Structural Similarity Index)
	// For now, return a simple correlation-based similarity

	baselineBounds := baseline.Bounds()
	actualBounds := actual.Bounds()

	if !baselineBounds.Eq(actualBounds) {
		return 0.0, errors.New("images must have the same dimensions")
	}

	var correlation float64
	var baselineSum, actualSum float64
	totalPixels := float64(baselineBounds.Dx() * baselineBounds.Dy())

	// Calculate means
	for y := baselineBounds.Min.Y; y < baselineBounds.Max.Y; y++ {
		for x := baselineBounds.Min.X; x < baselineBounds.Max.X; x++ {
			baselineGray := ic.rgbaToGray(baseline.At(x, y))
			actualGray := ic.rgbaToGray(actual.At(x, y))

			baselineSum += baselineGray
			actualSum += actualGray
		}
	}

	baselineMean := baselineSum / totalPixels
	actualMean := actualSum / totalPixels

	// Calculate correlation
	var numerator, baselineVariance, actualVariance float64

	for y := baselineBounds.Min.Y; y < baselineBounds.Max.Y; y++ {
		for x := baselineBounds.Min.X; x < baselineBounds.Max.X; x++ {
			baselineGray := ic.rgbaToGray(baseline.At(x, y))
			actualGray := ic.rgbaToGray(actual.At(x, y))

			baselineDiff := baselineGray - baselineMean
			actualDiff := actualGray - actualMean

			numerator += baselineDiff * actualDiff
			baselineVariance += baselineDiff * baselineDiff
			actualVariance += actualDiff * actualDiff
		}
	}

	denominator := math.Sqrt(baselineVariance * actualVariance)
	if denominator == 0 {
		return 1.0, nil // Images are identical
	}

	correlation = numerator / denominator

	// Convert correlation to similarity score (0-1)
	similarity := (correlation + 1.0) / 2.0

	return similarity, nil
}

// rgbaToGray converts RGBA color to grayscale
func (ic *ImageComparison) rgbaToGray(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	// Convert to 0-255 range and apply standard grayscale formula
	gray := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
	return gray / 255.0
}

// GenerateHistogram generates a color histogram for an image
func (ic *ImageComparison) GenerateHistogram(img image.Image) map[string]int {
	histogram := make(map[string]int)
	bounds := img.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Convert to 0-255 range and create color key
			colorKey := fmt.Sprintf("%d,%d,%d", r>>8, g>>8, b>>8)
			histogram[colorKey]++
		}
	}

	return histogram
}

// CompareHistograms compares color histograms of two images
func (ic *ImageComparison) CompareHistograms(baseline, actual image.Image) (float64, error) {
	baselineHist := ic.GenerateHistogram(baseline)
	actualHist := ic.GenerateHistogram(actual)

	// Calculate histogram intersection
	var intersection, union int
	allColors := make(map[string]bool)

	for color := range baselineHist {
		allColors[color] = true
	}
	for color := range actualHist {
		allColors[color] = true
	}

	for color := range allColors {
		baselineCount := baselineHist[color]
		actualCount := actualHist[color]

		intersection += int(math.Min(float64(baselineCount), float64(actualCount)))
		union += int(math.Max(float64(baselineCount), float64(actualCount)))
	}

	if union == 0 {
		return 1.0, nil
	}

	return float64(intersection) / float64(union), nil
}

// PerformAdvancedComparison performs advanced image comparison with multiple metrics
func (ic *ImageComparison) PerformAdvancedComparison(baseline, actual image.Image, config *VisualTestConfig) (*AdvancedComparisonResult, error) {
	// Basic pixel comparison
	pixelComparison, err := ic.CompareImages(baseline, actual, config)
	if err != nil {
		return nil, errors.Wrap(err, "performing pixel comparison")
	}

	// Structural similarity
	structuralSimilarity, err := ic.CalculateStructuralSimilarity(baseline, actual)
	if err != nil {
		return nil, errors.Wrap(err, "calculating structural similarity")
	}

	// Histogram comparison
	histogramSimilarity, err := ic.CompareHistograms(baseline, actual)
	if err != nil {
		return nil, errors.Wrap(err, "comparing histograms")
	}

	// Calculate overall score
	overallScore := (pixelComparison.MatchingScore + structuralSimilarity + histogramSimilarity) / 3.0

	return &AdvancedComparisonResult{
		PixelComparison:      pixelComparison,
		StructuralSimilarity: structuralSimilarity,
		HistogramSimilarity:  histogramSimilarity,
		OverallScore:         overallScore,
		Passed:               overallScore >= (1.0 - config.Threshold),
	}, nil
}

// AdvancedComparisonResult contains results from advanced comparison
type AdvancedComparisonResult struct {
	PixelComparison      *ComparisonResult
	StructuralSimilarity float64
	HistogramSimilarity  float64
	OverallScore         float64
	Passed               bool
}
