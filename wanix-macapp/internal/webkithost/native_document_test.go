package webkithost

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePageSelection(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		pageCount uint
		want      []uint
		wantErr   bool
	}{
		{name: "all", pageCount: 3, want: []uint{0, 1, 2}},
		{name: "single", in: "2", pageCount: 3, want: []uint{1}},
		{name: "range", in: "1-3", pageCount: 4, want: []uint{0, 1, 2}},
		{name: "list", in: "1,3", pageCount: 3, want: []uint{0, 2}},
		{name: "zero", in: "0", pageCount: 3, wantErr: true},
		{name: "past end", in: "4", pageCount: 3, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePageSelection(tt.in, tt.pageCount)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("pages = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatPDFMeta(t *testing.T) {
	got, err := formatPDFMeta(pdfMeta{PageCount: 2, AllowsCopying: true})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{`"page_count": 2`, `"allows_copying": true`} {
		if !strings.Contains(text, want) {
			t.Fatalf("meta = %q, missing %q", text, want)
		}
	}
	if got[len(got)-1] != '\n' {
		t.Fatalf("meta does not end in newline: %q", got)
	}
}
