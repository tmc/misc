package webkithost

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmc/apple/appkit"
	"github.com/tmc/apple/applicationservices"
	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/objectivec"
	"github.com/tmc/apple/webkit"
	"github.com/tmc/misc/wanix-macapp/internal/applefs"
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
	cfg         Config
	window      appkit.NSWindow
	webView     webkit.WKWebView
	controller  webkit.WKUserContentController
	bridge      webkit.WKScriptMessageHandlerObject
	server      *assetServer
	mainLoop    foundation.NSRunLoop
	ready       chan Message
	errors      chan Message
	pendingMu   sync.Mutex
	pending     map[string]chan Message
	commandID   atomic.Uint64
	appleFS     *applefs.Root
	indicatorMu sync.Mutex
	indicators  map[string]appkit.NSStatusItem
	speechMu    sync.Mutex
	speech      map[string]speechSynth
	axMu        sync.Mutex
	axElements  map[string]applicationservices.AXUIElementRef
	vzMu        sync.Mutex
	vzVMs       map[string]vzSession
	micMu       sync.Mutex
	micStreams  map[string]context.CancelFunc
	screenMu    sync.Mutex
	screenRuns  map[string]context.CancelFunc
}

// New creates a host. It must be called on the AppKit main thread.
func New(cfg Config) *Host {
	if cfg.ReadyTimeout == 0 {
		cfg.ReadyTimeout = 15 * time.Second
	}
	h := &Host{
		cfg:        cfg,
		mainLoop:   foundation.GetRunLoopClass().MainRunLoop(),
		ready:      make(chan Message, 1),
		errors:     make(chan Message, 16),
		pending:    make(map[string]chan Message),
		appleFS:    applefs.NewRoot(),
		indicators: make(map[string]appkit.NSStatusItem),
		speech:     make(map[string]speechSynth),
		axElements: make(map[string]applicationservices.AXUIElementRef),
		vzVMs:      make(map[string]vzSession),
		micStreams: make(map[string]context.CancelFunc),
		screenRuns: make(map[string]context.CancelFunc),
	}
	h.bridge = NewBridge(h.handleMessage)
	h.appleFS.Host = nativeAppleHost{h: h}
	h.appleFS.OnWrite = func(name string, data []byte) {
		h.updateMacOSCache(name, data, false)
	}
	h.appleFS.OnAppend = func(name string, data []byte) {
		h.updateMacOSCache(name, data, true)
	}

	h.controller = webkit.NewWKUserContentController()
	h.controller.AddScriptMessageHandlerName(h.bridge, "wanix")
	h.controller.AddUserScript(webkit.NewUserScriptWithSourceInjectionTimeForMainFrameOnly(
		nativeBridgeScript(h.appleFS),
		webkit.WKUserScriptInjectionTimeAtDocumentStart,
		true,
	))

	wcfg := webkit.NewWKWebViewConfiguration()
	wcfg.SetUserContentController(h.controller)

	frame := corefoundation.CGRect{Size: corefoundation.CGSize{Width: 1100, Height: 760}}
	h.webView = webkit.GetWKWebViewClass().Alloc().InitWithFrameConfiguration(frame, wcfg)
	h.webView.SetInspectable(cfg.Inspectable)

	h.window = appkit.GetNSWindowClass().Alloc().InitWithContentRectStyleMaskBackingDefer(
		frame,
		appkit.NSWindowStyleMaskTitled|appkit.NSWindowStyleMaskClosable|appkit.NSWindowStyleMaskMiniaturizable|appkit.NSWindowStyleMaskResizable,
		appkit.NSBackingStoreBuffered,
		false,
	)
	h.window.SetTitle("Wanix")
	h.window.SetContentView(h.webView)
	h.window.Center()
	if cfg.Visible {
		h.window.MakeKeyAndOrderFront(nil)
	}

	return h
}

func nativeBridgeScript(root *applefs.Root) string {
	cloneData, err := json.Marshal(root.CloneSchemas())
	if err != nil {
		cloneData = []byte("{}")
	}
	initialData, err := json.Marshal(snapshotAppleFS(root, 3))
	if err != nil {
		initialData = []byte(`{"dirs":{},"files":{}}`)
	}
	script := strings.Replace(NativeBridgeScript, "__WANIX_CLONE_SCHEMAS__", string(cloneData), 1)
	return strings.Replace(script, "__WANIX_INITIAL_FS__", string(initialData), 1)
}

type appleFSSnapshot struct {
	Dirs  map[string][]applefs.Entry `json:"dirs"`
	Files map[string]string          `json:"files"`
}

func snapshotAppleFS(root *applefs.Root, depth int) appleFSSnapshot {
	snap := appleFSSnapshot{
		Dirs:  make(map[string][]applefs.Entry),
		Files: make(map[string]string),
	}
	snapshotAppleFSDir(root, snap, "", depth)
	return snap
}

func snapshotAppleFSDir(root *applefs.Root, snap appleFSSnapshot, name string, depth int) {
	readName := name
	if readName == "" {
		readName = "."
	}
	entries, err := root.ReadDir(readName)
	if err != nil {
		return
	}
	snap.Dirs[name] = entries
	for _, entry := range entries {
		child := entry.Name
		if name != "" {
			child = name + "/" + entry.Name
		}
		if entry.Dir {
			if depth > 0 {
				snapshotAppleFSDir(root, snap, child, depth-1)
			}
			continue
		}
		if entry.Name == "clone" {
			continue
		}
		data, err := root.ReadFile(child)
		if err == nil {
			snap.Files[child] = base64.StdEncoding.EncodeToString(data)
		}
	}
}

func (h *Host) updateMacOSCache(name string, data []byte, append bool) {
	if h.webView.GetID() == 0 {
		return
	}
	jsName, err := jsString(name)
	if err != nil {
		return
	}
	jsData, err := jsString(base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return
	}
	method := "_updateCache"
	if append {
		method = "_appendCache"
	}
	h.performOnMain(func() {
		h.webView.EvaluateJavaScriptCompletionHandler(
			`window.__wanixMacOSFS && window.__wanixMacOSFS.`+method+`(`+jsName+`, `+jsData+`)`,
			nil,
		)
	})
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
			msg, ok := h.readyBySnapshot()
			if ok {
				return msg, nil
			}
			return Message{}, fmt.Errorf("wanix system ready timeout after assets %s; page %s: %w", h.server.Summary(), h.readySnapshot(), ctx.Err())
		}
	}
}

func (h *Host) readyBySnapshot() (Message, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := h.Eval(ctx, `JSON.stringify({
  isReady: !!(document.getElementById("system") && document.getElementById("system").isReady),
  capabilities: {
    wasm: typeof WebAssembly !== "undefined",
    worker: typeof Worker !== "undefined",
    messageChannel: typeof MessageChannel !== "undefined",
    blob: typeof Blob !== "undefined",
    fetch: typeof fetch !== "undefined",
    sharedArrayBuffer: typeof SharedArrayBuffer !== "undefined",
    crossOriginIsolated: !!globalThis.crossOriginIsolated
  }
})`)
	if err != nil {
		return Message{}, false
	}
	var status struct {
		IsReady      bool            `json:"isReady"`
		Capabilities map[string]bool `json:"capabilities"`
	}
	if err := jsonUnmarshalString(result, &status); err != nil || !status.IsReady {
		return Message{}, false
	}
	return Message{Type: "ready", Capabilities: status.Capabilities}, true
}

func (h *Host) readySnapshot() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := h.Eval(ctx, `JSON.stringify({
  readyState: document.readyState,
  location: String(location.href),
  hasWebkit: !!(window.webkit && window.webkit.messageHandlers),
  hasWanixHandler: !!(window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.wanix),
  hasSystem: !!document.getElementById("system"),
  isReady: !!(document.getElementById("system") && document.getElementById("system").isReady),
  hasRoot: !!(document.getElementById("system") && document.getElementById("system")._root),
  systemReadyType: String(document.getElementById("system") && typeof document.getElementById("system")._ready),
  setupNamespaceType: String(document.getElementById("system") && typeof document.getElementById("system")._setupNamespace),
  openPortType: String(document.getElementById("system") && typeof document.getElementById("system")._openPort),
  setupNamespaceText: String(document.getElementById("system") && document.getElementById("system")._setupNamespace).slice(0, 80),
  openPortText: String(document.getElementById("system") && document.getElementById("system")._openPort).slice(0, 80),
  wasmReadyType: String(document.getElementById("system") && typeof document.getElementById("system")._wasmReady),
  controllerStarted: !!window.wanixHostControllerStarted,
  ensureStarted: !!window.wanixHostEnsureNamespaceStarted,
  runtimeHooksReady: !!window.wanixHostRuntimeHooksReady,
  namespaceEnsured: !!window.wanixHostNamespaceEnsured,
  wanixGlobalKeys: window.__wanix ? Object.keys(window.__wanix).join(",") : ""
})`)
	if err != nil {
		return "snapshot error: " + err.Error()
	}
	return result
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
	status, err := h.ReadFile(ctx, "macos/status")
	if err != nil {
		return fmt.Errorf("macos namespace status: %w", err)
	}
	if !bytes.Contains([]byte(status), []byte("api macos\n")) {
		return fmt.Errorf("macos namespace status: %q", status)
	}
	if legacy, err := h.ReadFile(ctx, "mnt/macos/status"); err != nil {
		return fmt.Errorf("legacy macos namespace status: %w", err)
	} else if !bytes.Contains([]byte(legacy), []byte("api macos\n")) {
		return fmt.Errorf("legacy macos namespace status: %q", legacy)
	}
	if err := h.WriteFile(ctx, "macos/window/title", "Wanix Native Self-Test\n"); err != nil {
		return fmt.Errorf("macos namespace title: %w", err)
	}
	if err := h.selfTestAppleFS(ctx); err != nil {
		return err
	}
	return nil
}

func (h *Host) selfTestAppleFS(ctx context.Context) error {
	entries, err := h.ReadMacOSDir(ctx, "")
	if err != nil {
		return fmt.Errorf("macos namespace readdir: %w", err)
	}
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.Dir {
			seen[entry.Name] = true
		}
	}
	for _, name := range []string{
		"appkit", "notify", "touchid", "vision", "image", "document",
		"speech", "mic", "screen", "ax", "keychain", "reachability", "vz",
	} {
		if !seen[name] {
			return fmt.Errorf("macos namespace missing %s in %v", name, entries)
		}
	}
	checks := []struct {
		path string
		want string
	}{
		{"macos/appkit/status", "api appkit\n"},
		{"macos/vision/status", "status ok\n"},
		{"macos/speech/voices", "["},
		{"macos/reachability/status", "api reachability\n"},
		{"macos/vz/status", "api vz\n"},
	}
	for _, check := range checks {
		data, err := h.ReadFile(ctx, check.path)
		if err != nil {
			return fmt.Errorf("read %s: %w", check.path, err)
		}
		if !bytes.Contains([]byte(data), []byte(check.want)) {
			return fmt.Errorf("read %s: missing %q in %q", check.path, check.want, data)
		}
	}
	if err := h.selfTestCloneSurfaces(ctx); err != nil {
		return err
	}
	if os.Getenv("WANIX_MACAPP_SELFTEST_LIVE") == "1" {
		if err := h.selfTestLiveAppleFS(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (h *Host) selfTestCloneSurfaces(ctx context.Context) error {
	visionID, err := h.cloneID(ctx, "macos/vision/clone")
	if err != nil {
		return err
	}
	if err := h.WriteFile(ctx, "macos/vision/"+visionID+"/format", "json\n"); err != nil {
		return fmt.Errorf("vision format: %w", err)
	}
	if err := h.waitFile(ctx, "macos/vision/"+visionID+"/format", "json\n"); err != nil {
		return fmt.Errorf("vision format round trip: %w", err)
	}

	screenID, err := h.cloneID(ctx, "macos/screen/clone")
	if err != nil {
		return err
	}
	if err := h.WriteFile(ctx, "macos/screen/"+screenID+"/ctl", "fps 2\n"); err != nil {
		return fmt.Errorf("screen fps ctl: %w", err)
	}
	if err := h.waitFile(ctx, "macos/screen/"+screenID+"/fps", "2\n"); err != nil {
		return fmt.Errorf("screen fps round trip: %w", err)
	}

	micID, err := h.cloneID(ctx, "macos/mic/clone")
	if err != nil {
		return err
	}
	if err := h.WriteFile(ctx, "macos/mic/"+micID+"/ctl", "duration 100ms\n"); err != nil {
		return fmt.Errorf("mic duration ctl: %w", err)
	}
	if err := h.waitFile(ctx, "macos/mic/"+micID+"/duration", "100ms\n"); err != nil {
		return fmt.Errorf("mic duration round trip: %w", err)
	}

	vzID, err := h.cloneID(ctx, "macos/vz/clone")
	if err != nil {
		return err
	}
	cfg := `{"cpus":1,"memoryMiB":512,"bootMode":"efi"}` + "\n"
	if err := h.WriteFile(ctx, "macos/vz/"+vzID+"/config", cfg); err != nil {
		return fmt.Errorf("vz config: %w", err)
	}
	if err := h.waitFile(ctx, "macos/vz/"+vzID+"/config", cfg); err != nil {
		return fmt.Errorf("vz config round trip: %w", err)
	}
	return nil
}

func (h *Host) waitFile(ctx context.Context, path, want string) error {
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	var last string
	var lastErr error
	for {
		got, err := h.ReadFile(ctx, path)
		if err == nil && got == want {
			return nil
		}
		last = got
		lastErr = err
		select {
		case <-tick.C:
		case <-deadline.C:
			return fmt.Errorf("%s: got %q err %v", path, last, lastErr)
		case <-ctx.Done():
			return fmt.Errorf("%s: got %q err %v: %w", path, last, lastErr, ctx.Err())
		}
	}
}

func (h *Host) selfTestLiveAppleFS(ctx context.Context) error {
	micID, err := h.cloneID(ctx, "macos/mic/clone")
	if err != nil {
		return err
	}
	if err := h.WriteFile(ctx, "macos/mic/"+micID+"/ctl", "duration 100ms\n"); err != nil {
		return err
	}
	if err := h.WriteFile(ctx, "macos/mic/"+micID+"/ctl", "oneshot\n"); err != nil {
		return fmt.Errorf("mic oneshot: %w", err)
	}
	if disk := os.Getenv("WANIX_MACAPP_SELFTEST_VZ_DISK"); disk != "" {
		vzID, err := h.cloneID(ctx, "macos/vz/clone")
		if err != nil {
			return err
		}
		if err := h.WriteFile(ctx, "macos/vz/"+vzID+"/config", `{"cpus":1,"memoryMiB":1024,"bootMode":"efi","readOnly":true}`+"\n"); err != nil {
			return err
		}
		if err := h.WriteFile(ctx, "macos/vz/"+vzID+"/disk", disk+"\n"); err != nil {
			return err
		}
		if err := h.WriteFile(ctx, "macos/vz/"+vzID+"/ctl", "validate\n"); err != nil {
			return fmt.Errorf("vz validate: %w", err)
		}
	}
	return nil
}

func (h *Host) cloneID(ctx context.Context, path string) (string, error) {
	id, err := h.ReadFile(ctx, path)
	if err != nil {
		return "", fmt.Errorf("clone %s: %w", path, err)
	}
	return strings.TrimSpace(id), nil
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
  if (!name.startsWith("macos/") && !name.startsWith("mnt/macos/")) {
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

// ReadMacOSDir reads the hydrated macOS hostfs cache in the WebKit page.
func (h *Host) ReadMacOSDir(ctx context.Context, path string) ([]appleFSEntry, error) {
	name, err := jsString(path)
	if err != nil {
		return nil, err
	}
	result, err := h.EvalAsync(ctx, `return JSON.stringify(window.__wanixMacOSFS.readDir(`+name+`));`)
	if err != nil {
		return nil, err
	}
	var entries []appleFSEntry
	if err := jsonUnmarshalString(result, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ReadDir reads a directory through the Wanix root handle.
func (h *Host) ReadDir(ctx context.Context, path string) ([]appleFSEntry, error) {
	name, err := jsString(path)
	if err != nil {
		return nil, err
	}
	script := fmt.Sprintf(`
  const root = document.getElementById("system").root;
  return JSON.stringify(await root.readDir(%s));
`, name)
	result, err := h.EvalAsync(ctx, script)
	if err != nil {
		return nil, err
	}
	var entries []appleFSEntry
	if err := jsonUnmarshalString(result, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (h *Host) handleMessage(msg Message) {
	if msg.Type == "native.call" {
		h.handleNativeCall(msg)
		return
	}
	if msg.Type == "native.fs" {
		h.handleNativeFS(msg)
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
