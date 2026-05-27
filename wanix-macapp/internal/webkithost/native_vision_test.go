package webkithost

import (
	"strings"
	"testing"
)

func TestFormatVisionBarcodes(t *testing.T) {
	barcodes := []visionBarcode{{Payload: "hello", Symbology: "VNBarcodeSymbologyQR", Confidence: 0.99}}
	text, err := formatVisionBarcodes(barcodes, "text")
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "hello\n" {
		t.Fatalf("text = %q", text)
	}
	data, err := formatVisionBarcodes(barcodes, "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"payload": "hello"`) {
		t.Fatalf("json = %q", data)
	}
}

func TestFormatVisionClassifications(t *testing.T) {
	classes := []visionClassification{{Identifier: "animal", Confidence: 0.75}}
	text, err := formatVisionClassifications(classes, "text")
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "animal\n" {
		t.Fatalf("text = %q", text)
	}
	data, err := formatVisionClassifications(classes, "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"identifier": "animal"`) {
		t.Fatalf("json = %q", data)
	}
}

func TestFormatVisionRectangles(t *testing.T) {
	rectangles := []visionRectangle{{
		Confidence:  0.8,
		BoundingBox: visionRect{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4},
	}}
	text, err := formatVisionRectangles(rectangles, "text")
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "0.1 0.2 0.3 0.4 0.8\n" {
		t.Fatalf("text = %q", text)
	}
	data, err := formatVisionRectangles(rectangles, "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"bounding_box"`) {
		t.Fatalf("json = %q", data)
	}
}
