package webkithost

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/apple/coregraphics"
	"github.com/tmc/apple/coreimage"
	"github.com/tmc/apple/foundation"
)

func (h *Host) applyImageSession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	head := verb
	if i := strings.IndexByte(verb, ' '); i >= 0 {
		head = verb[:i]
	}
	switch head {
	case "filter":
		return h.appleFS.WriteFile("image/"+id+"/filter", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "filter"))+"\n"))
	case "format":
		return h.appleFS.WriteFile("image/"+id+"/format", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "format"))+"\n"))
	}
	if head != "run" {
		return nil
	}
	input, err := h.appleFS.ReadFile("image/" + id + "/in")
	if err != nil {
		return err
	}
	filter := strings.TrimSpace(readAppleFSString(h, "image/"+id+"/filter"))
	format := strings.TrimSpace(readAppleFSString(h, "image/"+id+"/format"))
	out, err := transformImage(input, filter, format)
	if err != nil {
		_ = h.appleFS.WriteFile("image/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	if err := h.appleFS.WriteFile("image/"+id+"/out", out); err != nil {
		return err
	}
	return h.appleFS.WriteFile("image/"+id+"/status", []byte(fmt.Sprintf("status done\nbytes %d\n", len(out))))
}

func transformImage(data []byte, filter, format string) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	in, err := os.CreateTemp("", "wanix-image-in-*")
	if err != nil {
		return nil, fmt.Errorf("create temp input: %w", err)
	}
	inPath := in.Name()
	defer os.Remove(inPath)
	if _, err := in.Write(data); err != nil {
		in.Close()
		return nil, fmt.Errorf("write temp input: %w", err)
	}
	if err := in.Close(); err != nil {
		return nil, fmt.Errorf("close temp input: %w", err)
	}

	img := coreimage.NewImageWithContentsOfURL(foundation.NewURLFileURLWithPath(inPath))
	if img.ID == 0 {
		return nil, fmt.Errorf("load image")
	}
	output, err := applyImageFilter(img, filter)
	if err != nil {
		return nil, err
	}
	return renderImage(output, format)
}

func applyImageFilter(img coreimage.CIImage, filter string) (coreimage.ICIImage, error) {
	name := strings.TrimSpace(filter)
	if name == "" {
		return img, nil
	}
	switch strings.Fields(name)[0] {
	case "grayscale":
		f := coreimage.NewFilterWithName("CIColorControls")
		if f.ID == 0 {
			return nil, fmt.Errorf("CIColorControls unavailable")
		}
		f.SetValueForKey(img, "inputImage")
		f.SetValueForKey(foundation.NewNumberWithDouble(0), "inputSaturation")
		out := f.OutputImage()
		if out == nil {
			return nil, fmt.Errorf("grayscale produced no output")
		}
		return out.ImageByCroppingToRect(img.Extent()), nil
	case "sepia":
		f := coreimage.NewFilterWithName("CISepiaTone")
		if f.ID == 0 {
			return nil, fmt.Errorf("CISepiaTone unavailable")
		}
		f.SetValueForKey(img, "inputImage")
		f.SetValueForKey(foundation.NewNumberWithDouble(1), "inputIntensity")
		out := f.OutputImage()
		if out == nil {
			return nil, fmt.Errorf("sepia produced no output")
		}
		return out.ImageByCroppingToRect(img.Extent()), nil
	default:
		return nil, fmt.Errorf("unknown filter %q", filter)
	}
}

func renderImage(img coreimage.ICIImage, format string) ([]byte, error) {
	ext := ".png"
	switch strings.TrimSpace(format) {
	case "", "png":
		ext = ".png"
	case "jpeg", "jpg":
		ext = ".jpg"
	case "tiff":
		ext = ".tiff"
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
	out, err := os.CreateTemp("", "wanix-image-out-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("create temp output: %w", err)
	}
	outPath := out.Name()
	out.Close()
	defer os.Remove(outPath)

	ctx := coreimage.NewCIContext()
	colorSpace := coregraphics.CGColorSpaceCreateDeviceRGB()
	url := foundation.NewURLFileURLWithPath(outPath)
	switch ext {
	case ".jpg":
		ok, err := ctx.WriteJPEGRepresentationOfImageToURLColorSpaceOptionsError(img, url, colorSpace, nil)
		if err != nil {
			return nil, fmt.Errorf("write jpeg: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("write jpeg failed")
		}
	case ".tiff":
		data := ctx.TIFFRepresentationOfImageFormatColorSpaceOptions(img, 24, colorSpace, nil)
		if data.ID == 0 {
			return nil, fmt.Errorf("write tiff failed")
		}
		if !data.WriteToFileAtomically(outPath, true) {
			return nil, fmt.Errorf("write tiff failed")
		}
	default:
		ok, err := ctx.WritePNGRepresentationOfImageToURLFormatColorSpaceOptionsError(img, url, 24, colorSpace, nil)
		if err != nil {
			return nil, fmt.Errorf("write png: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("write png failed")
		}
	}
	data, err := os.ReadFile(filepath.Clean(outPath))
	if err != nil {
		return nil, fmt.Errorf("read temp output: %w", err)
	}
	return data, nil
}
