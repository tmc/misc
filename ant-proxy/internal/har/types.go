package har

import (
    "time"
)

// Entry represents a HAR entry
type Entry struct {
    Request     Request     `json:"request"`
    Response    Response    `json:"response"`
    StartedAt   time.Time   `json:"startedAt"`
    CompletedAt time.Time   `json:"completedAt"`
}

type Request struct {
    Method  string            `json:"method"`
    URL     string            `json:"url"`
    Headers map[string]string `json:"headers"`
    Body    string            `json:"body"`
}

type Response struct {
    Status  int               `json:"status"`
    Headers map[string]string `json:"headers"`
    Body    string           `json:"body"`
}

