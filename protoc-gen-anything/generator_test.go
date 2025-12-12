package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		want    *Generator
		wantErr bool
	}{
		{"basics", Options{TemplateDir: "."}, &Generator{TemplateDir: "."}, false},
		{"bad-directory", Options{TemplateDir: "testdata/non-existent"}, &Generator{TemplateDir: "testdata/non-existent"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(tt.opts)
			if diff := cmp.Diff(tt.want, g, cmpopts.IgnoreUnexported(*g)); diff != "" {
				t.Errorf("NewGenerator() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
