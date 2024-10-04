package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/fatih/color"
)

func logDebug(state *AppState, format string, v ...interface{}) {
	if state.DebugLevel > 0 {
		color.Set(color.FgHiCyan)
		log.Printf(format, v...)
		color.Unset()
	}
}

func logVerbose(state *AppState, format string, v ...interface{}) {
	if state.DebugLevel > 1 {
		color.Set(color.FgHiMagenta)
		log.Printf(format, v...)
		color.Unset()
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
