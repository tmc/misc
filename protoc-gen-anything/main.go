package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	logLevel := flags.String("log_level", "info", "log level (debug, info, warn, error)")
	templateDir := flags.String("templates", "", "path to custom templates")
	verboseMode := flags.Bool("verbose", false, "enable verbose mode")
	continueOnError := flags.Bool("continue_on_error", false, "continue on error")
	runGoImports := flags.Bool("goimports", false, "run goimports on generated Go files")
	txtarMode := flags.Bool("txtar", false, "parse template output as txtar archives")
	colorMode := flags.String("color", "auto", "color output: auto, always, never")
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}
	opts.Run(func(p *protogen.Plugin) error {
		return NewGenerator(Options{
			TemplateDir:     *templateDir,
			Verbose:         *verboseMode,
			ContinueOnError: *continueOnError,
			RunGoImports:    *runGoImports,
			TxtarMode:       *txtarMode,
			Logger:          setupLogger(*logLevel, *colorMode),
		}).Generate(p)
	})
}

func setupLogger(logLevel, colorMode string) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Determine if we should use color
	useColor := false
	switch colorMode {
	case "always":
		useColor = true
	case "never":
		useColor = false
	default: // "auto"
		useColor = term.IsTerminal(int(os.Stderr.Fd()))
	}

	if useColor {
		return slog.New(&colorHandler{
			out:   os.Stderr,
			level: level,
		})
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// colorHandler is a colored slog handler for terminal output
type colorHandler struct {
	out   io.Writer
	level slog.Level
}

func (h *colorHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *colorHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	// For INFO messages, just show file being generated (simplified)
	if r.Level == slog.LevelInfo && r.Message == "generating file" {
		var fileName string
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "outputFileName" {
				fileName = a.Value.String()
			}
			return true
		})
		fmt.Fprintf(&b, "%s✓%s %s\n", colorGreen, colorReset, fileName)
		_, err := h.out.Write([]byte(b.String()))
		return err
	}

	// For errors, show clean error format
	if r.Level == slog.LevelError {
		var errVal string
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "error" {
				errVal = a.Value.String()
			}
			return true
		})

		// Extract the YAML error with context (file:line:col: message\n    context\n    ^)
		if yamlErr := extractYamlError(errVal); yamlErr != "" {
			// Split into parts for coloring
			lines := strings.SplitN(yamlErr, "\n", 3)
			fmt.Fprintf(&b, "%s%serror:%s %s\n", colorRed, colorBold, colorReset, lines[0])
			if len(lines) > 1 {
				fmt.Fprintf(&b, "%s%s%s\n", colorGray, lines[1], colorReset)
			}
			if len(lines) > 2 {
				fmt.Fprintf(&b, "%s%s%s%s\n", colorRed, colorBold, lines[2], colorReset)
			}
		} else {
			fmt.Fprintf(&b, "%s%serror:%s %s\n", colorRed, colorBold, colorReset, r.Message)
		}
		_, err := h.out.Write([]byte(b.String()))
		return err
	}

	// For warnings
	if r.Level == slog.LevelWarn {
		fmt.Fprintf(&b, "%s%swarn:%s %s\n", colorYellow, colorBold, colorReset, r.Message)
		_, err := h.out.Write([]byte(b.String()))
		return err
	}

	// Default: just message
	fmt.Fprintf(&b, "%s\n", r.Message)
	_, err := h.out.Write([]byte(b.String()))
	return err
}

// extractYamlError pulls out the clean yaml error from a template error
func extractYamlError(errStr string) string {
	// Look for pattern: file.yaml:N:N: unknown field "xyz"\n    context\n    ^
	if idx := strings.Index(errStr, ".yaml:"); idx >= 0 {
		// Find start of filename
		start := idx
		for start > 0 && errStr[start-1] != ' ' && errStr[start-1] != ':' {
			start--
		}
		return errStr[start:]
	}
	return ""
}

func (h *colorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // simplified - doesn't carry attrs
}

func (h *colorHandler) WithGroup(name string) slog.Handler {
	return h // simplified - doesn't support groups
}
