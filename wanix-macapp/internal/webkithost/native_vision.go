package webkithost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/vision"
)

type visionTextLine struct {
	Text       string  `json:"text"`
	Confidence float32 `json:"confidence"`
}

type visionBarcode struct {
	Payload    string  `json:"payload"`
	Symbology  string  `json:"symbology"`
	Confidence float32 `json:"confidence"`
}

type visionClassification struct {
	Identifier string  `json:"identifier"`
	Confidence float32 `json:"confidence"`
}

type visionPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type visionRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type visionRectangle struct {
	Confidence  float32     `json:"confidence"`
	BoundingBox visionRect  `json:"bounding_box"`
	TopLeft     visionPoint `json:"top_left"`
	TopRight    visionPoint `json:"top_right"`
	BottomLeft  visionPoint `json:"bottom_left"`
	BottomRight visionPoint `json:"bottom_right"`
}

func (h *Host) applyVisionSession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	head := verb
	if i := strings.IndexByte(verb, ' '); i >= 0 {
		head = verb[:i]
	}
	switch head {
	case "format":
		return h.appleFS.WriteFile("vision/"+id+"/format", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "format"))+"\n"))
	case "lang":
		return h.appleFS.WriteFile("vision/"+id+"/lang", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "lang"))+"\n"))
	case "ocr":
		if err := h.appleFS.WriteFile("vision/"+id+"/request", []byte("ocr\n")); err != nil {
			return err
		}
		if level := strings.TrimSpace(strings.TrimPrefix(verb, "ocr")); level != "" {
			return h.appleFS.WriteFile("vision/"+id+"/level", []byte(level+"\n"))
		}
	case "barcode":
		return h.appleFS.WriteFile("vision/"+id+"/request", []byte("barcode\n"))
	case "classify":
		return h.appleFS.WriteFile("vision/"+id+"/request", []byte("classify\n"))
	case "rectangles":
		return h.appleFS.WriteFile("vision/"+id+"/request", []byte("rectangles\n"))
	}
	if head != "run" && head != "ocr" && head != "barcode" && head != "classify" && head != "rectangles" {
		return nil
	}
	input, err := h.appleFS.ReadFile("vision/" + id + "/in")
	if err != nil {
		return err
	}
	request := strings.TrimSpace(readAppleFSString(h, "vision/"+id+"/request"))
	lang := strings.TrimSpace(readAppleFSString(h, "vision/"+id+"/lang"))
	level := strings.TrimSpace(readAppleFSString(h, "vision/"+id+"/level"))
	format := strings.TrimSpace(readAppleFSString(h, "vision/"+id+"/format"))
	var out []byte
	var countName string
	var count int
	switch request {
	case "", "ocr":
		lines, err := recognizeText(input, lang, level)
		if err != nil {
			_ = h.appleFS.WriteFile("vision/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
			return err
		}
		out, err = formatVisionText(lines, format)
		countName = "lines"
		count = len(lines)
	case "barcode":
		barcodes, err := detectBarcodes(input)
		if err != nil {
			_ = h.appleFS.WriteFile("vision/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
			return err
		}
		out, err = formatVisionBarcodes(barcodes, format)
		countName = "barcodes"
		count = len(barcodes)
	case "classify":
		classes, err := classifyImage(input)
		if err != nil {
			_ = h.appleFS.WriteFile("vision/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
			return err
		}
		out, err = formatVisionClassifications(classes, format)
		countName = "classes"
		count = len(classes)
	case "rectangles":
		rectangles, err := detectRectangles(input)
		if err != nil {
			_ = h.appleFS.WriteFile("vision/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
			return err
		}
		out, err = formatVisionRectangles(rectangles, format)
		countName = "rectangles"
		count = len(rectangles)
	default:
		err := fmt.Errorf("unknown request %q", request)
		_ = h.appleFS.WriteFile("vision/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	if err != nil {
		_ = h.appleFS.WriteFile("vision/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	if err := h.appleFS.WriteFile("vision/"+id+"/out", out); err != nil {
		return err
	}
	return h.appleFS.WriteFile("vision/"+id+"/status", []byte(fmt.Sprintf("status done\n%s %d\n", countName, count)))
}

func recognizeText(image []byte, lang, level string) ([]visionTextLine, error) {
	if len(image) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	data := foundation.GetNSDataClass().Alloc().InitWithBytesLength(image)
	handler := vision.NewImageRequestHandlerWithDataOptions(data, nil)
	request := vision.NewVNRecognizeTextRequest()
	switch level {
	case "", "accurate":
		request.SetRecognitionLevel(vision.VNRequestTextRecognitionLevelAccurate)
	case "fast":
		request.SetRecognitionLevel(vision.VNRequestTextRecognitionLevelFast)
	default:
		return nil, fmt.Errorf("unknown recognition level %q", level)
	}
	request.SetUsesLanguageCorrection(true)
	if langs := fieldsCSV(lang); len(langs) > 0 {
		request.SetRecognitionLanguages(langs)
	}
	if _, err := handler.PerformRequestsError([]vision.VNRequest{request.VNRequest}); err != nil {
		return nil, fmt.Errorf("perform ocr: %w", err)
	}
	var lines []visionTextLine
	for _, obs := range request.Results() {
		textObs := vision.VNRecognizedTextObservationFromID(obs.GetID())
		candidates := textObs.TopCandidates(1)
		if len(candidates) == 0 {
			continue
		}
		c := candidates[0]
		lines = append(lines, visionTextLine{Text: c.String(), Confidence: float32(c.Confidence())})
	}
	return lines, nil
}

func detectBarcodes(image []byte) ([]visionBarcode, error) {
	if len(image) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	data := foundation.GetNSDataClass().Alloc().InitWithBytesLength(image)
	handler := vision.NewImageRequestHandlerWithDataOptions(data, nil)
	request := vision.NewVNDetectBarcodesRequest()
	if _, err := handler.PerformRequestsError([]vision.VNRequest{request.VNImageBasedRequest.VNRequest}); err != nil {
		return nil, fmt.Errorf("perform barcode: %w", err)
	}
	var barcodes []visionBarcode
	for _, obs := range request.Results() {
		barcode := vision.VNBarcodeObservationFromID(obs.GetID())
		barcodes = append(barcodes, visionBarcode{
			Payload:    barcode.PayloadStringValue(),
			Symbology:  string(barcode.Symbology()),
			Confidence: float32(barcode.Confidence()),
		})
	}
	return barcodes, nil
}

func classifyImage(image []byte) ([]visionClassification, error) {
	if len(image) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	data := foundation.GetNSDataClass().Alloc().InitWithBytesLength(image)
	handler := vision.NewImageRequestHandlerWithDataOptions(data, nil)
	request := vision.NewVNClassifyImageRequest()
	if _, err := handler.PerformRequestsError([]vision.VNRequest{request.VNImageBasedRequest.VNRequest}); err != nil {
		return nil, fmt.Errorf("perform classify: %w", err)
	}
	var classes []visionClassification
	for _, obs := range request.Results() {
		class := vision.VNClassificationObservationFromID(obs.GetID())
		classes = append(classes, visionClassification{
			Identifier: class.Identifier(),
			Confidence: float32(class.Confidence()),
		})
	}
	return classes, nil
}

func detectRectangles(image []byte) ([]visionRectangle, error) {
	if len(image) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	data := foundation.GetNSDataClass().Alloc().InitWithBytesLength(image)
	handler := vision.NewImageRequestHandlerWithDataOptions(data, nil)
	request := vision.NewVNDetectRectanglesRequest()
	if _, err := handler.PerformRequestsError([]vision.VNRequest{request.VNImageBasedRequest.VNRequest}); err != nil {
		return nil, fmt.Errorf("perform rectangles: %w", err)
	}
	var rectangles []visionRectangle
	for _, obs := range request.Results() {
		rect := vision.VNRectangleObservationFromID(obs.GetID())
		rectangles = append(rectangles, visionRectangle{
			Confidence:  float32(rect.Confidence()),
			BoundingBox: visionRectFromCGRect(rect.BoundingBox()),
			TopLeft:     visionPointFromCGPoint(rect.TopLeft()),
			TopRight:    visionPointFromCGPoint(rect.TopRight()),
			BottomLeft:  visionPointFromCGPoint(rect.BottomLeft()),
			BottomRight: visionPointFromCGPoint(rect.BottomRight()),
		})
	}
	return rectangles, nil
}

func formatVisionText(lines []visionTextLine, format string) ([]byte, error) {
	switch strings.TrimSpace(format) {
	case "", "text":
		var b bytes.Buffer
		for _, line := range lines {
			b.WriteString(line.Text)
			b.WriteByte('\n')
		}
		return b.Bytes(), nil
	case "json":
		data, err := json.MarshalIndent(lines, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
}

func formatVisionClassifications(classes []visionClassification, format string) ([]byte, error) {
	switch strings.TrimSpace(format) {
	case "", "text":
		var b bytes.Buffer
		for _, class := range classes {
			b.WriteString(class.Identifier)
			b.WriteByte('\n')
		}
		return b.Bytes(), nil
	case "json":
		data, err := json.MarshalIndent(classes, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
}

func formatVisionRectangles(rectangles []visionRectangle, format string) ([]byte, error) {
	switch strings.TrimSpace(format) {
	case "", "text":
		var b bytes.Buffer
		for _, rect := range rectangles {
			fmt.Fprintf(&b, "%.6g %.6g %.6g %.6g %.6g\n",
				rect.BoundingBox.X, rect.BoundingBox.Y, rect.BoundingBox.Width, rect.BoundingBox.Height, rect.Confidence)
		}
		return b.Bytes(), nil
	case "json":
		data, err := json.MarshalIndent(rectangles, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
}

func formatVisionBarcodes(barcodes []visionBarcode, format string) ([]byte, error) {
	switch strings.TrimSpace(format) {
	case "", "text":
		var b bytes.Buffer
		for _, barcode := range barcodes {
			b.WriteString(barcode.Payload)
			b.WriteByte('\n')
		}
		return b.Bytes(), nil
	case "json":
		data, err := json.MarshalIndent(barcodes, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
}

func visionPointFromCGPoint(p corefoundation.CGPoint) visionPoint {
	return visionPoint{X: p.X, Y: p.Y}
}

func visionRectFromCGRect(r corefoundation.CGRect) visionRect {
	return visionRect{
		X:      r.Origin.X,
		Y:      r.Origin.Y,
		Width:  r.Size.Width,
		Height: r.Size.Height,
	}
}

func fieldsCSV(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == '\t' })
	var out []string
	for _, field := range fields {
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}
