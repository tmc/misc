package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// UnifiedConfig represents a configuration structure that can be shared across
// all eslog components (CLI, TUI, and OpenTelemetry export)
type UnifiedConfig struct {
	// Core settings
	DefaultRootPID int    `json:"default_root_pid"`
	MaxArgsLength  int    `json:"max_args_length"`
	DefaultFormat  string `json:"default_format"`

	// Filter defaults
	DefaultPIDFilter  int    `json:"default_pid_filter"`
	DefaultNameFilter string `json:"default_name_filter"`
	DefaultEventType  int    `json:"default_event_type"`
	DefaultTTYFilter  string `json:"default_tty_filter"`

	// Command extractors
	CommandExtractors []CommandExtractor `json:"command_extractors"`

	// UI settings
	TUI TUIConfig `json:"tui"`

	// OpenTelemetry settings
	OpenTelemetry OTelConfig `json:"opentelemetry"`
}

// TUIConfig contains all Terminal UI specific settings
type TUIConfig struct {
	ColorScheme            string            `json:"color_scheme"`
	DefaultExpandLevel     int               `json:"default_expand_level"`
	ShowTooltips           bool              `json:"show_tooltips"`
	FileOperationIcons     bool              `json:"file_operation_icons"`
	CustomColors           map[string]string `json:"custom_colors"`
	AutoExpandFilteredTree bool              `json:"auto_expand_filtered_tree"`
}

// OTelConfig contains all OpenTelemetry export related settings
type OTelConfig struct {
	DefaultServiceName     string `json:"default_service_name"`
	DefaultExporter        string `json:"default_exporter"`
	DefaultEndpoint        string `json:"default_endpoint"`
	SkipStatsEvents        bool   `json:"skip_stats_events"`
	SkipLookupEvents       bool   `json:"skip_lookup_events"`
	BatchSize              int    `json:"batch_size"`
	CreateRootSpan         bool   `json:"create_root_span"`
	DefaultRootSpanName    string `json:"default_root_span_name"`
	RespectExistingContext bool   `json:"respect_existing_context"`
	// Metrics settings
	UseMetrics            bool          `json:"use_metrics"`
	AggregateIO           bool          `json:"aggregate_io"`
	MetricsTemporality    string        `json:"metrics_temporality"` // delta or cumulative
	MetricsExportInterval string        `json:"metrics_export_interval"` // duration format like "5s"
}

// CommandExtractor defines a pattern for extracting commands from process arguments
type CommandExtractor struct {
	Pattern     string `json:"pattern"`
	Group       int    `json:"group"`
	DisplayName string `json:"display_name"`
}

// DefaultConfig returns the default configuration 
func DefaultUnifiedConfig() *UnifiedConfig {
	return &UnifiedConfig{
		DefaultRootPID: 0,
		MaxArgsLength:  120,
		DefaultFormat:  "default",

		DefaultPIDFilter:  0,
		DefaultNameFilter: "",
		DefaultEventType:  0,
		DefaultTTYFilter:  "",

		CommandExtractors: []CommandExtractor{
			{
				Pattern:     "source\\s+.*\\s+&&\\s+eval\\s+'([^']+)'",
				Group:       1,
				DisplayName: "EVAL:",
			},
			{
				Pattern:     "which\\s+(\\S+)",
				Group:       1,
				DisplayName: "WHICH:",
			},
			{
				Pattern:     "source\\s+([^\\s;]+)",
				Group:       1,
				DisplayName: "SOURCE:",
			},
			{
				Pattern:     "go\\s+test\\s+(.+)",
				Group:       1,
				DisplayName: "GO TEST:",
			},
			{
				Pattern:     "SHELL:\\s+(.+)",
				Group:       1,
				DisplayName: "SHELL:",
			},
		},

		TUI: TUIConfig{
			ColorScheme:            "default",
			DefaultExpandLevel:     2,
			ShowTooltips:           true,
			FileOperationIcons:     true,
			AutoExpandFilteredTree: true,
			CustomColors: map[string]string{
				"active":    "#16A085",
				"completed": "#888888",
				"error":     "#E74C3C",
				"selected":  "#2C3E50",
				"header":    "#FFFFFF",
				"tooltip":   "#F39C12",
			},
		},

		OpenTelemetry: OTelConfig{
			DefaultServiceName:     "eslog",
			DefaultExporter:        "stdout",
			DefaultEndpoint:        "localhost:4317",
			SkipStatsEvents:        true,
			SkipLookupEvents:       true,
			BatchSize:              100,
			CreateRootSpan:         true,
			DefaultRootSpanName:    "eslog-session",
			RespectExistingContext: true,
		},
	}
}

// LoadUnifiedConfig loads a configuration from a file
func LoadUnifiedConfig(configPath string) (*UnifiedConfig, error) {
	// Start with default configuration
	config := DefaultUnifiedConfig()

	// Read and parse the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Unmarshal into our config structure
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return config, nil
}

// SaveUnifiedConfig saves the configuration to a file
func SaveUnifiedConfig(config *UnifiedConfig, configPath string) error {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Marshal the configuration to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	// Write to the file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// FindAndLoadUnifiedConfig tries to find and load a configuration file
// from various locations in order of preference.
func FindAndLoadUnifiedConfig() (*UnifiedConfig, string, error) {
	// Default to the base config
	config := DefaultUnifiedConfig()
	
	// Try to find configuration files in order of preference
	configPaths := []string{
		"./.eslogrc.json",                      // Current directory
		filepath.Join(os.Getenv("HOME"), ".eslogrc.json"), // Home directory
		filepath.Join(os.Getenv("HOME"), ".config/eslog/config.json"), // XDG config directory
	}

	var loadedPath string
	
	// Try each path
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			cfg, err := LoadUnifiedConfig(path)
			if err == nil {
				config = cfg
				loadedPath = path
				break
			}
		}
	}

	return config, loadedPath, nil
}

// ApplyConfig applies the configuration to the provided command-line flags
// This is useful for setting defaults before flag parsing
func (config *UnifiedConfig) ApplyToFlags(
	rootPID *int,
	maxArgsLen *int,
	formatStr *string,
	filterPID *int,
	filterName *string,
	filterEvent *int,
	filterTTY *string,
	// Include other flags as needed
) {
	// Only set non-zero values to avoid overriding explicit flag values
	if config.DefaultRootPID != 0 && *rootPID == 0 {
		*rootPID = config.DefaultRootPID
	}
	
	if config.MaxArgsLength != 0 && *maxArgsLen == 120 { // 120 is the default
		*maxArgsLen = config.MaxArgsLength
	}
	
	if config.DefaultFormat != "" && *formatStr == "default" {
		*formatStr = config.DefaultFormat
	}
	
	if config.DefaultPIDFilter != 0 && *filterPID == 0 {
		*filterPID = config.DefaultPIDFilter
	}
	
	if config.DefaultNameFilter != "" && *filterName == "" {
		*filterName = config.DefaultNameFilter
	}
	
	if config.DefaultEventType != 0 && *filterEvent == 0 {
		*filterEvent = config.DefaultEventType
	}
	
	if config.DefaultTTYFilter != "" && *filterTTY == "" {
		*filterTTY = config.DefaultTTYFilter
	}
}

// ApplyOTelConfig applies OpenTelemetry configuration to the provided flags
func (config *UnifiedConfig) ApplyOTelConfig(
	serviceName *string,
	exporter *string,
	endpoint *string,
	skipStats *bool,
	skipLookups *bool,
	batchSize *int,
	createRootSpan *bool,
	rootSpanName *string,
	respectTraceparent *bool,
	// New metrics parameters
	useMetrics *bool,
	aggregateIO *bool,
	temporality *string,
	exportInterval *string,
) {
	otel := config.OpenTelemetry

	// Apply OpenTelemetry settings
	if otel.DefaultServiceName != "" && *serviceName == "eslog" {
		*serviceName = otel.DefaultServiceName
	}

	if otel.DefaultExporter != "" && *exporter == "stdout" {
		*exporter = otel.DefaultExporter
	}

	if otel.DefaultEndpoint != "" && *endpoint == "localhost:4317" {
		*endpoint = otel.DefaultEndpoint
	}

	// These are boolean flags, we want to respect the configured value
	*skipStats = otel.SkipStatsEvents
	*skipLookups = otel.SkipLookupEvents

	if otel.BatchSize != 0 && *batchSize == 100 {
		*batchSize = otel.BatchSize
	}

	*createRootSpan = otel.CreateRootSpan

	if otel.DefaultRootSpanName != "" && *rootSpanName == "eslog-session" {
		*rootSpanName = otel.DefaultRootSpanName
	}

	*respectTraceparent = otel.RespectExistingContext

	// Apply metrics settings if provided
	if useMetrics != nil {
		*useMetrics = otel.UseMetrics
	}

	if aggregateIO != nil {
		*aggregateIO = otel.AggregateIO
	}

	if temporality != nil && otel.MetricsTemporality != "" && *temporality == "delta" {
		*temporality = otel.MetricsTemporality
	}

	if exportInterval != nil && otel.MetricsExportInterval != "" && *exportInterval == "5s" {
		*exportInterval = otel.MetricsExportInterval
	}
}

// ApplyTUIConfig applies TUI configuration to the appropriate variables
func (config *UnifiedConfig) ApplyTUIConfig(
	showTooltips *bool,
	defaultExpandLevel *int,
	useIcons *bool,
	// Other TUI settings
) {
	tui := config.TUI
	
	*showTooltips = tui.ShowTooltips
	*defaultExpandLevel = tui.DefaultExpandLevel
	*useIcons = tui.FileOperationIcons
}

// GetCommandExtractors returns the command extractors from the configuration
func (config *UnifiedConfig) GetCommandExtractors() []CommandExtractor {
	return config.CommandExtractors
}