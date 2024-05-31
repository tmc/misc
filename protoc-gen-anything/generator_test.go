package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestGenerate(t *testing.T) {
	type args struct {
		templateDir string
	}
	tests := []struct {
		name    string
		args    args
		want    *Generator
		wantErr bool
	}{
		{"basics", args{templateDir: "."}, &Generator{TemplateDir: "."}, false},
		{"bad-directory", args{templateDir: "testdata/non-existent"}, &Generator{TemplateDir: "testdata/non-existent"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(tt.args.templateDir)
			if diff := cmp.Diff(tt.want, g, cmpopts.IgnoreUnexported(*g)); diff != "" {
				t.Errorf("newGenerator() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
