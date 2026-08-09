package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/vision"
)

// detectFaces returns the bounding box of each face Vision detects in img.
// Boxes use normalized coordinates (0-1) with the origin at the bottom-left,
// matching ocr.TextObservation.BoundingBox.
func detectFaces(img image.Image) ([]corefoundation.CGRect, error) {
	if img == nil {
		return nil, fmt.Errorf("nil image")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	data := foundation.NewDataFromBytes(buf.Bytes())
	if data.ID == 0 {
		return nil, fmt.Errorf("create NSData from image bytes")
	}

	handler := vision.NewImageRequestHandlerWithDataOptions(data, nil)
	request := vision.NewVNDetectFaceRectanglesRequest()
	ok, err := handler.PerformRequestsError([]vision.VNRequest{
		vision.VNRequestFromID(request.ID),
	})
	if err != nil {
		return nil, fmt.Errorf("perform requests: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("perform requests returned false")
	}

	var boxes []corefoundation.CGRect
	for _, obs := range request.Results() {
		face := vision.VNFaceObservationFromID(obs.ID)
		boxes = append(boxes, face.BoundingBox())
	}
	return boxes, nil
}

// faceRects converts detected face boxes to pixel rectangles, grown by expand
// as a fraction of each box's own size. Vision returns boxes cropped close to
// the face, so the growth covers hair, chin, and neck.
func faceRects(boxes []corefoundation.CGRect, bounds image.Rectangle, expand float64) []image.Rectangle {
	var rects []image.Rectangle
	for _, box := range boxes {
		r := expandRect(normRect(box, bounds), bounds, expand)
		if r.Empty() {
			continue
		}
		rects = append(rects, r)
	}
	return rects
}

// expandRect grows r by frac of its width and height on every side, clipped to
// bounds. A negative frac is treated as zero.
func expandRect(r, bounds image.Rectangle, frac float64) image.Rectangle {
	if frac < 0 {
		frac = 0
	}
	dx := int(float64(r.Dx()) * frac)
	dy := int(float64(r.Dy()) * frac)
	r = image.Rect(r.Min.X-dx, r.Min.Y-dy, r.Max.X+dx, r.Max.Y+dy)
	return r.Intersect(bounds)
}
