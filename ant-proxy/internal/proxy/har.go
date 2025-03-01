package proxy

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"

    "github.com/google/martian/v3/har"
)

// SaveRequestResponse saves the request and response to a HAR entry
func SaveRequestResponse(req *http.Request, resp *http.Response) (*har.Entry, error) {
    entry := &har.Entry{
        StartedDateTime: har.Time(time.Now()),
        Request:        har.NewRequest(req),
        Response:       har.NewResponse(resp),
    }
    return entry, nil
}

