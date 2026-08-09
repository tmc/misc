// Command imgredact redacts sensitive text and faces from images.
//
// It uses Apple's Vision framework to detect text regions matching
// configured patterns and, with -faces, to detect faces, then blurs or
// paints over the matched regions.
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
//
// Use -faces to blur detected faces, and -text=false to redact faces only:
//
//	imgredact -faces -text=false /tmp/photo.jpg
//
// Face boxes from Vision hug the face, so imgredact grows each one by
// -face-expand (a fraction of the box's own size, 0.2 by default) to cover
// hair, chin, and neck.
package main
