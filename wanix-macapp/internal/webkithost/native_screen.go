package webkithost

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/coregraphics"
	"github.com/tmc/apple/coreimage"
	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/screencapturekit"
)

type screenDisplay struct {
	ID     uint32  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type screenWindow struct {
	ID       uint32  `json:"id"`
	Title    string  `json:"title,omitempty"`
	AppName  string  `json:"appName,omitempty"`
	BundleID string  `json:"bundleID,omitempty"`
	PID      int32   `json:"pid,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	OnScreen bool    `json:"onScreen"`
	Active   bool    `json:"active"`
	Layer    int     `json:"layer"`
}

func (h *Host) applyScreenCtl(verb string) error {
	switch verb {
	case "request-auth", "refresh":
		return h.refreshScreenContent()
	default:
		return nil
	}
}

func (h *Host) applyScreenSession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	switch {
	case strings.HasPrefix(verb, "target "):
		target := strings.TrimSpace(strings.TrimPrefix(verb, "target "))
		if err := h.appleFS.WriteFile("screen/"+id+"/target", []byte(target+"\n")); err != nil {
			return err
		}
		return h.appleFS.WriteFile("screen/"+id+"/status", []byte("status configured\n"))
	case strings.HasPrefix(verb, "fps "):
		fps, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(verb, "fps ")))
		if err != nil {
			return fmt.Errorf("parse screen fps: %w", err)
		}
		if err := h.appleFS.WriteFile("screen/"+id+"/fps", []byte(fmt.Sprintf("%d\n", clampStreamFPS(fps)))); err != nil {
			return err
		}
		return h.appleFS.WriteFile("screen/"+id+"/status", []byte("status configured\n"))
	case strings.HasPrefix(verb, "format "):
		return h.appleFS.WriteFile("screen/"+id+"/status", []byte("status configured\n"))
	case verb == "oneshot":
		return h.captureScreenOneShot(id)
	case verb == "start":
		return h.startScreenStream(id)
	case verb == "stop", verb == "destroy":
		h.stopScreenStream(id)
		return h.appleFS.WriteFile("screen/"+id+"/status", []byte("status idle\n"))
	default:
		return nil
	}
}

func (h *Host) refreshScreenContent() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	content, err := screencapturekit.GetSCShareableContentClass().GetShareableContent(ctx)
	if err != nil {
		_ = h.appleFS.WriteFile("screen/status", []byte("api screen\nstatus error\nauthorized false\nerror "+err.Error()+"\n"))
		return fmt.Errorf("screen shareable content: %w", err)
	}
	displays, windows := screenContentEntries(content)
	displayJSON, err := formatScreenJSON(displays)
	if err != nil {
		return err
	}
	windowJSON, err := formatScreenJSON(windows)
	if err != nil {
		return err
	}
	if err := h.appleFS.WriteFile("screen/displays", displayJSON); err != nil {
		return err
	}
	if err := h.appleFS.WriteFile("screen/windows", windowJSON); err != nil {
		return err
	}
	status := fmt.Sprintf("api screen\nstatus ok\nauthorized true\ndisplays %d\nwindows %d\n", len(displays), len(windows))
	return h.appleFS.WriteFile("screen/status", []byte(status))
}

func (h *Host) captureScreenOneShot(id string) error {
	target := strings.TrimSpace(readAppleFSString(h, "screen/"+id+"/target"))
	image, err := captureScreenPNG(target)
	if err != nil {
		_ = h.appleFS.WriteFile("screen/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	if err := h.appleFS.WriteFile("screen/"+id+"/frames", image); err != nil {
		return err
	}
	return h.appleFS.WriteFile("screen/"+id+"/status", []byte(fmt.Sprintf("status done\nbytes %d\n", len(image))))
}

func (h *Host) startScreenStream(id string) error {
	h.stopScreenStream(id)
	if err := h.appleFS.WriteFile("screen/"+id+"/frames", nil); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.screenMu.Lock()
	h.screenRuns[id] = cancel
	h.screenMu.Unlock()
	if err := h.appleFS.WriteFile("screen/"+id+"/status", []byte("status running\nframes 0\nbytes 0\n")); err != nil {
		h.stopScreenStream(id)
		return err
	}
	go h.runScreenStream(ctx, id)
	return nil
}

func (h *Host) stopScreenStream(id string) {
	h.screenMu.Lock()
	cancel := h.screenRuns[id]
	delete(h.screenRuns, id)
	h.screenMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (h *Host) runScreenStream(ctx context.Context, id string) {
	fps, err := strconv.Atoi(strings.TrimSpace(readAppleFSString(h, "screen/"+id+"/fps")))
	if err != nil {
		fps = 1
	}
	fps = clampStreamFPS(fps)
	interval := time.Second / time.Duration(fps)
	tick := time.NewTicker(interval)
	defer tick.Stop()

	var frames, bytes int
	for {
		select {
		case <-ctx.Done():
			_ = h.appleFS.WriteFile("screen/"+id+"/status", []byte(fmt.Sprintf("status stopped\nframes %d\nbytes %d\n", frames, bytes)))
			return
		case <-tick.C:
			target := strings.TrimSpace(readAppleFSString(h, "screen/"+id+"/target"))
			image, err := captureScreenPNG(target)
			if err != nil {
				_ = h.appleFS.WriteFile("screen/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
				return
			}
			record := screenFrameRecord(frames+1, image)
			if err := appendAppleFSFile(h, "screen/"+id+"/frames", record); err != nil {
				_ = h.appleFS.WriteFile("screen/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
				return
			}
			frames++
			bytes += len(image)
			_ = h.appleFS.WriteFile("screen/"+id+"/status", []byte(fmt.Sprintf("status running\nframes %d\nbytes %d\n", frames, bytes)))
		}
	}
}

func screenFrameRecord(n int, image []byte) []byte {
	header := []byte(fmt.Sprintf("frame %d bytes %d\n", n, len(image)))
	out := make([]byte, 0, len(header)+len(image)+1)
	out = append(out, header...)
	out = append(out, image...)
	out = append(out, '\n')
	return out
}

func captureScreenPNG(target string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	content, err := screencapturekit.GetSCShareableContentClass().GetShareableContent(ctx)
	if err != nil {
		return nil, fmt.Errorf("screen shareable content: %w", err)
	}
	filter, err := screenContentFilter(content, target)
	if err != nil {
		return nil, err
	}
	config := screencapturekit.NewSCStreamConfiguration()
	cgImage, err := screencapturekit.GetSCScreenshotManagerClass().CaptureImageWithFilterConfiguration(ctx, filter, config)
	if err != nil {
		return nil, fmt.Errorf("screen capture: %w", err)
	}
	if cgImage == 0 {
		return nil, fmt.Errorf("screen capture returned nil image")
	}
	defer corefoundation.CFRelease(corefoundation.CFTypeRef(cgImage))

	return encodeScreenPNG(cgImage)
}

func screenContentFilter(content *screencapturekit.SCShareableContent, target string) (screencapturekit.SCContentFilter, error) {
	kind, arg, ok := strings.Cut(target, " ")
	if !ok && strings.TrimSpace(target) == "" {
		display, ok := screenFindDisplay(content, 0)
		if !ok {
			return screencapturekit.SCContentFilter{}, fmt.Errorf("no displays available")
		}
		return screencapturekit.NewContentFilterWithDisplayExcludingWindows(display, nil), nil
	}
	if !ok {
		return screencapturekit.SCContentFilter{}, fmt.Errorf("bad screen target %q", target)
	}
	n, err := strconv.ParseUint(strings.TrimSpace(arg), 10, 32)
	if err != nil {
		return screencapturekit.SCContentFilter{}, fmt.Errorf("bad screen target %q", target)
	}
	switch strings.TrimSpace(kind) {
	case "display":
		display, ok := screenFindDisplay(content, uint32(n))
		if !ok {
			return screencapturekit.SCContentFilter{}, fmt.Errorf("display %d not found", n)
		}
		return screencapturekit.NewContentFilterWithDisplayExcludingWindows(display, nil), nil
	case "window":
		window, ok := screenFindWindow(content, uint32(n))
		if !ok {
			return screencapturekit.SCContentFilter{}, fmt.Errorf("window %d not found", n)
		}
		return screencapturekit.NewContentFilterWithDesktopIndependentWindow(window), nil
	default:
		return screencapturekit.SCContentFilter{}, fmt.Errorf("bad screen target %q", target)
	}
}

func screenFindDisplay(content *screencapturekit.SCShareableContent, id uint32) (screencapturekit.SCDisplay, bool) {
	displays := content.Displays()
	if id == 0 && len(displays) > 0 {
		return displays[0], true
	}
	for _, d := range displays {
		if d.DisplayID() == id {
			return d, true
		}
	}
	return screencapturekit.SCDisplay{}, false
}

func screenFindWindow(content *screencapturekit.SCShareableContent, id uint32) (screencapturekit.SCWindow, bool) {
	for _, w := range content.Windows() {
		if w.WindowID() == id {
			return w, true
		}
	}
	return screencapturekit.SCWindow{}, false
}

func encodeScreenPNG(cgImage coregraphics.CGImageRef) ([]byte, error) {
	ciImage := coreimage.NewImageWithCGImage(cgImage)
	if ciImage.ID == 0 {
		return nil, fmt.Errorf("create screen image")
	}
	ciCtx := coreimage.NewCIContext()
	colorSpace := coregraphics.CGColorSpaceCreateDeviceRGB()

	dir, err := os.MkdirTemp("", "wanix-screen-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	name := filepath.Join(dir, "frame.png")
	url := foundation.NewURLFileURLWithPath(name)
	ok, writeErr := ciCtx.WritePNGRepresentationOfImageToURLFormatColorSpaceOptionsError(ciImage, url, 24, colorSpace, nil)
	if writeErr != nil {
		return nil, fmt.Errorf("write screen png: %w", writeErr)
	}
	if !ok {
		return nil, fmt.Errorf("write screen png failed")
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read screen png: %w", err)
	}
	return data, nil
}

func screenContentEntries(content *screencapturekit.SCShareableContent) ([]screenDisplay, []screenWindow) {
	if content == nil {
		return nil, nil
	}
	displays := make([]screenDisplay, 0, len(content.Displays()))
	for _, d := range content.Displays() {
		frame := d.Frame()
		displays = append(displays, screenDisplay{
			ID:     d.DisplayID(),
			X:      frame.Origin.X,
			Y:      frame.Origin.Y,
			Width:  frame.Size.Width,
			Height: frame.Size.Height,
		})
	}
	windows := make([]screenWindow, 0, len(content.Windows()))
	for _, w := range content.Windows() {
		frame := w.Frame()
		win := screenWindow{
			ID:       w.WindowID(),
			Title:    w.Title(),
			X:        frame.Origin.X,
			Y:        frame.Origin.Y,
			Width:    frame.Size.Width,
			Height:   frame.Size.Height,
			OnScreen: w.IsOnScreen(),
			Active:   w.IsActive(),
			Layer:    w.WindowLayer(),
		}
		if app, ok := w.OwningApplication().(screencapturekit.SCRunningApplication); ok && app.ID != 0 {
			win.AppName = app.ApplicationName()
			win.BundleID = app.BundleIdentifier()
			win.PID = app.ProcessID()
		}
		windows = append(windows, win)
	}
	return displays, windows
}

func formatScreenJSON(v any) ([]byte, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
