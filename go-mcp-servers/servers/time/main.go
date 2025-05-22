package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tmc/misc/go-mcp-servers/lib/mcpframework"
)

func main() {
	server := mcpframework.NewServer("time-mcp-server", "1.0.0")
	server.SetInstructions("A Model Context Protocol server that provides time and date utilities including current time, timezone conversions, date parsing, and scheduling helpers.")

	// Register time tools
	registerTimeTools(server)
	setupResourceHandlers(server)

	// Run the server
	ctx := context.Background()
	if err := server.Run(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func registerTimeTools(server *mcpframework.Server) {
	// Current time tool
	server.RegisterTool("current_time", "Get the current time", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "Timezone to display time in (e.g., 'UTC', 'America/New_York')",
				"default":     "Local",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Time format (RFC3339, RFC822, Kitchen, Unix, or custom Go format)",
				"default":     "RFC3339",
			},
		},
	}, handleCurrentTime)

	// Parse time tool
	server.RegisterTool("parse_time", "Parse a time string", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"time_string": map[string]interface{}{
				"type":        "string",
				"description": "Time string to parse",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Expected format of the time string (optional)",
			},
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "Timezone to interpret the time in",
				"default":     "Local",
			},
		},
		Required: []string{"time_string"},
	}, handleParseTime)

	// Convert timezone tool
	server.RegisterTool("convert_timezone", "Convert time from one timezone to another", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"time_string": map[string]interface{}{
				"type":        "string",
				"description": "Time string to convert",
			},
			"from_timezone": map[string]interface{}{
				"type":        "string",
				"description": "Source timezone",
				"default":     "Local",
			},
			"to_timezone": map[string]interface{}{
				"type":        "string",
				"description": "Target timezone",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Output format",
				"default":     "RFC3339",
			},
		},
		Required: []string{"time_string", "to_timezone"},
	}, handleConvertTimezone)

	// Add duration tool
	server.RegisterTool("add_duration", "Add duration to a time", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"time_string": map[string]interface{}{
				"type":        "string",
				"description": "Base time (if empty, uses current time)",
			},
			"duration": map[string]interface{}{
				"type":        "string",
				"description": "Duration to add (e.g., '1h30m', '2d', '3w')",
			},
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "Timezone for the calculation",
				"default":     "Local",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Output format",
				"default":     "RFC3339",
			},
		},
		Required: []string{"duration"},
	}, handleAddDuration)

	// Time difference tool
	server.RegisterTool("time_diff", "Calculate difference between two times", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"time1": map[string]interface{}{
				"type":        "string",
				"description": "First time",
			},
			"time2": map[string]interface{}{
				"type":        "string",
				"description": "Second time (if empty, uses current time)",
			},
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "Timezone for interpretation",
				"default":     "Local",
			},
			"unit": map[string]interface{}{
				"type":        "string",
				"description": "Unit for result (seconds, minutes, hours, days)",
				"default":     "auto",
			},
		},
		Required: []string{"time1"},
	}, handleTimeDiff)

	// Format time tool
	server.RegisterTool("format_time", "Format a time in different ways", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"time_string": map[string]interface{}{
				"type":        "string",
				"description": "Time to format (if empty, uses current time)",
			},
			"formats": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of formats to show",
				"default":     []string{"RFC3339", "RFC822", "Kitchen", "Unix"},
			},
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "Timezone for display",
				"default":     "Local",
			},
		},
	}, handleFormatTime)

	// List timezones tool
	server.RegisterTool("list_timezones", "List available timezones", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"filter": map[string]interface{}{
				"type":        "string",
				"description": "Filter timezones by region or name",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of timezones to return",
				"default":     50,
			},
		},
	}, handleListTimezones)

	// Sleep/wait tool
	server.RegisterTool("sleep", "Sleep for a specified duration", &mcpframework.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"duration": map[string]interface{}{
				"type":        "string",
				"description": "Duration to sleep (e.g., '5s', '1m', '2h')",
			},
		},
		Required: []string{"duration"},
	}, handleSleep)
}

func setupResourceHandlers(server *mcpframework.Server) {
	// Set up resource listing for time information
	server.SetResourceLister(func(ctx context.Context) (*mcpframework.ListResourcesResult, error) {
		resources := []mcpframework.Resource{
			{
				URI:         "time://current",
				Name:        "Current Time",
				Description: "Current local time",
				MimeType:    "text/plain",
			},
			{
				URI:         "time://utc",
				Name:        "Current UTC Time",
				Description: "Current UTC time",
				MimeType:    "text/plain",
			},
			{
				URI:         "time://timezones",
				Name:        "Available Timezones",
				Description: "List of available timezones",
				MimeType:    "text/plain",
			},
		}
		return &mcpframework.ListResourcesResult{Resources: resources}, nil
	})

	// Set up resource reading for time data
	server.RegisterResourceHandler("time://*", func(ctx context.Context, uri string) (*mcpframework.ReadResourceResult, error) {
		switch {
		case strings.HasSuffix(uri, "/current"):
			return getTimeResourceContent("current")
		case strings.HasSuffix(uri, "/utc"):
			return getTimeResourceContent("utc")
		case strings.HasSuffix(uri, "/timezones"):
			return getTimeResourceContent("timezones")
		default:
			return nil, fmt.Errorf("unsupported time resource")
		}
	})
}

func getTimeResourceContent(resourceType string) (*mcpframework.ReadResourceResult, error) {
	var content string
	
	switch resourceType {
	case "current":
		content = time.Now().Format(time.RFC3339)
	case "utc":
		content = time.Now().UTC().Format(time.RFC3339)
	case "timezones":
		// List some common timezones
		timezones := []string{
			"UTC", "Local",
			"America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
			"Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Rome",
			"Asia/Tokyo", "Asia/Shanghai", "Asia/Kolkata", "Asia/Dubai",
			"Australia/Sydney", "Australia/Melbourne",
		}
		content = strings.Join(timezones, "\n")
	default:
		return nil, fmt.Errorf("unknown resource type")
	}

	return &mcpframework.ReadResourceResult{
		Contents: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: content,
			},
		},
	}, nil
}

func handleCurrentTime(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Timezone string `json:"timezone"`
		Format   string `json:"format"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Timezone == "" {
		args.Timezone = "Local"
	}
	if args.Format == "" {
		args.Format = "RFC3339"
	}

	now := time.Now()
	
	// Handle timezone
	if args.Timezone != "Local" {
		loc, err := time.LoadLocation(args.Timezone)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Invalid timezone: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
		now = now.In(loc)
	}

	formatted, err := formatTime(now, args.Format)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error formatting time: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: formatted,
			},
		},
	}, nil
}

func handleParseTime(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		TimeString string `json:"time_string"`
		Format     string `json:"format"`
		Timezone   string `json:"timezone"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var parsedTime time.Time
	var err error

	if args.Format != "" {
		parsedTime, err = time.Parse(args.Format, args.TimeString)
	} else {
		// Try common formats
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			time.RFC822,
			time.RFC822Z,
			time.Kitchen,
			"2006-01-02 15:04:05",
			"2006-01-02",
			"15:04:05",
		}

		for _, format := range formats {
			parsedTime, err = time.Parse(format, args.TimeString)
			if err == nil {
				break
			}
		}
	}

	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to parse time: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	// Apply timezone if specified
	if args.Timezone != "" && args.Timezone != "Local" {
		loc, err := time.LoadLocation(args.Timezone)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Invalid timezone: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
		parsedTime = parsedTime.In(loc)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Parsed time: %s\n", parsedTime.Format(time.RFC3339)))
	result.WriteString(fmt.Sprintf("Unix timestamp: %d\n", parsedTime.Unix()))
	result.WriteString(fmt.Sprintf("Weekday: %s\n", parsedTime.Weekday()))
	result.WriteString(fmt.Sprintf("Year day: %d\n", parsedTime.YearDay()))

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

func handleConvertTimezone(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		TimeString   string `json:"time_string"`
		FromTimezone string `json:"from_timezone"`
		ToTimezone   string `json:"to_timezone"`
		Format       string `json:"format"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.FromTimezone == "" {
		args.FromTimezone = "Local"
	}
	if args.Format == "" {
		args.Format = "RFC3339"
	}

	// Parse the input time
	parsedTime, err := time.Parse(time.RFC3339, args.TimeString)
	if err != nil {
		// Try other formats
		formats := []string{time.RFC822, time.Kitchen, "2006-01-02 15:04:05"}
		for _, format := range formats {
			parsedTime, err = time.Parse(format, args.TimeString)
			if err == nil {
				break
			}
		}
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Failed to parse time: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
	}

	// Apply source timezone
	if args.FromTimezone != "Local" {
		fromLoc, err := time.LoadLocation(args.FromTimezone)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Invalid source timezone: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
		parsedTime = parsedTime.In(fromLoc)
	}

	// Convert to target timezone
	toLoc, err := time.LoadLocation(args.ToTimezone)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Invalid target timezone: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	convertedTime := parsedTime.In(toLoc)
	formatted, err := formatTime(convertedTime, args.Format)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error formatting time: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: formatted,
			},
		},
	}, nil
}

func handleAddDuration(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		TimeString string `json:"time_string"`
		Duration   string `json:"duration"`
		Timezone   string `json:"timezone"`
		Format     string `json:"format"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Format == "" {
		args.Format = "RFC3339"
	}

	var baseTime time.Time
	if args.TimeString == "" {
		baseTime = time.Now()
	} else {
		var err error
		baseTime, err = time.Parse(time.RFC3339, args.TimeString)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Failed to parse time: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
	}

	// Parse duration (handle extended formats like 'd' for days, 'w' for weeks)
	duration, err := parseDuration(args.Duration)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Invalid duration: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	resultTime := baseTime.Add(duration)

	// Apply timezone
	if args.Timezone != "" && args.Timezone != "Local" {
		loc, err := time.LoadLocation(args.Timezone)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Invalid timezone: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
		resultTime = resultTime.In(loc)
	}

	formatted, err := formatTime(resultTime, args.Format)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error formatting time: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: formatted,
			},
		},
	}, nil
}

func handleTimeDiff(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Time1    string `json:"time1"`
		Time2    string `json:"time2"`
		Timezone string `json:"timezone"`
		Unit     string `json:"unit"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Unit == "" {
		args.Unit = "auto"
	}

	// Parse first time
	time1, err := time.Parse(time.RFC3339, args.Time1)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to parse time1: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	// Parse second time
	var time2 time.Time
	if args.Time2 == "" {
		time2 = time.Now()
	} else {
		time2, err = time.Parse(time.RFC3339, args.Time2)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Failed to parse time2: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
	}

	diff := time2.Sub(time1)
	
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Duration: %s\n", diff))
	result.WriteString(fmt.Sprintf("Seconds: %.0f\n", diff.Seconds()))
	result.WriteString(fmt.Sprintf("Minutes: %.2f\n", diff.Minutes()))
	result.WriteString(fmt.Sprintf("Hours: %.2f\n", diff.Hours()))
	result.WriteString(fmt.Sprintf("Days: %.2f\n", diff.Hours()/24))

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

func handleFormatTime(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		TimeString string   `json:"time_string"`
		Formats    []string `json:"formats"`
		Timezone   string   `json:"timezone"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if len(args.Formats) == 0 {
		args.Formats = []string{"RFC3339", "RFC822", "Kitchen", "Unix"}
	}

	var targetTime time.Time
	if args.TimeString == "" {
		targetTime = time.Now()
	} else {
		var err error
		targetTime, err = time.Parse(time.RFC3339, args.TimeString)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Failed to parse time: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
	}

	// Apply timezone
	if args.Timezone != "" && args.Timezone != "Local" {
		loc, err := time.LoadLocation(args.Timezone)
		if err != nil {
			return &mcpframework.CallToolResult{
				Content: []interface{}{
					mcpframework.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Invalid timezone: %s", err.Error()),
					},
				},
				IsError: true,
			}, nil
		}
		targetTime = targetTime.In(loc)
	}

	var result strings.Builder
	for _, formatName := range args.Formats {
		formatted, err := formatTime(targetTime, formatName)
		if err != nil {
			result.WriteString(fmt.Sprintf("%s: Error - %s\n", formatName, err.Error()))
		} else {
			result.WriteString(fmt.Sprintf("%s: %s\n", formatName, formatted))
		}
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

func handleListTimezones(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Filter string `json:"filter"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Limit == 0 {
		args.Limit = 50
	}

	// Common timezones
	timezones := []string{
		"UTC", "Local",
		"America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
		"America/Toronto", "America/Vancouver", "America/Mexico_City", "America/Sao_Paulo",
		"Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Rome", "Europe/Madrid",
		"Europe/Amsterdam", "Europe/Stockholm", "Europe/Moscow",
		"Asia/Tokyo", "Asia/Shanghai", "Asia/Seoul", "Asia/Hong_Kong", "Asia/Bangkok",
		"Asia/Kolkata", "Asia/Dubai", "Asia/Singapore", "Asia/Jakarta",
		"Australia/Sydney", "Australia/Melbourne", "Australia/Perth",
		"Pacific/Auckland", "Pacific/Honolulu",
		"Africa/Cairo", "Africa/Johannesburg", "Africa/Lagos",
	}

	var filtered []string
	count := 0
	for _, tz := range timezones {
		if count >= args.Limit {
			break
		}
		if args.Filter == "" || strings.Contains(strings.ToLower(tz), strings.ToLower(args.Filter)) {
			filtered = append(filtered, tz)
			count++
		}
	}

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: strings.Join(filtered, "\n"),
			},
		},
	}, nil
}

func handleSleep(ctx context.Context, params mcpframework.CallToolParams) (*mcpframework.CallToolResult, error) {
	var args struct {
		Duration string `json:"duration"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	duration, err := time.ParseDuration(args.Duration)
	if err != nil {
		return &mcpframework.CallToolResult{
			Content: []interface{}{
				mcpframework.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Invalid duration: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	startTime := time.Now()
	time.Sleep(duration)
	endTime := time.Now()

	return &mcpframework.CallToolResult{
		Content: []interface{}{
			mcpframework.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Slept for %s (actual: %s)", duration, endTime.Sub(startTime)),
			},
		},
	}, nil
}

func formatTime(t time.Time, format string) (string, error) {
	switch format {
	case "RFC3339":
		return t.Format(time.RFC3339), nil
	case "RFC822":
		return t.Format(time.RFC822), nil
	case "Kitchen":
		return t.Format(time.Kitchen), nil
	case "Unix":
		return strconv.FormatInt(t.Unix(), 10), nil
	case "UnixNano":
		return strconv.FormatInt(t.UnixNano(), 10), nil
	default:
		// Assume it's a custom Go time format
		return t.Format(format), nil
	}
}

func parseDuration(s string) (time.Duration, error) {
	// Handle extended formats
	if strings.Contains(s, "d") || strings.Contains(s, "w") {
		// Replace 'd' with 'h' * 24 and 'w' with 'h' * 168
		s = strings.ReplaceAll(s, "w", "168h")
		s = strings.ReplaceAll(s, "d", "24h")
	}
	
	return time.ParseDuration(s)
}