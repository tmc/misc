package llm

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

type StorageConfig struct {
    Directory       string
    Format          string
    IncludeMetadata bool
    IncludeTokens   bool
    IncludeTimings  bool
}

type Storage struct {
    config StorageConfig
}

func NewStorage(config StorageConfig) *Storage {
    return &Storage{config: config}
}

func (s *Storage) SaveMetrics(params interface{}) error {
    if s.config.Directory == "" {
        return nil // Skip if no directory configured
    }

    if err := os.MkdirAll(s.config.Directory, 0755); err != nil {
        return fmt.Errorf("create metrics directory: %w", err)
    }

    output := struct {
        Timestamp  time.Time   `json:"timestamp"`
        RequestID  string      `json:"requestId"`
        Parameters interface{} `json:"parameters,omitempty"`
    }{
        Timestamp:  time.Now(),
        RequestID:  fmt.Sprintf("req_%d", time.Now().UnixNano()),
        Parameters: params,
    }

    filename := fmt.Sprintf("metrics_%d.json", time.Now().UnixNano())
    filepath := filepath.Join(s.config.Directory, filename)

    f, err := os.Create(filepath)
    if err != nil {
        return fmt.Errorf("create metrics file: %w", err)
    }
    defer f.Close()

    encoder := json.NewEncoder(f)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(output); err != nil {
        return fmt.Errorf("encode metrics: %w", err)
    }

    return nil
}

