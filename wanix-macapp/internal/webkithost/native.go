package webkithost

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/apple/appkit"
)

type nativeParams struct {
	Value   string `json:"value"`
	Command string `json:"command"`
	JSON    string `json:"json"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Style   string `json:"style"`
	Buttons string `json:"buttons"`
}

func (h *Host) handleNativeCall(msg Message) {
	var p nativeParams
	if len(msg.Params) > 0 {
		_ = json.Unmarshal(msg.Params, &p)
	}
	h.performOnMain(func() {
		if err := h.dispatchNative(msg.Method, p); err != nil {
			select {
			case h.errors <- Message{Type: "error", Message: err.Error()}:
			default:
			}
		}
	})
}

func (h *Host) dispatchNative(method string, p nativeParams) error {
	switch method {
	case "app.name":
		if p.Value == "" {
			return nil
		}
		if h.window.GetID() == 0 {
			return nil
		}
		h.window.SetTitle(p.Value)
	case "app.activate":
		appkit.GetNSApplicationClass().SharedApplication().Activate()
	case "app.quit":
		appkit.GetNSApplicationClass().SharedApplication().Terminate(nil)
	case "window.0.title":
		if h.window.GetID() == 0 {
			return nil
		}
		h.window.SetTitle(p.Value)
	case "window.0.ctl":
		if h.window.GetID() == 0 {
			return nil
		}
		switch strings.TrimSpace(p.Command) {
		case "center":
			h.window.Center()
		case "fullscreen", "toggle-fullscreen":
			h.window.ToggleFullScreen(nil)
		case "miniaturize", "minimize":
			h.window.Miniaturize(nil)
		case "close":
			h.window.PerformClose(nil)
		case "":
			return nil
		default:
			return fmt.Errorf("macos window ctl: unknown command %q", p.Command)
		}
	case "pasteboard.text":
		pb := appkit.GetNSPasteboardClass().GeneralPasteboard()
		pb.ClearContents()
		if !pb.SetStringForType(p.Value, appkit.NSPasteboardTypes.String) {
			return fmt.Errorf("macos pasteboard: set text failed")
		}
	case "dialog.open":
		return fmt.Errorf("macos dialog.open: not implemented")
	case "alert.show":
		return h.showAlert(p)
	default:
		return fmt.Errorf("macos: unknown method %q", method)
	}
	return nil
}

func (h *Host) showAlert(p nativeParams) error {
	alert := appkit.NewNSAlert()
	alert.SetMessageText(defaultString(p.Title, "Wanix"))
	alert.SetInformativeText(p.Message)
	alert.SetAlertStyle(alertStyle(p.Style))
	buttons := alertButtons(p.Buttons)
	for _, button := range buttons {
		alert.AddButtonWithTitle(button)
	}
	result := alert.RunModal()
	index := int(result) - 1000
	if index < 0 || index >= len(buttons) {
		index = -1
	}
	text := fmt.Sprintf(`{"button":%q,"index":%d,"response":%d}`+"\n", "", index, int(result))
	if index >= 0 {
		text = fmt.Sprintf(`{"button":%q,"index":%d,"response":%d}`+"\n", buttons[index], index, int(result))
	}
	js, err := jsString(text)
	if err != nil {
		return err
	}
	h.webView.EvaluateJavaScriptCompletionHandler(
		`window.__wanixMacOSFS && window.__wanixMacOSFS._set("alert/result", `+js+`)`,
		nil,
	)
	return nil
}

func alertButtons(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' })
	var out []string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	if len(out) == 0 {
		return []string{"OK"}
	}
	return out
}

func alertStyle(s string) appkit.NSAlertStyle {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return appkit.NSAlertStyleCritical
	case "warning":
		return appkit.NSAlertStyleWarning
	default:
		return appkit.NSAlertStyleInformational
	}
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
