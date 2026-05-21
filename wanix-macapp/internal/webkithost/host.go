package webkithost

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmc/apple/appkit"
	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/objectivec"
	"github.com/tmc/apple/webkit"
)

// Config controls a WebKit-backed Wanix host.
type Config struct {
	AssetsDir    string
	RCPath       string
	URL          string
	Visible      bool
	Inspectable  bool
	ReadyTimeout time.Duration
}

// Host owns the WebKit view and native bridge objects.
type Host struct {
	cfg        Config
	window     appkit.NSWindow
	webView    webkit.WKWebView
	controller webkit.WKUserContentController
	bridge     webkit.WKScriptMessageHandlerObject
	server     *assetServer
	mainLoop   foundation.NSRunLoop
	ready      chan Message
	errors     chan Message
	pendingMu  sync.Mutex
	pending    map[string]chan Message
	commandID  atomic.Uint64
}

// New creates a host. It must be called on the AppKit main thread.
func New(cfg Config) *Host {
	if cfg.ReadyTimeout == 0 {
		cfg.ReadyTimeout = 15 * time.Second
	}
	h := &Host{
		cfg:      cfg,
		mainLoop: foundation.GetRunLoopClass().MainRunLoop(),
		ready:    make(chan Message, 1),
		errors:   make(chan Message, 16),
		pending:  make(map[string]chan Message),
	}
	h.bridge = NewBridge(h.handleMessage)

	h.controller = webkit.NewWKUserContentController()
	h.controller.AddScriptMessageHandlerName(h.bridge, "wanix")
	h.controller.AddUserScript(webkit.NewUserScriptWithSourceInjectionTimeForMainFrameOnly(
		NativeBridgeScript,
		webkit.WKUserScriptInjectionTimeAtDocumentStart,
		true,
	))

	wcfg := webkit.NewWKWebViewConfiguration()
	wcfg.SetUserContentController(h.controller)

	frame := corefoundation.CGRect{Size: corefoundation.CGSize{Width: 1100, Height: 760}}
	h.webView = webkit.GetWKWebViewClass().Alloc().InitWithFrameConfiguration(frame, wcfg)
	h.webView.SetInspectable(cfg.Inspectable)

	if cfg.Visible {
		h.window = appkit.GetNSWindowClass().Alloc().InitWithContentRectStyleMaskBackingDefer(
			frame,
			appkit.NSWindowStyleMaskTitled|appkit.NSWindowStyleMaskClosable|appkit.NSWindowStyleMaskMiniaturizable|appkit.NSWindowStyleMaskResizable,
			appkit.NSBackingStoreBuffered,
			false,
		)
		h.window.SetTitle("Wanix")
		h.window.SetContentView(h.webView)
		h.window.Center()
		h.window.MakeKeyAndOrderFront(nil)
	}

	return h
}

// Load starts the Wanix bootstrap document.
func (h *Host) Load() error {
	if h.cfg.AssetsDir == "" {
		if h.cfg.URL == "" {
			return fmt.Errorf("load: asset directory or URL required")
		}
		return h.LoadURL(h.cfg.URL)
	}
	server, err := startAssetServer(Assets{Dir: h.cfg.AssetsDir, RCPath: h.cfg.RCPath})
	if err != nil {
		return err
	}
	h.server = server
	url := foundation.NewURLWithString(server.URL())
	h.webView.LoadRequest(foundation.NewURLRequestWithURL(url))
	return nil
}

// LoadURL loads an existing Wanix web page.
func (h *Host) LoadURL(rawurl string) error {
	url := foundation.NewURLWithString(rawurl)
	h.webView.LoadRequest(foundation.NewURLRequestWithURL(url))
	return nil
}

// WaitReady waits for the runtime bootstrap to report readiness.
func (h *Host) WaitReady(ctx context.Context) (Message, error) {
	ctx, cancel := context.WithTimeout(ctx, h.cfg.ReadyTimeout)
	defer cancel()
	for {
		select {
		case msg := <-h.ready:
			return msg, nil
		case msg := <-h.errors:
			return Message{}, fmt.Errorf("runtime error: %s", msg.Message)
		case <-ctx.Done():
			return Message{}, fmt.Errorf("wanix system ready timeout: %w", ctx.Err())
		}
	}
}

// Eval evaluates JavaScript in the WebKit view.
func (h *Host) Eval(ctx context.Context, script string) (string, error) {
	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	h.performOnMain(func() {
		h.webView.EvaluateJavaScriptCompletionHandler(script, func(obj objectivec.IObject, err error) {
			if err != nil {
				done <- result{err: fmt.Errorf("evaluate javascript: %w", err)}
				return
			}
			if obj == nil || obj.GetID() == 0 {
				done <- result{}
				return
			}
			done <- result{value: objectivec.ObjectFromID(obj.GetID()).Description()}
		})
	})
	select {
	case r := <-done:
		return r.value, r.err
	case <-ctx.Done():
		return "", fmt.Errorf("evaluate javascript timeout: %w", ctx.Err())
	}
}

// EvalAsync evaluates an async JavaScript expression through the bridge.
func (h *Host) EvalAsync(ctx context.Context, script string) (string, error) {
	id := strconv.FormatUint(h.commandID.Add(1), 10)
	ch := h.registerPending(id)
	defer h.unregisterPending(id)
	wrapped := fmt.Sprintf(`(function() {
  Promise.resolve().then(async function() {
    return await (async function() { %s })();
  }).then(function(value) {
    window.webkit.messageHandlers.wanix.postMessage(JSON.stringify({type: "result", id: %q, value: String(value)}));
  }).catch(function(err) {
    window.webkit.messageHandlers.wanix.postMessage(JSON.stringify({type: "error", id: %q, message: String(err && (err.stack || err.message) || err)}));
  });
})()`, script, id, id)
	if _, err := h.Eval(ctx, wrapped); err != nil {
		return "", err
	}
	for {
		select {
		case msg := <-ch:
			if msg.Type == "error" {
				return "", fmt.Errorf("javascript command %s: %s", id, msg.Message)
			}
			if msg.Type == "result" {
				if s, ok := msg.Value.(string); ok {
					return s, nil
				}
				return fmt.Sprint(msg.Value), nil
			}
		case <-ctx.Done():
			return "", fmt.Errorf("javascript command %s timeout: %w", id, ctx.Err())
		}
	}
}

// Close releases local host resources.
func (h *Host) Close(ctx context.Context) error {
	if h.server == nil {
		return nil
	}
	return h.server.Close(ctx)
}

// SelfTest verifies the first WebKit substrate invariants.
func (h *Host) SelfTest(ctx context.Context) error {
	ready, err := h.WaitReady(ctx)
	if err != nil {
		return err
	}
	for _, name := range []string{"wasm", "worker", "messageChannel", "blob", "fetch", "sharedArrayBuffer", "crossOriginIsolated"} {
		if !ready.Capabilities[name] {
			return fmt.Errorf("%s unavailable", name)
		}
	}
	result, err := h.Eval(ctx, CapabilityProbe)
	if err != nil {
		return err
	}
	var caps map[string]bool
	if err := jsonUnmarshalString(result, &caps); err != nil {
		return err
	}
	if !caps["system"] {
		return fmt.Errorf("wanix system element unavailable")
	}
	if err := h.FileRoundTrip(ctx, "tmp/hello.txt", "hello from native webkit\n"); err != nil {
		return err
	}
	if err := h.FileByteRoundTrip(ctx, "tmp/bytes.bin", []byte{0, 1, 2, 127, 128, 255}); err != nil {
		return err
	}
	status, err := h.ReadFile(ctx, "mnt/macos/status")
	if err != nil {
		return fmt.Errorf("macos namespace status: %w", err)
	}
	if !bytes.Contains([]byte(status), []byte(`"api":"macos"`)) {
		return fmt.Errorf("macos namespace status: %q", status)
	}
	if err := h.WriteFile(ctx, "mnt/macos/window/0/title", "Wanix Native Self-Test\n"); err != nil {
		return fmt.Errorf("macos namespace title: %w", err)
	}
	return nil
}

// FileRoundTrip writes and reads a file through the Wanix root handle.
func (h *Host) FileRoundTrip(ctx context.Context, path, data string) error {
	if err := h.WriteFile(ctx, path, data); err != nil {
		return err
	}
	got, err := h.ReadFile(ctx, path)
	if err != nil {
		return err
	}
	if got != data {
		return fmt.Errorf("file round trip %s: got %q, want %q", path, got, data)
	}
	return nil
}

// FileByteRoundTrip writes and reads bytes through the Wanix root handle.
func (h *Host) FileByteRoundTrip(ctx context.Context, path string, data []byte) error {
	if err := h.WriteFileBytes(ctx, path, data); err != nil {
		return err
	}
	got, err := h.ReadFileBytes(ctx, path)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, data) {
		return fmt.Errorf("file byte round trip %s: got %x, want %x", path, got, data)
	}
	return nil
}

// WriteFile writes a UTF-8 string through the Wanix root handle.
func (h *Host) WriteFile(ctx context.Context, path, data string) error {
	return h.WriteFileBytes(ctx, path, []byte(data))
}

// WriteFileBytes writes bytes through the Wanix root handle.
func (h *Host) WriteFileBytes(ctx context.Context, path string, data []byte) error {
	name, err := jsString(path)
	if err != nil {
		return err
	}
	encoded, err := jsString(base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`
  const root = document.getElementById("system").root;
  const name = %s;
  const encoded = %s;
  const binary = atob(encoded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  if (!name.startsWith("mnt/macos/")) {
    await root.makeDirAll(name.split("/").slice(0, -1).join("/") || ".");
  }
  await root.writeFile(name, bytes);
  return "ok";
`, name, encoded)
	_, err = h.EvalAsync(ctx, script)
	return err
}

// ReadFile reads a UTF-8 string through the Wanix root handle.
func (h *Host) ReadFile(ctx context.Context, path string) (string, error) {
	data, err := h.ReadFileBytes(ctx, path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadFileBytes reads bytes through the Wanix root handle.
func (h *Host) ReadFileBytes(ctx context.Context, path string) ([]byte, error) {
	name, err := jsString(path)
	if err != nil {
		return nil, err
	}
	script := fmt.Sprintf(`
  const root = document.getElementById("system").root;
  const bytes = await root.readFile(%s);
  let binary = "";
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
  return btoa(binary);
`, name)
	encoded, err := h.EvalAsync(ctx, script)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode file %s: %w", path, err)
	}
	return data, nil
}

func (h *Host) handleMessage(msg Message) {
	if msg.Type == "native.call" {
		h.handleNativeCall(msg)
		return
	}
	if msg.ID != "" {
		h.pendingMu.Lock()
		ch := h.pending[msg.ID]
		h.pendingMu.Unlock()
		if ch != nil {
			ch <- msg
			return
		}
	}
	if msg.Type == "ready" {
		select {
		case h.ready <- msg:
		default:
		}
		return
	}
	if msg.Type == "error" {
		select {
		case h.errors <- msg:
		default:
		}
	}
}

func (h *Host) registerPending(id string) chan Message {
	ch := make(chan Message, 1)
	h.pendingMu.Lock()
	h.pending[id] = ch
	h.pendingMu.Unlock()
	return ch
}

func (h *Host) unregisterPending(id string) {
	h.pendingMu.Lock()
	delete(h.pending, id)
	h.pendingMu.Unlock()
}

func (h *Host) performOnMain(fn func()) {
	h.mainLoop.PerformBlock(fn)
}

func jsString(s string) (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal javascript string: %w", err)
	}
	return string(data), nil
}
