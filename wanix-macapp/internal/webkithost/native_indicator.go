package webkithost

import (
	"fmt"
	"strings"

	"github.com/tmc/apple/appkit"
)

func indicatorID(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) < 3 || parts[0] != "appkit" || parts[1] != "indicator" {
		return ""
	}
	return parts[2]
}

func (h *Host) applyIndicatorSession(name, verb string) error {
	id := indicatorID(name)
	if id == "" {
		return nil
	}
	switch verb {
	case "show":
		item := h.ensureIndicator(id)
		item.SetVisible(true)
		h.setIndicatorTitle(item, h.indicatorText(id))
		return h.appleFS.WriteFile("appkit/indicator/"+id+"/status", []byte("status visible\n"))
	case "hide":
		if item, ok := h.indicator(id); ok {
			item.SetVisible(false)
		}
		return h.appleFS.WriteFile("appkit/indicator/"+id+"/status", []byte("status hidden\n"))
	case "destroy":
		h.removeIndicator(id)
		return h.appleFS.WriteFile("appkit/indicator/"+id+"/status", []byte("status destroyed\n"))
	case "":
		return nil
	default:
		return fmt.Errorf("appkit indicator ctl: unknown command %q", verb)
	}
}

func (h *Host) applyIndicatorText(name, text string) error {
	id := indicatorID(name)
	if id == "" {
		return nil
	}
	if item, ok := h.indicator(id); ok {
		h.setIndicatorTitle(item, text)
	}
	return nil
}

func (h *Host) ensureIndicator(id string) appkit.NSStatusItem {
	h.indicatorMu.Lock()
	defer h.indicatorMu.Unlock()
	if item, ok := h.indicators[id]; ok && item.GetID() != 0 {
		return item
	}
	item := appkit.GetNSStatusBarClass().SystemStatusBar().
		StatusItemWithLength(appkit.VariableStatusItemLength).(appkit.NSStatusItem)
	h.indicators[id] = item
	return item
}

func (h *Host) indicator(id string) (appkit.NSStatusItem, bool) {
	h.indicatorMu.Lock()
	defer h.indicatorMu.Unlock()
	item, ok := h.indicators[id]
	return item, ok && item.GetID() != 0
}

func (h *Host) removeIndicator(id string) {
	h.indicatorMu.Lock()
	item, ok := h.indicators[id]
	if ok {
		delete(h.indicators, id)
	}
	h.indicatorMu.Unlock()
	if ok && item.GetID() != 0 {
		appkit.GetNSStatusBarClass().SystemStatusBar().RemoveStatusItem(item)
	}
}

func (h *Host) setIndicatorTitle(item appkit.NSStatusItem, text string) {
	if text == "" {
		text = "Wanix"
	}
	if button := item.Button(); button != nil && button.GetID() != 0 {
		button.SetTitle(text)
	}
}

func (h *Host) indicatorText(id string) string {
	return strings.TrimSpace(readAppleFSString(h, "appkit/indicator/"+id+"/text"))
}
