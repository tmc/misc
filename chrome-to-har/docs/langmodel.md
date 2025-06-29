# LanguageModel API Integration

This document describes how to use Chrome's on-device LanguageModel API with chromedp/golang.

## Requirements

- Chrome Beta v138 or higher
- Chrome flags enabled:
  - `chrome://flags/#optimization-guide-on-device-model`
  - `chrome://flags/#prompt-api-for-gemini-nano`
  - `chrome://flags/#prompt-api-for-gemini-nano-multimodal-input`

## Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/langmodel"
)

func main() {
    ctx := context.Background()

    // Create browser with LanguageModel API flags
    b, err := browser.New(ctx, nil,
        browser.WithHeadless(false), // Must be visible
        browser.WithChromeFlags([]string{
            "enable-features=Gemini,AILanguageModelService",
            "enable-ai-language-model-service",
            "optimization-guide-on-device-model=enabled",
            "prompt-api-for-gemini-nano=enabled",
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer b.Close()

    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }

    if err := b.Navigate("about:blank"); err != nil {
        log.Fatal(err)
    }

    // Check availability
    availability, err := langmodel.CheckAvailability(b.Context())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Status: %s\n", availability)

    if availability == langmodel.AvailabilityAvailable {
        // Create model and generate
        model, err := langmodel.Create(b.Context(), &langmodel.Options{
            InferenceMode: langmodel.InferenceModeOnlyOnDevice,
            Temperature:   0.7,
        })
        if err != nil {
            log.Fatal(err)
        }

        response, err := model.Generate("Explain machine learning briefly")
        if err != nil {
            log.Fatal(err)
        }

        fmt.Println(response)
    }
}
```

## Command Line Example

```bash
# Check availability only
go run ./cmd/langmodel-example -check

# Generate with custom prompt
go run ./cmd/langmodel-example -prompt "What is Go programming language?"

# Wait for model download if needed
go run ./cmd/langmodel-example -wait -prompt "Hello world"
```

## API Reference

### Types

- `Availability`: "available", "downloading", "downloadable"
- `InferenceMode`: "prefer_on_device", "only_on_device", "only_in_cloud"

### Functions

- `CheckAvailability(ctx)`: Check if API is available
- `Create(ctx, opts)`: Create a model instance
- `WaitForDownload(ctx, timeout)`: Wait for model download
- `model.Generate(prompt)`: Generate text from prompt
- `model.GenerateStream(prompt, callback)`: Stream generation
- `model.MultimodalGenerate(prompt, image, mimeType)`: Text + image input

## Limitations

- Chrome Desktop only
- Maximum 6000 tokens for on-device inference
- Requires non-headless mode
- Beta API - subject to change