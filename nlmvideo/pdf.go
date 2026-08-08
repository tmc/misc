package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// The PDF writer is deliberately minimal: letter pages, one baseline
// JPEG per page (DCTDecode pass-through), Helvetica text. NotebookLM's
// converter errors on image-only PDFs, so every page carries a real
// text layer: the timestamp, a ?t= deep link, and the spoken words.

const (
	pageW     = 612.0 // letter, points
	pageH     = 792.0
	marginPt  = 36.0
	headerPt  = 11.0
	bodyPt    = 9.0
	lineGapPt = 12.0
	wrapCols  = 100
)

// writeFramesPDF writes one page per frame: header with timestamp and
// deep link, the frame image, and the transcript bucket text below.
func writeFramesPDF(path, id string, frames []frame, buckets map[int]string) error {
	var w pdfWriter
	font := w.add([]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"), nil)

	var pageRefs []int
	for _, f := range frames {
		jpeg, err := os.ReadFile(f.Path)
		if err != nil {
			return err
		}
		jw, jh, err := jpegSize(jpeg)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Path, err)
		}
		img := w.add([]byte(fmt.Sprintf(
			"<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>",
			jw, jh, len(jpeg))), jpeg)

		imgW := pageW - 2*marginPt
		imgH := imgW * float64(jh) / float64(jw)
		imgY := pageH - marginPt - 2*lineGapPt - imgH

		ts := fmt.Sprintf("%d:%02d", int(f.Seconds)/60, int(f.Seconds)%60)
		var content bytes.Buffer
		fmt.Fprintf(&content, "BT /F1 %g Tf %g %g Td (%s) Tj ET\n",
			headerPt, marginPt, pageH-marginPt-headerPt,
			escapePDF(fmt.Sprintf("Frame @ %s  -  https://youtu.be/%s?t=%d", ts, id, int(f.Seconds))))
		fmt.Fprintf(&content, "q %g 0 0 %g %g %g cm /Im0 Do Q\n", imgW, imgH, marginPt, imgY)
		y := imgY - lineGapPt - bodyPt
		for _, line := range wrap(spokenAt(buckets, f.Seconds), wrapCols) {
			fmt.Fprintf(&content, "BT /F1 %g Tf %g %g Td (%s) Tj ET\n",
				bodyPt, marginPt, y, escapePDF(line))
			y -= lineGapPt
			if y < marginPt {
				break
			}
		}
		stream := w.add([]byte(fmt.Sprintf("<< /Length %d >>", content.Len())), content.Bytes())

		page := w.add([]byte(fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %g %g] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> /XObject << /Im0 %d 0 R >> >> >>",
			pageW, pageH, stream, font, img)), nil)
		pageRefs = append(pageRefs, page)
	}

	var kids []string
	for _, r := range pageRefs {
		kids = append(kids, fmt.Sprintf("%d 0 R", r))
	}
	w.set(2, []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pageRefs))))
	w.set(1, []byte("<< /Type /Catalog /Pages 2 0 R >>"))
	return os.WriteFile(path, w.bytes(), 0o644)
}

// pdfWriter accumulates numbered objects and serializes them with a
// correct xref table. Objects 1 (catalog) and 2 (pages) are reserved.
type pdfWriter struct {
	objs [][]byte // index 0 => object 1
}

// add appends an object with the given dictionary and optional stream,
// returning its object number.
func (w *pdfWriter) add(dict, stream []byte) int {
	if len(w.objs) == 0 {
		w.objs = make([][]byte, 2) // reserve catalog + pages
	}
	var b bytes.Buffer
	b.Write(dict)
	if stream != nil {
		b.WriteString("\nstream\n")
		b.Write(stream)
		b.WriteString("\nendstream")
	}
	w.objs = append(w.objs, b.Bytes())
	return len(w.objs)
}

// set fills in a reserved object number.
func (w *pdfWriter) set(num int, dict []byte) {
	w.objs[num-1] = dict
}

func (w *pdfWriter) bytes() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(w.objs))
	for i, obj := range w.objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n", i+1)
		b.Write(obj)
		b.WriteString("\nendobj\n")
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(w.objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(w.objs)+1, xref)
	return b.Bytes()
}

// escapePDF escapes characters special in PDF literal strings and
// replaces non-Latin-1 bytes, which base-14 Helvetica cannot encode.
func escapePDF(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if r > 255 || r < 32 {
				b.WriteByte('?')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// wrap splits s into lines of at most cols characters at word
// boundaries.
func wrap(s string, cols int) []string {
	var lines []string
	var cur []string
	n := 0
	for _, w := range strings.Fields(s) {
		if n > 0 && n+1+len(w) > cols {
			lines = append(lines, strings.Join(cur, " "))
			cur, n = nil, 0
		}
		cur = append(cur, w)
		n += len(w)
		if len(cur) > 1 {
			n++
		}
	}
	if len(cur) > 0 {
		lines = append(lines, strings.Join(cur, " "))
	}
	return lines
}

// jpegSize reads the pixel dimensions from a baseline or progressive
// JPEG's SOF marker.
func jpegSize(data []byte) (w, h int, err error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, 0, fmt.Errorf("not a JPEG")
	}
	i := 2
	for i+9 < len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD9) {
			i += 2
			continue
		}
		length := int(binary.BigEndian.Uint16(data[i+2:]))
		if marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC {
			h = int(binary.BigEndian.Uint16(data[i+5:]))
			w = int(binary.BigEndian.Uint16(data[i+7:]))
			return w, h, nil
		}
		i += 2 + length
	}
	return 0, 0, fmt.Errorf("no SOF marker")
}
