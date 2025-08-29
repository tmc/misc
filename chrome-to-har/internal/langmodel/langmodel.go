// Package langmodel provides access to Chrome's LanguageModel APIs for on-device inference.
package langmodel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// Availability represents the status of the LanguageModel API
type Availability string

const (
	AvailabilityAvailable    Availability = "available"
	AvailabilityDownloading  Availability = "downloading"
	AvailabilityDownloadable Availability = "downloadable"
)

// InferenceMode controls where the model runs
type InferenceMode string

const (
	InferenceModePreferOnDevice InferenceMode = "prefer_on_device"
	InferenceModeOnlyOnDevice   InferenceMode = "only_on_device"
	InferenceModeOnlyInCloud    InferenceMode = "only_in_cloud"
)

// Model represents a LanguageModel instance
type Model struct {
	ctx context.Context
}

// Options for creating a model
type Options struct {
	InferenceMode InferenceMode `json:"inferenceMode,omitempty"`
	Temperature   float64       `json:"temperature,omitempty"`
	TopK          int           `json:"topK,omitempty"`
}

// CheckAvailability checks if the LanguageModel API is available
func CheckAvailability(ctx context.Context) (Availability, error) {
	// First check if the API exists
	var exists bool
	err := chromedp.Run(ctx, chromedp.Evaluate(`typeof LanguageModel !== 'undefined'`, &exists))
	if err != nil {
		return "", errors.Wrap(err, "checking if LanguageModel exists")
	}

	if !exists {
		return "", errors.New("LanguageModel API not available - requires Chrome Beta v138+ with flags enabled")
	}

	// Use chromedp.ActionFunc to handle async operations properly
	var result string
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var res interface{}
		script := `
			new Promise(async (resolve) => {
				try {
					const availability = await LanguageModel.availability();
					resolve(availability);
				} catch (error) {
					resolve('error: ' + error.message);
				}
			})
		`

		if err := chromedp.Evaluate(script, &res).Do(ctx); err != nil {
			return err
		}

		if str, ok := res.(string); ok {
			result = str
		} else {
			result = fmt.Sprintf("%v", res)
		}

		return nil
	}))

	if err != nil {
		return "", errors.Wrap(err, "checking LanguageModel availability")
	}

	switch result {
	case "available", "downloading", "downloadable":
		return Availability(result), nil
	default:
		if strings.HasPrefix(result, "error:") {
			return "", errors.New("LanguageModel API error: " + result)
		}
		return "", errors.New("unexpected availability result: " + result)
	}
}

// Create creates a new LanguageModel instance
func Create(ctx context.Context, opts *Options) (*Model, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Convert options to JSON
	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling options")
	}

	script := fmt.Sprintf(`
		(async () => {
			if (typeof LanguageModel === 'undefined') {
				throw new Error('LanguageModel API not available');
			}
			try {
				const options = %s;
				await LanguageModel.create(options);
				return 'success';
			} catch (error) {
				throw new Error('Failed to create model: ' + error.message);
			}
		})()
	`, string(optsJSON))

	var result string
	err = chromedp.Run(ctx, chromedp.Evaluate(script, &result))
	if err != nil {
		return nil, errors.Wrap(err, "creating LanguageModel")
	}

	if result != "success" {
		return nil, errors.New("failed to create model: " + result)
	}

	return &Model{ctx: ctx}, nil
}

// Generate generates text using the model
func (m *Model) Generate(prompt string) (string, error) {
	script := fmt.Sprintf(`
		(async () => {
			if (typeof LanguageModel === 'undefined') {
				throw new Error('LanguageModel API not available');
			}
			try {
				const model = await LanguageModel.create();
				const response = await model.generate(%s);
				return response;
			} catch (error) {
				throw new Error('Generation failed: ' + error.message);
			}
		})()
	`, jsonString(prompt))

	var result string
	err := chromedp.Run(m.ctx, chromedp.Evaluate(script, &result))
	if err != nil {
		return "", errors.Wrap(err, "generating text")
	}

	return result, nil
}

// GenerateStream generates text with streaming support
func (m *Model) GenerateStream(prompt string, callback func(string) error) error {
	script := fmt.Sprintf(`
		(async () => {
			if (typeof LanguageModel === 'undefined') {
				throw new Error('LanguageModel API not available');
			}
			try {
				const model = await LanguageModel.create();
				const stream = await model.generateStream(%s);
				const reader = stream.getReader();
				
				const chunks = [];
				while (true) {
					const { done, value } = await reader.read();
					if (done) break;
					chunks.push(value);
				}
				return chunks;
			} catch (error) {
				throw new Error('Stream generation failed: ' + error.message);
			}
		})()
	`, jsonString(prompt))

	var chunks []string
	err := chromedp.Run(m.ctx, chromedp.Evaluate(script, &chunks))
	if err != nil {
		return errors.Wrap(err, "generating stream")
	}

	for _, chunk := range chunks {
		if err := callback(chunk); err != nil {
			return errors.Wrap(err, "processing stream chunk")
		}
	}

	return nil
}

// MultimodalGenerate generates text from text and image input
func (m *Model) MultimodalGenerate(prompt string, imageData []byte, mimeType string) (string, error) {
	// Convert image to base64
	imageBase64 := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Encode(imageData))

	script := fmt.Sprintf(`
		(async () => {
			if (typeof LanguageModel === 'undefined') {
				throw new Error('LanguageModel API not available');
			}
			try {
				const model = await LanguageModel.create();
				
				// Create image blob from base64
				const response = await fetch(%s);
				const blob = await response.blob();
				
				const response = await model.generate({
					text: %s,
					image: blob
				});
				return response;
			} catch (error) {
				throw new Error('Multimodal generation failed: ' + error.message);
			}
		})()
	`, jsonString(imageBase64), jsonString(prompt))

	var result string
	err := chromedp.Run(m.ctx, chromedp.Evaluate(script, &result))
	if err != nil {
		return "", errors.Wrap(err, "generating multimodal text")
	}

	return result, nil
}

// WaitForDownload waits for model download to complete
func WaitForDownload(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	script := `
		(async () => {
			if (typeof LanguageModel === 'undefined') {
				throw new Error('LanguageModel API not available');
			}
			
			while (true) {
				const status = await LanguageModel.availability();
				if (status === 'available') {
					return 'ready';
				} else if (status === 'downloading') {
					await new Promise(resolve => setTimeout(resolve, 5000)); // Wait 5 seconds
				} else {
					throw new Error('Unexpected status: ' + status);
				}
			}
		})()
	`

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			var result string
			err := chromedp.Run(ctx, chromedp.Evaluate(script, &result))
			if err != nil {
				return errors.Wrap(err, "waiting for download")
			}
			if result == "ready" {
				return nil
			}
		}
	}
}

// Helper functions
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
