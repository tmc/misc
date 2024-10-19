package main

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func initLogger(debugLevel int) error {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	switch debugLevel {
	case 0:
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
		config.OutputPaths = []string{"stdout"}
		config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	case 1:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case 2:
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	var err error
	logger, err = config.Build()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	return nil
}

func logError(msg string, err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	logger.Error(msg, fields...)
}

func logInfo(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func logDebug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func logVerbose(msg string, fields ...zap.Field) {
	if logger.Core().Enabled(zapcore.DebugLevel) {
		// Elide or redact sensitive information for verbose logging
		for i, field := range fields {
			if field.Key == "audio_data" || field.Key == "raw_data" {
				fields[i] = zap.String(field.Key, "<elided>")
			}
		}
		logger.Debug("VERBOSE: "+msg, fields...)
	}
}

func generateID(prefix string) string {
	const charset = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	b := make([]byte, 21-len(prefix))
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return prefix + string(b)
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Helper function to print text output
func printText(text string) {
	fmt.Println(text)
}
