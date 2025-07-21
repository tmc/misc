package health

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Configuration management for health monitoring system

// HealthConfiguration represents the complete health monitoring configuration
type HealthConfiguration struct {
	// Core settings
	Enabled          bool          `json:"enabled" yaml:"enabled"`
	CheckInterval    time.Duration `json:"check_interval" yaml:"check_interval"`
	DefaultTimeout   time.Duration `json:"default_timeout" yaml:"default_timeout"`
	RetentionPeriod  time.Duration `json:"retention_period" yaml:"retention_period"`
	MaxHistorySize   int           `json:"max_history_size" yaml:"max_history_size"`
	
	// Feature flags
	EnablePredictive    bool `json:"enable_predictive" yaml:"enable_predictive"`
	EnableNotifications bool `json:"enable_notifications" yaml:"enable_notifications"`
	EnableMetrics       bool `json:"enable_metrics" yaml:"enable_metrics"`
	EnableDashboard     bool `json:"enable_dashboard" yaml:"enable_dashboard"`
	EnableAlerting      bool `json:"enable_alerting" yaml:"enable_alerting"`
	
	// Logging
	LogLevel string `json:"log_level" yaml:"log_level"`
	LogFile  string `json:"log_file" yaml:"log_file"`
	
	// Paths
	DataDir           string `json:"data_dir" yaml:"data_dir"`
	ConfigDir         string `json:"config_dir" yaml:"config_dir"`
	
	// Endpoints
	HealthEndpoint    string `json:"health_endpoint" yaml:"health_endpoint"`
	MetricsEndpoint   string `json:"metrics_endpoint" yaml:"metrics_endpoint"`
	DashboardEndpoint string `json:"dashboard_endpoint" yaml:"dashboard_endpoint"`
	
	// Dashboard configuration
	Dashboard DashboardConfiguration `json:"dashboard" yaml:"dashboard"`
	
	// Alert configuration
	Alerting AlertingConfiguration `json:"alerting" yaml:"alerting"`
	
	// Performance thresholds
	Thresholds ThresholdConfiguration `json:"thresholds" yaml:"thresholds"`
	
	// Check-specific configurations
	CheckConfigs map[string]CheckConfiguration `json:"check_configs" yaml:"check_configs"`
	
	// Security settings
	Security SecurityConfiguration `json:"security" yaml:"security"`
}

// DashboardConfiguration configures the web dashboard
type DashboardConfiguration struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Port        int    `json:"port" yaml:"port"`
	Host        string `json:"host" yaml:"host"`
	BasePath    string `json:"base_path" yaml:"base_path"`
	Title       string `json:"title" yaml:"title"`
	RefreshRate int    `json:"refresh_rate" yaml:"refresh_rate"`
	Theme       string `json:"theme" yaml:"theme"`
	EnableAPI   bool   `json:"enable_api" yaml:"enable_api"`
	EnableAuth  bool   `json:"enable_auth" yaml:"enable_auth"`
	Username    string `json:"username" yaml:"username"`
	Password    string `json:"password" yaml:"password"`
}

// AlertingConfiguration configures alerting behavior
type AlertingConfiguration struct {
	Enabled               bool                    `json:"enabled" yaml:"enabled"`
	DefaultSeverity       string                  `json:"default_severity" yaml:"default_severity"`
	FailureThreshold      int                     `json:"failure_threshold" yaml:"failure_threshold"`
	RecoveryNotification  bool                    `json:"recovery_notification" yaml:"recovery_notification"`
	SuppressFor           time.Duration           `json:"suppress_for" yaml:"suppress_for"`
	EscalationEnabled     bool                    `json:"escalation_enabled" yaml:"escalation_enabled"`
	Handlers              []AlertHandlerConfig    `json:"handlers" yaml:"handlers"`
	Rules                 []AlertRuleConfig       `json:"rules" yaml:"rules"`
}

// AlertHandlerConfig configures alert handlers
type AlertHandlerConfig struct {
	Type        string                 `json:"type" yaml:"type"`
	Enabled     bool                   `json:"enabled" yaml:"enabled"`
	Config      map[string]interface{} `json:"config" yaml:"config"`
	Severities  []string               `json:"severities" yaml:"severities"`
	Categories  []string               `json:"categories" yaml:"categories"`
}

// AlertRuleConfig configures alert rules
type AlertRuleConfig struct {
	Name         string             `json:"name" yaml:"name"`
	Condition    string             `json:"condition" yaml:"condition"`
	Severity     string             `json:"severity" yaml:"severity"`
	Message      string             `json:"message" yaml:"message"`
	Enabled      bool               `json:"enabled" yaml:"enabled"`
	Checks       []string           `json:"checks" yaml:"checks"`
	Categories   []string           `json:"categories" yaml:"categories"`
	Thresholds   map[string]float64 `json:"thresholds" yaml:"thresholds"`
	Escalation   EscalationConfig   `json:"escalation" yaml:"escalation"`
}

// EscalationConfig configures alert escalation
type EscalationConfig struct {
	Enabled       bool          `json:"enabled" yaml:"enabled"`
	AfterFailures int           `json:"after_failures" yaml:"after_failures"`
	AfterDuration time.Duration `json:"after_duration" yaml:"after_duration"`
	Severity      string        `json:"severity" yaml:"severity"`
	Actions       []string      `json:"actions" yaml:"actions"`
}

// ThresholdConfiguration defines performance thresholds
type ThresholdConfiguration struct {
	ResponseTime    ThresholdLevels `json:"response_time" yaml:"response_time"`
	MemoryUsage     ThresholdLevels `json:"memory_usage" yaml:"memory_usage"`
	CPUUsage        ThresholdLevels `json:"cpu_usage" yaml:"cpu_usage"`
	ErrorRate       ThresholdLevels `json:"error_rate" yaml:"error_rate"`
	AvailabilityRate ThresholdLevels `json:"availability_rate" yaml:"availability_rate"`
}

// ThresholdLevels defines warning and critical levels
type ThresholdLevels struct {
	Warning  float64 `json:"warning" yaml:"warning"`
	Critical float64 `json:"critical" yaml:"critical"`
}

// CheckConfiguration configures individual health checks
type CheckConfiguration struct {
	Enabled      bool                   `json:"enabled" yaml:"enabled"`
	Interval     time.Duration          `json:"interval" yaml:"interval"`
	Timeout      time.Duration          `json:"timeout" yaml:"timeout"`
	Critical     bool                   `json:"critical" yaml:"critical"`
	Description  string                 `json:"description" yaml:"description"`
	Category     string                 `json:"category" yaml:"category"`
	Tags         []string               `json:"tags" yaml:"tags"`
	Thresholds   map[string]float64     `json:"thresholds" yaml:"thresholds"`
	Dependencies []string               `json:"dependencies" yaml:"dependencies"`
	RetryConfig  RetryConfiguration     `json:"retry_config" yaml:"retry_config"`
	CustomConfig map[string]interface{} `json:"custom_config" yaml:"custom_config"`
}

// RetryConfiguration configures retry behavior
type RetryConfiguration struct {
	MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay" yaml:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay" yaml:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor" yaml:"backoff_factor"`
	JitterEnabled   bool          `json:"jitter_enabled" yaml:"jitter_enabled"`
}

// SecurityConfiguration configures security settings
type SecurityConfiguration struct {
	EnableAuth          bool     `json:"enable_auth" yaml:"enable_auth"`
	AuthToken           string   `json:"auth_token" yaml:"auth_token"`
	AllowedIPs          []string `json:"allowed_ips" yaml:"allowed_ips"`
	EnableTLS           bool     `json:"enable_tls" yaml:"enable_tls"`
	TLSCertFile         string   `json:"tls_cert_file" yaml:"tls_cert_file"`
	TLSKeyFile          string   `json:"tls_key_file" yaml:"tls_key_file"`
	EnableRateLimit     bool     `json:"enable_rate_limit" yaml:"enable_rate_limit"`
	RateLimitPerMinute  int      `json:"rate_limit_per_minute" yaml:"rate_limit_per_minute"`
	EnableAPIKey        bool     `json:"enable_api_key" yaml:"enable_api_key"`
	APIKeys             []string `json:"api_keys" yaml:"api_keys"`
}

// ConfigManager manages health monitoring configuration
type ConfigManager struct {
	config     *HealthConfiguration
	configPath string
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
	}
}

// LoadConfig loads configuration from file
func (cm *ConfigManager) LoadConfig() (*HealthConfiguration, error) {
	if cm.config != nil {
		return cm.config, nil
	}
	
	// If config file doesn't exist, create default
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		config := cm.getDefaultConfig()
		if err := cm.SaveConfig(config); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		cm.config = config
		return config, nil
	}
	
	// Load config from file
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config HealthConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// Validate and set defaults
	if err := cm.validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	cm.config = &config
	return cm.config, nil
}

// SaveConfig saves configuration to file
func (cm *ConfigManager) SaveConfig(config *HealthConfiguration) error {
	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(cm.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Validate config
	if err := cm.validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	cm.config = config
	return nil
}

// GetConfig returns current configuration
func (cm *ConfigManager) GetConfig() *HealthConfiguration {
	return cm.config
}

// UpdateConfig updates configuration
func (cm *ConfigManager) UpdateConfig(updates map[string]interface{}) error {
	if cm.config == nil {
		return fmt.Errorf("configuration not loaded")
	}
	
	// Apply updates (simplified - in real implementation, would use reflection or similar)
	// This is a placeholder for the actual update logic
	
	return cm.SaveConfig(cm.config)
}

// getDefaultConfig returns default configuration
func (cm *ConfigManager) getDefaultConfig() *HealthConfiguration {
	return &HealthConfiguration{
		Enabled:             true,
		CheckInterval:       30 * time.Second,
		DefaultTimeout:      10 * time.Second,
		RetentionPeriod:     24 * time.Hour,
		MaxHistorySize:      1000,
		EnablePredictive:    true,
		EnableNotifications: true,
		EnableMetrics:       true,
		EnableDashboard:     true,
		EnableAlerting:      true,
		LogLevel:            "info",
		LogFile:             "",
		DataDir:             "./health_data",
		ConfigDir:           "./config",
		HealthEndpoint:      "/health",
		MetricsEndpoint:     "/metrics",
		DashboardEndpoint:   "/dashboard",
		
		Dashboard: DashboardConfiguration{
			Enabled:     true,
			Port:        8080,
			Host:        "localhost",
			BasePath:    "/health",
			Title:       "Health Dashboard",
			RefreshRate: 30,
			Theme:       "auto",
			EnableAPI:   true,
			EnableAuth:  false,
		},
		
		Alerting: AlertingConfiguration{
			Enabled:              true,
			DefaultSeverity:      "warning",
			FailureThreshold:     3,
			RecoveryNotification: true,
			SuppressFor:          5 * time.Minute,
			EscalationEnabled:    true,
			Handlers: []AlertHandlerConfig{
				{
					Type:       "log",
					Enabled:    true,
					Config:     map[string]interface{}{},
					Severities: []string{"info", "warning", "error", "critical"},
					Categories: []string{"browser", "network", "memory", "performance", "extension", "api", "system"},
				},
			},
			Rules: []AlertRuleConfig{
				{
					Name:      "High Memory Usage",
					Condition: "memory_usage > warning_threshold",
					Severity:  "warning",
					Message:   "Memory usage is high",
					Enabled:   true,
					Categories: []string{"memory", "performance"},
					Thresholds: map[string]float64{
						"warning_threshold": 512,
						"critical_threshold": 1024,
					},
				},
				{
					Name:      "High Response Time",
					Condition: "response_time > warning_threshold",
					Severity:  "warning",
					Message:   "Response time is high",
					Enabled:   true,
					Categories: []string{"performance", "api"},
					Thresholds: map[string]float64{
						"warning_threshold": 5000,
						"critical_threshold": 10000,
					},
				},
			},
		},
		
		Thresholds: ThresholdConfiguration{
			ResponseTime: ThresholdLevels{
				Warning:  5000,  // 5 seconds
				Critical: 10000, // 10 seconds
			},
			MemoryUsage: ThresholdLevels{
				Warning:  512,  // 512 MB
				Critical: 1024, // 1 GB
			},
			CPUUsage: ThresholdLevels{
				Warning:  70,  // 70%
				Critical: 90,  // 90%
			},
			ErrorRate: ThresholdLevels{
				Warning:  5,   // 5%
				Critical: 10,  // 10%
			},
			AvailabilityRate: ThresholdLevels{
				Warning:  95,  // 95%
				Critical: 90,  // 90%
			},
		},
		
		CheckConfigs: map[string]CheckConfiguration{
			"chrome-connection": {
				Enabled:     true,
				Interval:    30 * time.Second,
				Timeout:     5 * time.Second,
				Critical:    true,
				Description: "Chrome DevTools connection health",
				Category:    "browser",
				Tags:        []string{"chrome", "devtools", "connection"},
				Thresholds:  map[string]float64{"response_time": 3000},
				RetryConfig: RetryConfiguration{
					MaxRetries:    3,
					InitialDelay:  1 * time.Second,
					MaxDelay:      30 * time.Second,
					BackoffFactor: 2.0,
					JitterEnabled: true,
				},
			},
			"ai-api": {
				Enabled:     true,
				Interval:    60 * time.Second,
				Timeout:     10 * time.Second,
				Critical:    true,
				Description: "AI API availability",
				Category:    "api",
				Tags:        []string{"ai", "api", "chrome"},
				Thresholds:  map[string]float64{"response_time": 5000},
				RetryConfig: RetryConfiguration{
					MaxRetries:    3,
					InitialDelay:  1 * time.Second,
					MaxDelay:      30 * time.Second,
					BackoffFactor: 2.0,
					JitterEnabled: true,
				},
			},
			"memory-usage": {
				Enabled:     true,
				Interval:    45 * time.Second,
				Timeout:     3 * time.Second,
				Critical:    false,
				Description: "Memory usage monitoring",
				Category:    "performance",
				Tags:        []string{"memory", "performance", "system"},
				Thresholds: map[string]float64{
					"warning_mb":  512,
					"critical_mb": 1024,
				},
				RetryConfig: RetryConfiguration{
					MaxRetries:    2,
					InitialDelay:  500 * time.Millisecond,
					MaxDelay:      10 * time.Second,
					BackoffFactor: 1.5,
					JitterEnabled: false,
				},
			},
			"network-connectivity": {
				Enabled:     true,
				Interval:    90 * time.Second,
				Timeout:     8 * time.Second,
				Critical:    false,
				Description: "Network connectivity check",
				Category:    "network",
				Tags:        []string{"network", "connectivity", "internet"},
				Thresholds:  map[string]float64{"response_time": 5000},
				RetryConfig: RetryConfiguration{
					MaxRetries:    3,
					InitialDelay:  2 * time.Second,
					MaxDelay:      30 * time.Second,
					BackoffFactor: 2.0,
					JitterEnabled: true,
				},
			},
		},
		
		Security: SecurityConfiguration{
			EnableAuth:         false,
			AuthToken:          "",
			AllowedIPs:         []string{"127.0.0.1", "::1"},
			EnableTLS:          false,
			TLSCertFile:        "",
			TLSKeyFile:         "",
			EnableRateLimit:    true,
			RateLimitPerMinute: 100,
			EnableAPIKey:       false,
			APIKeys:            []string{},
		},
	}
}

// validateConfig validates configuration
func (cm *ConfigManager) validateConfig(config *HealthConfiguration) error {
	if config.CheckInterval <= 0 {
		return fmt.Errorf("check_interval must be positive")
	}
	
	if config.DefaultTimeout <= 0 {
		return fmt.Errorf("default_timeout must be positive")
	}
	
	if config.MaxHistorySize <= 0 {
		return fmt.Errorf("max_history_size must be positive")
	}
	
	if config.Dashboard.Enabled && config.Dashboard.Port <= 0 {
		return fmt.Errorf("dashboard port must be positive")
	}
	
	// Validate check configurations
	for name, checkConfig := range config.CheckConfigs {
		if checkConfig.Interval <= 0 {
			return fmt.Errorf("check %s: interval must be positive", name)
		}
		
		if checkConfig.Timeout <= 0 {
			return fmt.Errorf("check %s: timeout must be positive", name)
		}
		
		if checkConfig.RetryConfig.MaxRetries < 0 {
			return fmt.Errorf("check %s: max_retries must be non-negative", name)
		}
	}
	
	// Validate threshold levels
	if config.Thresholds.ResponseTime.Warning <= 0 || config.Thresholds.ResponseTime.Critical <= 0 {
		return fmt.Errorf("response time thresholds must be positive")
	}
	
	if config.Thresholds.ResponseTime.Warning >= config.Thresholds.ResponseTime.Critical {
		return fmt.Errorf("response time warning threshold must be less than critical threshold")
	}
	
	return nil
}

// GetCheckConfig returns configuration for a specific check
func (cm *ConfigManager) GetCheckConfig(checkName string) (*CheckConfiguration, bool) {
	if cm.config == nil {
		return nil, false
	}
	
	config, exists := cm.config.CheckConfigs[checkName]
	return &config, exists
}

// UpdateCheckConfig updates configuration for a specific check
func (cm *ConfigManager) UpdateCheckConfig(checkName string, config CheckConfiguration) error {
	if cm.config == nil {
		return fmt.Errorf("configuration not loaded")
	}
	
	// Validate check config
	if config.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	
	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	
	cm.config.CheckConfigs[checkName] = config
	return cm.SaveConfig(cm.config)
}

// GetAvailableChecks returns list of available health checks
func (cm *ConfigManager) GetAvailableChecks() []string {
	if cm.config == nil {
		return []string{}
	}
	
	checks := make([]string, 0, len(cm.config.CheckConfigs))
	for name := range cm.config.CheckConfigs {
		checks = append(checks, name)
	}
	
	return checks
}

// ReloadConfig reloads configuration from file
func (cm *ConfigManager) ReloadConfig() error {
	cm.config = nil
	_, err := cm.LoadConfig()
	return err
}

// ValidateConfiguration validates a configuration without saving
func ValidateConfiguration(config *HealthConfiguration) error {
	cm := &ConfigManager{}
	return cm.validateConfig(config)
}

// CreateDefaultConfigFile creates a default configuration file
func CreateDefaultConfigFile(path string) error {
	cm := NewConfigManager(path)
	config := cm.getDefaultConfig()
	return cm.SaveConfig(config)
}