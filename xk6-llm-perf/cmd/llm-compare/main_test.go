package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoopbackGuard(t *testing.T) {
	h := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }), "127.0.0.1:1234")
	for _, tt := range []struct {
		host, origin string
		want         int
	}{{"127.0.0.1:1234", "", 200}, {"127.0.0.1:1234", "http://127.0.0.1:1234", 200}, {"evil.example:1234", "", 403}, {"127.0.0.1:1234", "http://evil.example", 403}, {"127.0.0.1:1234", "null", 403}} {
		r := httptest.NewRequest("GET", "http://"+tt.host+"/comparison.json", nil)
		r.Header.Set("Origin", tt.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Fatalf("%s %s: %d", tt.host, tt.origin, w.Code)
		}
	}
}
func TestInputBounds(t *testing.T) {
	if _, err := load(make([]string, 65)); err == nil {
		t.Fatal("oversized input accepted")
	}
	if err := run([]string{"-format=png"}, &bytes.Buffer{}); err == nil {
		t.Fatal("missing files accepted")
	}
}
