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
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	switch debugLevel {
	case 0:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case 1:
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	var err error
	logger, err = config.Build()
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	return nil
}

func logError(msg string, err error, fields ...zap.Field) {
	logger.Error(msg, append(fields, zap.Error(err))...)
}

func logInfo(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func logDebug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func logVerbose(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
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
