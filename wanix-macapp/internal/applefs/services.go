package applefs

import (
	"fmt"
	"strings"
)

func BuiltinServices() []*Service {
	return []*Service{
		appkitService(),
		notifyService(),
		touchIDService(),
		jobService("vision", "api vision\nstatus ok\n", "ocr\nbarcode\nclassify\nrectangles\n", []string{"ocr", "barcode", "classify", "rectangles", "lang", "format", "run", "abort", "destroy"}),
		jobService("image", "", "filter grayscale\nfilter sepia\n", []string{"filter", "format", "run", "abort", "destroy"}),
		jobService("document", "", "pdf-text\n", []string{"pdf-text", "pages", "format", "run", "abort", "destroy"}),
		speechService(),
		micService(),
		screenService(),
		axService(),
		keychainService(),
		reachabilityService(),
		vzService(),
		researchService("xpc", "XPC launchd/bootstrap authority is research-only.\n", []string{"listeners"}),
		researchService("webkit", "Embedded WebKit page control is research-only.\n", []string{"pages"}),
		blockedService("fskit", "FSKit requires signed extensions and System Settings enablement.\n", []string{"providers"}),
		blockedService("cloudkit", "CloudKit requires signed iCloud entitlements and account state.\n", []string{"account", "containers"}),
		blockedService("netext", "NetworkExtension requires system extension policy and host-admin setup.\n", nil),
		blockedService("endpointsecurity", "EndpointSecurity requires special entitlement and host-wide event authority.\n", nil),
		blockedService("sm", "ServiceManagement mutates durable launchd/login-item state.\n", []string{"services"}),
	}
}

func appkitService() *Service {
	s := NewService("appkit")
	s.File("status", ReadFile("api appkit\nstatus ok\n"))
	s.File("app/ctl", CtlFile{Verb: allow("activate", "quit")})
	s.File("app/status", NewTextFile("status unknown\n"))
	s.File("window/ctl", CtlFile{Verb: allow("center", "fullscreen", "toggle-fullscreen", "minimize", "miniaturize", "close")})
	s.File("window/title", NewTextFile("Wanix\n"))
	s.File("window/status", NewTextFile("status unknown\n"))
	s.File("pasteboard/ctl", CtlFile{Verb: allow("clear")})
	s.File("pasteboard/text", NewTextFile(""))
	s.Clone("alert/clone", func(id string) map[string]File {
		result := NewTextFile("")
		return map[string]File{
			"ctl":     CtlFile{Verb: func(v string) error { return setResult(v, "show", result, "button OK\nindex 0\nresponse 1000\n") }},
			"title":   NewTextFile("Wanix\n"),
			"message": NewTextFile(""),
			"style":   NewTextFile("informational\n"),
			"buttons": NewTextFile("OK\n"),
			"result":  result,
		}
	})
	s.Clone("picker/clone", func(id string) map[string]File {
		result := NewTextFile("cancelled true\n")
		return map[string]File{
			"ctl":    CtlFile{Verb: func(v string) error { return setResult(v, "show", result, "cancelled true\n") }},
			"mode":   NewTextFile("open-file\n"),
			"prompt": NewTextFile(""),
			"types":  NewTextFile(""),
			"result": result,
		}
	})
	s.Clone("indicator/clone", func(id string) map[string]File {
		status := NewTextFile("status hidden\n")
		return map[string]File{
			"ctl":    CtlFile{Verb: allow("show", "hide", "destroy")},
			"text":   NewTextFile(""),
			"icon":   NewTextFile(""),
			"status": status,
			"event":  NewStreamFile(nil),
		}
	})
	return s
}

func notifyService() *Service {
	s := NewService("notify")
	s.File("status", NewTextFile("api notify\nstatus not-determined\nauthorized false\n"))
	s.File("ctl", CtlFile{Verb: allow("request-auth", "refresh")})
	s.File("event", NewStreamFile(nil))
	s.Clone("clone", func(id string) map[string]File {
		result := NewTextFile("")
		return map[string]File{
			"ctl":      CtlFile{Verb: notifyCtl(result)},
			"title":    NewTextFile(""),
			"body":     NewTextFile(""),
			"subtitle": NewTextFile(""),
			"sound":    NewTextFile(""),
			"delay":    NewTextFile("0s\n"),
			"result":   result,
		}
	})
	return s
}

func touchIDService() *Service {
	s := NewService("touchid")
	s.File("status", NewTextFile("api touchid\navailable false\npolicy device-owner-authentication\n"))
	s.File("ctl", CtlFile{Verb: allow("refresh")})
	s.Clone("clone", func(id string) map[string]File {
		result := NewTextFile("")
		return map[string]File{
			"ctl":    CtlFile{Verb: func(v string) error { return setResult(v, "evaluate", result, "error unavailable\n") }},
			"reason": NewTextFile(""),
			"result": result,
		}
	})
	return s
}

func jobService(name, status, listing string, verbs []string) *Service {
	s := NewService(name)
	if status != "" {
		s.File("status", ReadFile(status))
	}
	listName := map[string]string{"vision": "requests", "image": "filters", "document": "formats"}[name]
	s.File(listName, ReadFile(listing))
	s.Clone("clone", func(id string) map[string]File {
		in := NewTextFile("")
		out := NewTextFile("")
		state := NewTextFile("status idle\n")
		files := map[string]File{
			"ctl":    CtlFile{Verb: jobCtl(verbs, in, out, state)},
			"in":     in,
			"out":    out,
			"status": state,
			"meta":   ReadFile(""),
		}
		switch name {
		case "vision":
			files["request"] = NewTextFile("ocr\n")
			files["lang"] = NewTextFile("")
			files["level"] = NewTextFile("accurate\n")
			files["format"] = NewTextFile("text\n")
		case "image":
			files["filter"] = NewTextFile("")
			files["format"] = NewTextFile("png\n")
		case "document":
			files["pages"] = NewTextFile("")
			files["format"] = NewTextFile("text\n")
		}
		return files
	})
	return s
}

func speechService() *Service {
	s := NewService("speech")
	s.File("ctl", CtlFile{Verb: allow("refresh")})
	s.File("voices", NewTextFile("[]\n"))
	s.Clone("clone", func(id string) map[string]File {
		state := NewTextFile("status idle\n")
		return map[string]File{
			"ctl":       CtlFile{Verb: speechCtl(state)},
			"text":      NewTextFile(""),
			"ssml":      NewTextFile(""),
			"voice":     NewTextFile("default\n"),
			"rate":      NewTextFile("default\n"),
			"pitch":     NewTextFile("default\n"),
			"volume":    NewTextFile("default\n"),
			"assistive": NewTextFile("default\n"),
			"status":    state,
			"event":     NewStreamFile(nil),
		}
	})
	return s
}

func micService() *Service {
	s := NewService("mic")
	s.File("status", NewTextFile("api mic\nstatus not-determined\nauthorized false\n"))
	s.File("ctl", CtlFile{Verb: allow("request-auth", "refresh")})
	s.File("devices", NewTextFile("[]\n"))
	s.Clone("clone", func(id string) map[string]File {
		m := streamSession("format pcm16 24000 1\n")(id)
		m["duration"] = NewTextFile("1s\n")
		m["data"] = NewStreamFile(nil)
		return m
	})
	return s
}

func screenService() *Service {
	s := NewService("screen")
	s.File("status", NewTextFile("api screen\nstatus not-determined\nauthorized false\n"))
	s.File("ctl", CtlFile{Verb: allow("request-auth", "refresh")})
	s.File("displays", NewTextFile("[]\n"))
	s.File("windows", NewTextFile("[]\n"))
	s.Clone("clone", func(id string) map[string]File {
		m := streamSession("format png\n")(id)
		m["target"] = NewTextFile("")
		m["fps"] = NewTextFile("1\n")
		m["frames"] = NewStreamFile(nil)
		delete(m, "data")
		return m
	})
	return s
}

func axService() *Service {
	s := NewService("ax")
	s.File("status", NewTextFile("api ax\ntrusted false\n"))
	s.File("ctl", CtlFile{Verb: allow("request-trust", "refresh")})
	s.File("apps", ReadFile("[]\n"))
	s.File("system/focused", NewTextFile("{}\n"))
	s.Clone("app/clone", func(id string) map[string]File {
		return map[string]File{
			"ctl":      CtlFile{Verb: allow("attach", "refresh", "destroy")},
			"pid":      NewTextFile(""),
			"bundleid": NewTextFile(""),
			"status":   NewTextFile("status unattached\n"),
			"tree":     NewTextFile("{}\n"),
			"focused":  NewTextFile("{}\n"),
			"event":    NewStreamFile(nil),
		}
	})
	s.Clone("element/clone", func(id string) map[string]File {
		return map[string]File{
			"ctl":     CtlFile{Verb: allow("from-focused", "press", "focus", "destroy")},
			"attrs":   NewTextFile("{}\n"),
			"actions": NewTextFile("[]\n"),
			"value":   NewTextFile(""),
			"frame":   NewTextFile("{}\n"),
			"result":  NewTextFile("status idle\n"),
			"event":   NewStreamFile(nil),
		}
	})
	return s
}

func keychainService() *Service {
	s := NewService("keychain")
	s.Clone("clone", func(id string) map[string]File {
		return map[string]File{
			"ctl":     CtlFile{Verb: allow("query generic-password", "query certificate", "trust-evaluate", "destroy")},
			"query":   NewTextFile("{}\n"),
			"results": NewTextFile("[]\n"),
			"status":  NewTextFile("status idle\n"),
		}
	})
	return s
}

func reachabilityService() *Service {
	s := NewService("reachability")
	s.File("ctl", CtlFile{Verb: allow("refresh")})
	s.File("status", NewTextFile("api reachability\nstatus unknown\n"))
	s.File("proxies", ReadFile("http-enable false\n"))
	s.Clone("clone", func(id string) map[string]File {
		return map[string]File{
			"ctl":    CtlFile{Verb: allow("check", "destroy")},
			"target": NewTextFile(""),
			"flags":  NewTextFile("reachable false\n"),
			"status": NewTextFile("status unknown\n"),
			"event":  NewStreamFile(nil),
		}
	})
	return s
}

func vzService() *Service {
	s := NewService("vz")
	s.File("status", NewTextFile("api vz\nstatus unknown\n"))
	s.Clone("clone", func(id string) map[string]File {
		return map[string]File{
			"ctl":     CtlFile{Verb: allow("validate", "start", "pause", "resume", "stop", "kill", "destroy")},
			"config":  NewTextFile("{}\n"),
			"disk":    NewTextFile(""),
			"net":     NewTextFile(""),
			"status":  NewTextFile("status unknown\n"),
			"console": NewStreamFile(nil),
			"event":   NewStreamFile(nil),
			"report":  NewTextFile("{}\n"),
		}
	})
	return s
}

func researchService(name, readme string, dirs []string) *Service {
	s := NewService(name).ReadOnly()
	s.File("status", ReadFile("api "+name+"\nstatus research-only\n"))
	s.File("README", ReadFile(readme))
	for _, dir := range dirs {
		s.Dir(dir)
	}
	return s
}

func blockedService(name, readme string, dirs []string) *Service {
	s := NewService(name).ReadOnly()
	s.File("status", ReadFile("api "+name+"\nstatus not-exposed\n"))
	s.File("README", ReadFile(readme))
	for _, dir := range dirs {
		s.Dir(dir)
	}
	return s
}

func allow(verbs ...string) func(string) error {
	allowed := make(map[string]bool)
	for _, verb := range verbs {
		allowed[verb] = true
	}
	return func(verb string) error {
		if allowed[verb] {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrBadCtl, verb)
	}
}

func notifyCtl(result *TextFile) func(string) error {
	return func(verb string) error {
		switch verb {
		case "schedule":
			return result.Write([]byte("status unavailable\n"))
		case "cancel":
			return result.Write([]byte("status cancelled\n"))
		default:
			return fmt.Errorf("%w: %s", ErrBadCtl, verb)
		}
	}
}

func jobCtl(verbs []string, in, out, state *TextFile) func(string) error {
	allowed := make(map[string]bool)
	for _, verb := range verbs {
		allowed[verb] = true
	}
	return func(verb string) error {
		head := verb
		if i := strings.IndexByte(verb, ' '); i >= 0 {
			head = verb[:i]
		}
		if !allowed[head] {
			return fmt.Errorf("%w: %s", ErrBadCtl, verb)
		}
		if head != "run" {
			return state.Write([]byte("status configured\n"))
		}
		data, _ := in.Read()
		_ = out.Write(data)
		return state.Write([]byte("status done\n"))
	}
}

func speechCtl(state *TextFile) func(string) error {
	return func(verb string) error {
		head := verb
		if i := strings.IndexByte(verb, ' '); i >= 0 {
			head = verb[:i]
		}
		switch head {
		case "speak":
			return state.Write([]byte("status speaking\n"))
		case "rate", "voice", "pitch", "volume", "assistive":
			return state.Write([]byte("status configured\n"))
		case "pause", "resume", "stop", "destroy":
			return state.Write([]byte("status idle\n"))
		default:
			return fmt.Errorf("%w: %s", ErrBadCtl, verb)
		}
	}
}

func streamSession(format string) func(string) map[string]File {
	return func(id string) map[string]File {
		state := NewTextFile("status idle\n")
		return map[string]File{
			"ctl":    CtlFile{Verb: streamCtl(state)},
			"status": state,
			"format": ReadFile(format),
			"data":   ReadFile(""),
			"event":  NewStreamFile(nil),
		}
	}
}

func streamCtl(state *TextFile) func(string) error {
	return func(verb string) error {
		head := verb
		if i := strings.IndexByte(verb, ' '); i >= 0 {
			head = verb[:i]
		}
		switch head {
		case "device", "format", "rate", "channels", "target", "fps", "duration":
			return state.Write([]byte("status configured\n"))
		case "start", "oneshot":
			return state.Write([]byte("status unavailable\n"))
		case "stop", "destroy":
			return state.Write([]byte("status idle\n"))
		default:
			return fmt.Errorf("%w: %s", ErrBadCtl, verb)
		}
	}
}

func setResult(got, want string, result *TextFile, text string) error {
	if got != want {
		return fmt.Errorf("%w: %s", ErrBadCtl, got)
	}
	return result.Write([]byte(text))
}
