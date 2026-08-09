// Command imgredact redacts sensitive OCR-detected text from images.
//
// It uses Apple's Vision framework via github.com/tmc/apple/x/vzkit/ocr
// to detect text regions matching configured patterns, then paints opaque
// rectangles over the matched regions.
//
// The built-in pattern matches Stripe test keys (pk_test_... and
// sk_test_...) with some tolerance for OCR artifacts. Pass -pattern to
// add regexp matchers; pass -builtin=false to disable the default.
//
// By default, imgredact writes a PNG next to the input file:
//
//	imgredact /tmp/download.png
//
// which produces:
//
//	/tmp/download.redacted.png
//
// Use -o to choose an output path or -in-place to overwrite the input.
// Use -pattern to match additional regexps:
//
//	imgredact -pattern 'SECRET-\d+' /tmp/download.png
package main
