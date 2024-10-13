package main

import (
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/fatih/color"
)

type AppError struct {
    Err     error
    Message string
    Code    string
}

func (e *AppError) Error() string {
    return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func NewAppError(err error, message, code string) *AppError {
    return &AppError{
        Err:     err,
        Message: message,
        Code:    code,
    }
}

func logError(state *AppState, err error, format string, v ...interface{}) {
    msg := fmt.Sprintf(format, v...)
    if appErr, ok := err.(*AppError); ok {
        color.Set(color.FgHiRed)
        log.Printf("[ERROR] %s: %s (Code: %s)", msg, appErr.Error(), appErr.Code)
    } else {
        color.Set(color.FgHiRed)
        log.Printf("[ERROR] %s: %v", msg, err)
    }
    color.Unset()
}

func logInfo(state *AppState, format string, v ...interface{}) {
    color.Set(color.FgHiBlue)
    log.Printf("[INFO] "+format, v...)
    color.Unset()
}

func logDebug(state *AppState, format string, v ...interface{}) {
    if state.DebugLevel > 0 {
        color.Set(color.FgHiCyan)
        log.Printf("[DEBUG] "+format, v...)
        color.Unset()
    }
}

func logVerbose(state *AppState, format string, v ...interface{}) {
    if state.DebugLevel > 1 {
        color.Set(color.FgHiMagenta)
        log.Printf("[VERBOSE] "+format, v...)
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
