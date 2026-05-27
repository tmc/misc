package applefs

import (
	"errors"
	"strings"
	"testing"
)

func TestRootMountsDesignedServices(t *testing.T) {
	root := NewRoot()
	if got := readString(t, root, "status"); got != "api macos\nstatus ok\n" {
		t.Fatalf("root status = %q", got)
	}
	entries, err := root.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool)
	for _, entry := range entries {
		if entry.Dir {
			got[entry.Name] = true
		}
	}
	for _, name := range []string{
		"appkit", "notify", "touchid", "vision", "image", "document",
		"speech", "mic", "screen", "ax", "keychain", "reachability", "vz",
		"xpc", "webkit", "fskit", "cloudkit", "netext", "endpointsecurity", "sm",
	} {
		if !got[name] {
			t.Fatalf("root missing service %s", name)
		}
	}
	for _, name := range []string{"app", "window", "pasteboard", "alert"} {
		if !got[name] {
			t.Fatalf("root missing legacy alias %s", name)
		}
	}
}

func TestAppkitCloneSessions(t *testing.T) {
	root := NewRoot()
	id := readString(t, root, "appkit/alert/clone")
	id = strings.TrimSpace(id)
	if id != "1" {
		t.Fatalf("alert id = %q, want 1", id)
	}
	if err := root.WriteFile("appkit/alert/"+id+"/title", []byte("Confirm\n")); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("appkit/alert/"+id+"/ctl", []byte("show\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "appkit/alert/"+id+"/result"); !strings.Contains(got, "button OK") {
		t.Fatalf("alert result = %q", got)
	}

	indicator := strings.TrimSpace(readString(t, root, "appkit/indicator/clone"))
	if err := root.WriteFile("appkit/indicator/"+indicator+"/text", []byte("Wanix\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "appkit/indicator/"+indicator+"/text"); got != "Wanix\n" {
		t.Fatalf("indicator text = %q", got)
	}
}

func TestLegacyAliases(t *testing.T) {
	root := NewRoot()
	if err := root.WriteFile("window/title", []byte("Legacy\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "appkit/window/title"); got != "Legacy\n" {
		t.Fatalf("alias write = %q", got)
	}
}

func TestEnsureClonePath(t *testing.T) {
	root := NewRoot()
	if !root.EnsureClonePath("appkit/picker/3/prompt") {
		t.Fatal("EnsureClonePath returned false")
	}
	if err := root.WriteFile("appkit/picker/3/prompt", []byte("Pick one\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "appkit/picker/3/prompt"); got != "Pick one\n" {
		t.Fatalf("picker prompt = %q", got)
	}
	if id := strings.TrimSpace(readString(t, root, "appkit/picker/clone")); id != "4" {
		t.Fatalf("next picker id = %q, want 4", id)
	}
}

func TestAppendFile(t *testing.T) {
	root := NewRoot()
	id := strings.TrimSpace(readString(t, root, "mic/clone"))
	if err := root.AppendFile("mic/"+id+"/data", []byte("ab")); err != nil {
		t.Fatal(err)
	}
	if err := root.AppendFile("mic/"+id+"/data", []byte("cd")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "mic/"+id+"/data"); got != "abcd" {
		t.Fatalf("appended data = %q", got)
	}
	if got := readString(t, root, "mic/"+id+"/data"); got != "" {
		t.Fatalf("drained data = %q, want empty", got)
	}
}

func TestHostApplyRunsAfterWrite(t *testing.T) {
	root := NewRoot()
	var gotName, gotData string
	root.Host = hostFunc(func(name string, data []byte) error {
		gotName = name
		gotData = string(data)
		return nil
	})
	if err := root.WriteFile("appkit/window/title", []byte("Title\n")); err != nil {
		t.Fatal(err)
	}
	if gotName != "appkit/window/title" || gotData != "Title\n" {
		t.Fatalf("host apply = (%q, %q)", gotName, gotData)
	}

	if err := root.WriteFile("window/title", []byte("Alias\n")); err != nil {
		t.Fatal(err)
	}
	if gotName != "appkit/window/title" || gotData != "Alias\n" {
		t.Fatalf("alias host apply = (%q, %q)", gotName, gotData)
	}
}

func TestCanonicalWritesMirrorAliases(t *testing.T) {
	root := NewRoot()
	var writes []string
	root.OnWrite = func(name string, data []byte) {
		writes = append(writes, name+"="+string(data))
	}
	if !root.EnsureClonePath("appkit/alert/1/result") {
		t.Fatal("EnsureClonePath returned false")
	}
	if err := root.WriteFile("appkit/alert/1/result", []byte("button OK\n")); err != nil {
		t.Fatal(err)
	}
	if !containsString(writes, "appkit/alert/1/result=button OK\n") {
		t.Fatalf("writes missing canonical update: %v", writes)
	}
	if !containsString(writes, "alert/1/result=button OK\n") {
		t.Fatalf("writes missing alias update: %v", writes)
	}

	var appends []string
	root.OnAppend = func(name string, data []byte) {
		appends = append(appends, name+"="+string(data))
	}
	if err := root.AppendFile("appkit/alert/1/result", []byte("again\n")); err != nil {
		t.Fatal(err)
	}
	if !containsString(appends, "appkit/alert/1/result=again\n") {
		t.Fatalf("appends missing canonical update: %v", appends)
	}
	if !containsString(appends, "alert/1/result=again\n") {
		t.Fatalf("appends missing alias update: %v", appends)
	}
}

func TestCloneSchemas(t *testing.T) {
	root := NewRoot()
	schemas := root.CloneSchemas()
	for _, key := range []string{"appkit/alert", "alert", "appkit/picker", "vision", "ax/app"} {
		if len(schemas[key]) == 0 {
			t.Fatalf("missing clone schema %s", key)
		}
	}
	if got := schemas["vision"]["format"]; got != "text\n" {
		t.Fatalf("vision format schema = %q", got)
	}
}

type hostFunc func(string, []byte) error

func (f hostFunc) Apply(name string, data []byte) error {
	return f(name, data)
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestAuthorizationSurfaces(t *testing.T) {
	root := NewRoot()
	for _, name := range []string{"notify/ctl", "mic/ctl", "screen/ctl"} {
		if err := root.WriteFile(name, []byte("request-auth\n")); err != nil {
			t.Fatalf("%s request-auth: %v", name, err)
		}
	}
	if err := root.WriteFile("ax/ctl", []byte("request-trust\n")); err != nil {
		t.Fatalf("ax request-trust: %v", err)
	}
	for _, name := range []string{"mic/status", "screen/status", "ax/status"} {
		if got := readString(t, root, name); got == "" {
			t.Fatalf("%s empty", name)
		}
	}
}

func TestBatchJobLifecycle(t *testing.T) {
	root := NewRoot()
	for _, svc := range []string{"vision", "image", "document"} {
		t.Run(svc, func(t *testing.T) {
			id := strings.TrimSpace(readString(t, root, svc+"/clone"))
			if err := root.WriteFile(svc+"/"+id+"/in", []byte("payload")); err != nil {
				t.Fatal(err)
			}
			if err := root.WriteFile(svc+"/"+id+"/ctl", []byte("run\n")); err != nil {
				t.Fatal(err)
			}
			if got := readString(t, root, svc+"/"+id+"/out"); got != "payload" {
				t.Fatalf("%s out = %q", svc, got)
			}
			if got := readString(t, root, svc+"/"+id+"/status"); !strings.Contains(got, "done") {
				t.Fatalf("%s status = %q", svc, got)
			}
		})
	}
}

func TestJobConfigFiles(t *testing.T) {
	root := NewRoot()
	id := strings.TrimSpace(readString(t, root, "vision/clone"))
	for _, file := range []string{"lang", "level", "format"} {
		if got := readString(t, root, "vision/"+id+"/"+file); got == "" && file != "lang" {
			t.Fatalf("vision %s is empty", file)
		}
	}
	if err := root.WriteFile("vision/"+id+"/format", []byte("json\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "vision/"+id+"/format"); got != "json\n" {
		t.Fatalf("vision format = %q", got)
	}

	speech := strings.TrimSpace(readString(t, root, "speech/clone"))
	if err := root.WriteFile("speech/"+speech+"/ctl", []byte("rate 0.55\n")); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("speech/"+speech+"/ctl", []byte("voice en-US\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "speech/"+speech+"/status"); !strings.Contains(got, "configured") {
		t.Fatalf("speech status = %q", got)
	}

	screen := strings.TrimSpace(readString(t, root, "screen/clone"))
	if err := root.WriteFile("screen/"+screen+"/target", []byte("display 1\n")); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("screen/"+screen+"/frames", []byte("png")); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("screen/"+screen+"/fps", []byte("2\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "screen/"+screen+"/target"); got != "display 1\n" {
		t.Fatalf("screen target = %q", got)
	}
	if got := readString(t, root, "screen/"+screen+"/frames"); got != "png" {
		t.Fatalf("screen frames = %q", got)
	}
	if got := readString(t, root, "screen/"+screen+"/fps"); got != "2\n" {
		t.Fatalf("screen fps = %q", got)
	}

	mic := strings.TrimSpace(readString(t, root, "mic/clone"))
	if err := root.WriteFile("mic/"+mic+"/duration", []byte("250ms\n")); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("mic/"+mic+"/data", []byte("pcm")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "mic/"+mic+"/duration"); got != "250ms\n" {
		t.Fatalf("mic duration = %q", got)
	}
	if got := readString(t, root, "mic/"+mic+"/data"); got != "pcm" {
		t.Fatalf("mic data = %q", got)
	}

	reachability := strings.TrimSpace(readString(t, root, "reachability/clone"))
	if err := root.WriteFile("reachability/"+reachability+"/target", []byte("default\n")); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("reachability/"+reachability+"/flags", []byte("reachable true\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "reachability/"+reachability+"/target"); got != "default\n" {
		t.Fatalf("reachability target = %q", got)
	}
	if got := readString(t, root, "reachability/"+reachability+"/flags"); got != "reachable true\n" {
		t.Fatalf("reachability flags = %q", got)
	}

	axapp := strings.TrimSpace(readString(t, root, "ax/app/clone"))
	if err := root.WriteFile("ax/app/"+axapp+"/tree", []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, root, "ax/app/"+axapp+"/tree"); got != "{}\n" {
		t.Fatalf("ax app tree = %q", got)
	}
}

func TestBlockedServicesAreReadOnly(t *testing.T) {
	root := NewRoot()
	for _, svc := range []string{"fskit", "cloudkit", "netext", "endpointsecurity", "sm"} {
		t.Run(svc, func(t *testing.T) {
			if got := readString(t, root, svc+"/status"); !strings.Contains(got, "not-exposed") {
				t.Fatalf("%s status = %q", svc, got)
			}
			err := root.WriteFile(svc+"/status", []byte("x"))
			if !errors.Is(err, ErrPermission) {
				t.Fatalf("%s write err = %v, want permission", svc, err)
			}
		})
	}
}

func TestUnknownCtlFailsClosed(t *testing.T) {
	root := NewRoot()
	if err := root.WriteFile("appkit/window/ctl", []byte("explode\n")); !errors.Is(err, ErrBadCtl) {
		t.Fatalf("unknown ctl err = %v, want bad ctl", err)
	}
}

func readString(t *testing.T, root *Root, name string) string {
	t.Helper()
	data, err := root.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}
