package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tmc/apple/x/vzkit/ocr"
)

// stripeKeyPattern matches Stripe test keys with some tolerance for OCR
// artifacts (underscores rendered as spaces, optional separators). It
// requires a key body of at least 16 alphanumeric characters so ordinary
// prose like "desktop testing" or "backpack test" does not match.
const stripeKeyPattern = `(?i)\b[ps]k[_ ]?test[_ ]?[A-Za-z0-9]{16,}\b`

type patternList []string

func (p *patternList) String() string     { return strings.Join(*p, ",") }
func (p *patternList) Set(s string) error { *p = append(*p, s); return nil }

var (
	outputFlag   = flag.String("o", "", "Output path. Defaults to INPUT.redacted.png")
	inPlaceFlag  = flag.Bool("in-place", false, "Overwrite the input file")
	paddingFlag  = flag.Int("padding", 8, "Padding in pixels around each OCR match")
	styleFlag    = flag.String("style", "blur", "Redaction style: blur or solid")
	fillFlag     = flag.String("fill", "#000000", "Fill color for redaction rectangles")
	blurFlag     = flag.Int("blur", 10, "Blur strength in pixels for -style blur")
	verboseFlag  = flag.Bool("v", false, "Print OCR matches")
	patternsFlag patternList
	builtinFlag  = flag.Bool("builtin", true, "Include the built-in Stripe test key pattern")
)

func init() {
	flag.Var(&patternsFlag, "pattern", "Regexp to match sensitive OCR text. Repeat for multiple. Combined with the built-in pattern unless -builtin=false.")
}

func main() {
	flag.Parse()
	if err := run(flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "imgredact: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: imgredact [flags] image")
	}
	if *outputFlag != "" && *inPlaceFlag {
		return fmt.Errorf("-o and -in-place are mutually exclusive")
	}
	if err := validateStyle(*styleFlag); err != nil {
		return err
	}

	inPath := args[0]
	outPath := *outputFlag
	if *inPlaceFlag {
		outPath = inPath
	}
	if outPath == "" {
		outPath = defaultOutputPath(inPath)
	}

	fill, err := parseHexColor(*fillFlag)
	if err != nil {
		return err
	}

	patterns, err := compilePatterns(patternsFlag, *builtinFlag)
	if err != nil {
		return err
	}

	img, err := loadImage(inPath)
	if err != nil {
		return err
	}

	svc := ocr.NewService(*verboseFlag)
	observations, err := svc.RecognizeText(img)
	if err != nil {
		return fmt.Errorf("ocr %s: %w", inPath, err)
	}

	rects := matchedRects(observations, img.Bounds(), *paddingFlag, patterns)
	if len(rects) == 0 {
		return fmt.Errorf("no matching sensitive text found in %s", inPath)
	}

	dst := cloneRGBA(img)
	for _, r := range mergeRects(rects) {
		applyRedaction(dst, r, fill)
	}

	if err := writePNG(outPath, dst); err != nil {
		return err
	}
	return nil
}

func validateStyle(s string) error {
	switch s {
	case "blur", "solid":
		return nil
	default:
		return fmt.Errorf("invalid -style %q", s)
	}
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}

func defaultOutputPath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + ".redacted.png"
}

func compilePatterns(custom []string, builtin bool) ([]*regexp.Regexp, error) {
	var raws []string
	if builtin {
		raws = append(raws, stripeKeyPattern)
	}
	raws = append(raws, custom...)
	var res []*regexp.Regexp
	for _, p := range raws {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("compile pattern %q: %w", p, err)
		}
		res = append(res, re)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("no patterns: -pattern required when -builtin=false")
	}
	return res, nil
}

func matchedRects(observations []ocr.TextObservation, bounds image.Rectangle, padding int, patterns []*regexp.Regexp) []image.Rectangle {
	var rects []image.Rectangle
	for _, obs := range observations {
		if !isSensitive(obs.Text, patterns) {
			continue
		}
		rects = append(rects, paddedRect(observationRect(obs, bounds), bounds, padding))
	}
	return rects
}

func isSensitive(s string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func observationRect(obs ocr.TextObservation, bounds image.Rectangle) image.Rectangle {
	w, h := bounds.Dx(), bounds.Dy()
	x1 := int(obs.BoundingBox.Origin.X * float64(w))
	y1 := int((1 - obs.BoundingBox.Origin.Y - obs.BoundingBox.Size.Height) * float64(h))
	x2 := int((obs.BoundingBox.Origin.X + obs.BoundingBox.Size.Width) * float64(w))
	y2 := int((1 - obs.BoundingBox.Origin.Y) * float64(h))
	return image.Rect(x1, y1, x2, y2).Canon()
}

func paddedRect(r, bounds image.Rectangle, padding int) image.Rectangle {
	if padding < 0 {
		padding = 0
	}
	r = image.Rect(r.Min.X-padding, r.Min.Y-padding, r.Max.X+padding, r.Max.Y+padding)
	return r.Intersect(bounds)
}

func mergeRects(rects []image.Rectangle) []image.Rectangle {
	if len(rects) < 2 {
		return rects
	}
	merged := append([]image.Rectangle(nil), rects...)
	changed := true
	for changed {
		changed = false
		var next []image.Rectangle
		for len(merged) > 0 {
			r := merged[0]
			merged = merged[1:]
			i := 0
			for i < len(merged) {
				if touches(r, merged[i]) {
					r = r.Union(merged[i])
					merged = append(merged[:i], merged[i+1:]...)
					changed = true
					continue
				}
				i++
			}
			next = append(next, r)
		}
		merged = next
	}
	return merged
}

func touches(a, b image.Rectangle) bool {
	return a.Inset(-1).Overlaps(b.Inset(-1))
}

func cloneRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func applyRedaction(dst *image.RGBA, r image.Rectangle, fill color.Color) {
	switch *styleFlag {
	case "solid":
		draw.Draw(dst, r, &image.Uniform{C: fill}, image.Point{}, draw.Src)
	default:
		applyBlurRedaction(dst, r, fill, *blurFlag)
	}
}

func applyBlurRedaction(dst *image.RGBA, r image.Rectangle, fill color.Color, blur int) {
	if blur < 1 {
		blur = 1
	}
	src := cloneSubImage(dst, r)
	pixelate(src, blur)
	boxBlur(src, blur)
	draw.Draw(dst, r, src, src.Bounds().Min, draw.Src)
	applyTint(dst, r, fill, 80)
}

func cloneSubImage(src *image.RGBA, r image.Rectangle) *image.RGBA {
	sub := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(sub, sub.Bounds(), src, r.Min, draw.Src)
	return sub
}

func pixelate(img *image.RGBA, size int) {
	if size < 1 {
		size = 1
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += size {
		for x := b.Min.X; x < b.Max.X; x += size {
			block := image.Rect(x, y, min(x+size, b.Max.X), min(y+size, b.Max.Y))
			c := averageColor(img, block)
			draw.Draw(img, block, &image.Uniform{C: c}, image.Point{}, draw.Src)
		}
	}
}

func boxBlur(img *image.RGBA, radius int) {
	if radius < 1 {
		return
	}
	src := cloneRGBA(img)
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			block := image.Rect(max(x-radius, b.Min.X), max(y-radius, b.Min.Y), min(x+radius+1, b.Max.X), min(y+radius+1, b.Max.Y))
			img.Set(x, y, averageColor(src, block))
		}
	}
}

func averageColor(img image.Image, r image.Rectangle) color.RGBA {
	if r.Empty() {
		return color.RGBA{}
	}
	var rs, gs, bs, as uint64
	var n uint64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			rr, gg, bb, aa := img.At(x, y).RGBA()
			rs += uint64(rr >> 8)
			gs += uint64(gg >> 8)
			bs += uint64(bb >> 8)
			as += uint64(aa >> 8)
			n++
		}
	}
	return color.RGBA{
		R: uint8(rs / n),
		G: uint8(gs / n),
		B: uint8(bs / n),
		A: uint8(as / n),
	}
}

func applyTint(dst *image.RGBA, r image.Rectangle, fill color.Color, alpha uint8) {
	rr, gg, bb, _ := fill.RGBA()
	mask := color.RGBA{R: uint8(rr >> 8), G: uint8(gg >> 8), B: uint8(bb >> 8), A: alpha}
	draw.DrawMask(dst, r, &image.Uniform{C: mask}, image.Point{}, &image.Uniform{C: color.Alpha{A: alpha}}, image.Point{}, draw.Over)
}

func parseHexColor(s string) (color.Color, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return nil, fmt.Errorf("invalid fill color %q", s)
	}
	var rgb [3]uint8
	for i := 0; i < 3; i++ {
		n, ok := fromHexByte(s[i*2 : i*2+2])
		if !ok {
			return nil, fmt.Errorf("invalid fill color %q", s)
		}
		rgb[i] = n
	}
	return color.RGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 255}, nil
}

func fromHexByte(s string) (uint8, bool) {
	var n uint8
	for i := 0; i < len(s); i++ {
		n <<= 4
		switch c := s[i]; {
		case '0' <= c && c <= '9':
			n |= c - '0'
		case 'a' <= c && c <= 'f':
			n |= c - 'a' + 10
		case 'A' <= c && c <= 'F':
			n |= c - 'A' + 10
		default:
			return 0, false
		}
	}
	return n, true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
