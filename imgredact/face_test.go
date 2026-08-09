package main

import (
	"image"
	"testing"

	"github.com/tmc/apple/corefoundation"
)

func box(x, y, w, h float64) corefoundation.CGRect {
	return corefoundation.CGRect{
		Origin: corefoundation.CGPoint{X: x, Y: y},
		Size:   corefoundation.CGSize{Width: w, Height: h},
	}
}

func TestNormRect(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 100)
	tests := []struct {
		name string
		box  corefoundation.CGRect
		want image.Rectangle
	}{
		{name: "centered", box: box(0.25, 0.25, 0.5, 0.25), want: image.Rect(50, 50, 150, 75)},
		{name: "bottom left", box: box(0, 0, 0.5, 0.5), want: image.Rect(0, 50, 100, 100)},
		{name: "full frame", box: box(0, 0, 1, 1), want: image.Rect(0, 0, 200, 100)},
	}
	for _, tt := range tests {
		if got := normRect(tt.box, bounds); got != tt.want {
			t.Errorf("%s: normRect = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestExpandRect(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	tests := []struct {
		name string
		r    image.Rectangle
		frac float64
		want image.Rectangle
	}{
		{name: "quarter", r: image.Rect(40, 40, 60, 60), frac: 0.25, want: image.Rect(35, 35, 65, 65)},
		{name: "zero", r: image.Rect(40, 40, 60, 60), frac: 0, want: image.Rect(40, 40, 60, 60)},
		{name: "negative clamped", r: image.Rect(40, 40, 60, 60), frac: -1, want: image.Rect(40, 40, 60, 60)},
		{name: "clipped to bounds", r: image.Rect(0, 0, 20, 20), frac: 0.5, want: image.Rect(0, 0, 30, 30)},
	}
	for _, tt := range tests {
		if got := expandRect(tt.r, bounds, tt.frac); got != tt.want {
			t.Errorf("%s: expandRect = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFaceRects(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 200)
	boxes := []corefoundation.CGRect{
		box(0.25, 0.5, 0.2, 0.2), // 50,60 .. 90,100 before expansion
		box(0, 0, 0, 0),          // degenerate: dropped
	}
	got := faceRects(boxes, bounds, 0.25)
	want := []image.Rectangle{image.Rect(40, 50, 100, 110)}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("faceRects = %v, want %v", got, want)
	}
}

func TestFaceRectsEmpty(t *testing.T) {
	if got := faceRects(nil, image.Rect(0, 0, 10, 10), 0.25); got != nil {
		t.Fatalf("faceRects(nil) = %v, want nil", got)
	}
}
