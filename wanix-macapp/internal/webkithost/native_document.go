package webkithost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/pdfkit"
)

type pdfTextPage struct {
	Page uint   `json:"page"`
	Text string `json:"text"`
}

type pdfMeta struct {
	PageCount      uint   `json:"page_count"`
	Encrypted      bool   `json:"encrypted"`
	Locked         bool   `json:"locked"`
	AllowsCopying  bool   `json:"allows_copying"`
	AllowsPrinting bool   `json:"allows_printing"`
	Attributes     string `json:"attributes,omitempty"`
}

func (h *Host) applyDocumentSession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	head := verb
	if i := strings.IndexByte(verb, ' '); i >= 0 {
		head = verb[:i]
	}
	switch head {
	case "format":
		return h.appleFS.WriteFile("document/"+id+"/format", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "format"))+"\n"))
	case "pages":
		return h.appleFS.WriteFile("document/"+id+"/pages", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "pages"))+"\n"))
	case "pdf-text":
		return h.appleFS.WriteFile("document/"+id+"/status", []byte("status configured\n"))
	}
	if head != "run" {
		return nil
	}
	input, err := h.appleFS.ReadFile("document/" + id + "/in")
	if err != nil {
		return err
	}
	pages := strings.TrimSpace(readAppleFSString(h, "document/"+id+"/pages"))
	format := strings.TrimSpace(readAppleFSString(h, "document/"+id+"/format"))
	text, err := extractPDFText(input, pages)
	if err != nil {
		_ = h.appleFS.WriteFile("document/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	out, err := formatPDFText(text, format)
	if err != nil {
		_ = h.appleFS.WriteFile("document/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	if err := h.appleFS.WriteFile("document/"+id+"/out", out); err != nil {
		return err
	}
	meta, err := extractPDFMeta(input)
	if err != nil {
		_ = h.appleFS.WriteFile("document/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	metaText, err := formatPDFMeta(meta)
	if err != nil {
		return err
	}
	if err := h.appleFS.WriteFile("document/"+id+"/meta", metaText); err != nil {
		return err
	}
	return h.appleFS.WriteFile("document/"+id+"/status", []byte(fmt.Sprintf("status done\npages %d\n", len(text))))
}

func extractPDFText(data []byte, pages string) ([]pdfTextPage, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty document")
	}
	nsdata := foundation.GetNSDataClass().Alloc().InitWithBytesLength(data)
	doc := pdfkit.NewPDFDocumentWithData(nsdata)
	if doc.ID == 0 {
		return nil, fmt.Errorf("open pdf")
	}
	pageCount := doc.PageCount()
	if pageCount == 0 {
		return nil, nil
	}
	indices, err := parsePageSelection(pages, pageCount)
	if err != nil {
		return nil, err
	}
	out := make([]pdfTextPage, 0, len(indices))
	for _, index := range indices {
		page := doc.PageAtIndex(index)
		if page == nil || page.GetID() == 0 {
			continue
		}
		out = append(out, pdfTextPage{Page: index + 1, Text: page.String()})
	}
	return out, nil
}

func extractPDFMeta(data []byte) (pdfMeta, error) {
	if len(data) == 0 {
		return pdfMeta{}, fmt.Errorf("empty document")
	}
	nsdata := foundation.GetNSDataClass().Alloc().InitWithBytesLength(data)
	doc := pdfkit.NewPDFDocumentWithData(nsdata)
	if doc.ID == 0 {
		return pdfMeta{}, fmt.Errorf("open pdf")
	}
	meta := pdfMeta{
		PageCount:      doc.PageCount(),
		Encrypted:      doc.IsEncrypted(),
		Locked:         doc.IsLocked(),
		AllowsCopying:  doc.AllowsCopying(),
		AllowsPrinting: doc.AllowsPrinting(),
	}
	if attrs := doc.DocumentAttributes(); attrs != nil && attrs.GetID() != 0 {
		meta.Attributes = attrs.Description()
	}
	return meta, nil
}

func formatPDFMeta(meta pdfMeta) ([]byte, error) {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func formatPDFText(pages []pdfTextPage, format string) ([]byte, error) {
	switch strings.TrimSpace(format) {
	case "", "text":
		var b bytes.Buffer
		for i, page := range pages {
			if i > 0 && !strings.HasSuffix(b.String(), "\n") {
				b.WriteByte('\n')
			}
			b.WriteString(page.Text)
			if !strings.HasSuffix(page.Text, "\n") {
				b.WriteByte('\n')
			}
		}
		return b.Bytes(), nil
	case "json":
		data, err := json.MarshalIndent(pages, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
}

func parsePageSelection(s string, pageCount uint) ([]uint, error) {
	if strings.TrimSpace(s) == "" {
		pages := make([]uint, pageCount)
		for i := range pages {
			pages[i] = uint(i)
		}
		return pages, nil
	}
	var pages []uint
	for _, field := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
		if field == "" {
			continue
		}
		start, end, ok := strings.Cut(field, "-")
		if !ok {
			end = start
		}
		first, err := parsePDFPage(start, pageCount)
		if err != nil {
			return nil, err
		}
		last, err := parsePDFPage(end, pageCount)
		if err != nil {
			return nil, err
		}
		if last < first {
			return nil, fmt.Errorf("invalid page range %q", field)
		}
		for page := first; page <= last; page++ {
			pages = append(pages, page-1)
		}
	}
	return pages, nil
}

func parsePDFPage(s string, pageCount uint) (uint, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse page %q: %w", s, err)
	}
	if n == 0 || n > uint64(pageCount) {
		return 0, fmt.Errorf("page %d out of range 1-%d", n, pageCount)
	}
	return uint(n), nil
}
