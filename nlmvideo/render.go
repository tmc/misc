package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// renderPDF renders the markdown bundle to PDF. When md2html and a
// PDF-capable churl are both on PATH it renders through Chrome
// (md2html → HTML, churl Page.printToPDF), which gives real typography
// and clickable links; otherwise it falls back to the built-in writer.
// Either way every page carries a text layer, which NotebookLM's
// converter requires.
func renderPDF(bundleDir, mdPath, pdfPath, id string, frames []frame, buckets map[int]string) error {
	if err := renderChrome(bundleDir, mdPath, pdfPath); err != nil {
		fmt.Fprintf(os.Stderr, "nlmvideo: chrome render unavailable (%v); using built-in writer\n", err)
		return writeFramesPDF(pdfPath, id, frames, buckets)
	}
	return nil
}

func renderChrome(bundleDir, mdPath, pdfPath string) error {
	if _, err := exec.LookPath("md2html"); err != nil {
		return err
	}
	churl, err := exec.LookPath("churl")
	if err != nil {
		return err
	}
	htmlDir, err := os.MkdirTemp("", "nlmvideo-html-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(htmlDir)

	cmd := exec.Command("md2html", "-html", htmlDir, ".")
	cmd.Dir = bundleDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("md2html: %w: %s", err, out)
	}
	// md2html does not copy non-markdown assets; the images live in
	// the bundle's frames/ directory.
	if err := os.Symlink(filepath.Join(bundleDir, "frames"), filepath.Join(htmlDir, "frames")); err != nil && !os.IsExist(err) {
		return err
	}
	base := filepath.Base(mdPath)
	page := filepath.Join(htmlDir, base[:len(base)-len(".md")]+".html")
	if _, err := os.Stat(page); err != nil {
		return fmt.Errorf("md2html produced no %s", page)
	}
	out, err := exec.Command(churl, "-output-format", "pdf", "-o", pdfPath, "file://"+page).CombinedOutput()
	if err != nil {
		return fmt.Errorf("churl: %w: %s", err, out)
	}
	return nil
}
