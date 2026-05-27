package webkithost

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unsafe"

	"github.com/tmc/apple/applicationservices"
	"github.com/tmc/apple/corefoundation"
)

const (
	axTreeMaxDepth = 2
	axTreeMaxNodes = 128
)

type axTreeNode struct {
	Attrs    map[string]string `json:"attrs,omitempty"`
	Actions  []string          `json:"actions,omitempty"`
	Children []axTreeNode      `json:"children,omitempty"`
	Error    string            `json:"error,omitempty"`
}

func (h *Host) applyAXCtl(verb string) error {
	switch verb {
	case "refresh":
		return h.refreshAXStatus()
	case "request-trust":
		return h.requestAXTrust()
	default:
		return nil
	}
}

func (h *Host) refreshAXStatus() error {
	trusted := applicationservices.AXIsProcessTrusted()
	if err := h.appleFS.WriteFile("ax/status", []byte("api ax\ntrusted "+fmt.Sprint(trusted)+"\n")); err != nil {
		return err
	}
	if !trusted {
		return nil
	}
	system := applicationservices.AXUIElementCreateSystemWide()
	if system == 0 {
		return nil
	}
	defer corefoundation.CFRelease(cfPtr(system))
	focused, err := copyAXAttribute(system, "AXFocusedUIElement")
	if err != nil {
		_ = h.appleFS.WriteFile("ax/system/focused", []byte("{\"error\":"+quoteJSON(err.Error())+"}\n"))
		return nil
	}
	text, err := json.MarshalIndent(focused, "", "  ")
	if err != nil {
		return err
	}
	text = append(text, '\n')
	return h.appleFS.WriteFile("ax/system/focused", text)
}

func (h *Host) requestAXTrust() error {
	key := cfString("AXTrustedCheckOptionPrompt")
	defer corefoundation.CFRelease(cfPtr(key))
	keys := []unsafe.Pointer{cfPtr(key)}
	values := []unsafe.Pointer{cfPtr(corefoundation.KCFBooleanTrue)}
	options := corefoundation.CFDictionaryCreate(
		corefoundation.KCFAllocatorDefault,
		unsafe.Pointer(&keys[0]),
		unsafe.Pointer(&values[0]),
		1,
		&corefoundation.KCFTypeDictionaryKeyCallBacks,
		&corefoundation.KCFTypeDictionaryValueCallBacks,
	)
	if options == 0 {
		return h.refreshAXStatus()
	}
	defer corefoundation.CFRelease(cfPtr(options))
	trusted := applicationservices.AXIsProcessTrustedWithOptions(options)
	return h.appleFS.WriteFile("ax/status", []byte("api ax\ntrusted "+fmt.Sprint(trusted)+"\nprompt true\n"))
}

func (h *Host) applyAXAppSession(name, verb string) error {
	id := strings.Split(name, "/")[2]
	switch verb {
	case "attach", "refresh":
		return h.refreshAXApp(id)
	default:
		return nil
	}
}

func (h *Host) applyAXElementSession(name, verb string) error {
	id := strings.Split(name, "/")[2]
	switch verb {
	case "from-focused":
		return h.captureFocusedAXElement(id)
	case "press":
		return h.performAXElementAction(id, "AXPress")
	case "focus":
		return h.performAXElementAction(id, "AXRaise")
	case "destroy":
		h.deleteAXElement(id)
		return h.appleFS.WriteFile("ax/element/"+id+"/result", []byte("status destroyed\n"))
	default:
		return nil
	}
}

func (h *Host) captureFocusedAXElement(id string) error {
	if err := h.refreshAXStatus(); err != nil {
		return err
	}
	system := applicationservices.AXUIElementCreateSystemWide()
	if system == 0 {
		err := fmt.Errorf("create system element")
		_ = h.appleFS.WriteFile("ax/element/"+id+"/result", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	defer corefoundation.CFRelease(cfPtr(system))
	element, err := copyAXAttributeValue(system, "AXFocusedUIElement")
	if err != nil {
		_ = h.appleFS.WriteFile("ax/element/"+id+"/result", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	h.setAXElement(id, applicationservices.AXUIElementRef(uintptr(element)))
	if err := h.refreshAXElementFiles(id, applicationservices.AXUIElementRef(uintptr(element))); err != nil {
		return err
	}
	return h.appleFS.WriteFile("ax/element/"+id+"/result", []byte("status focused\n"))
}

func (h *Host) performAXElementAction(id, actionName string) error {
	element, ok := h.axElement(id)
	if !ok {
		err := fmt.Errorf("element not captured")
		_ = h.appleFS.WriteFile("ax/element/"+id+"/result", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	action := cfString(actionName)
	defer corefoundation.CFRelease(cfPtr(action))
	if axerr := applicationservices.AXUIElementPerformAction(element, action); axerr != 0 {
		err := fmt.Errorf("perform %s: ax error %d", actionName, axerr)
		_ = h.appleFS.WriteFile("ax/element/"+id+"/result", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	return h.appleFS.WriteFile("ax/element/"+id+"/result", []byte("status ok\naction "+actionName+"\n"))
}

func (h *Host) refreshAXElementFiles(id string, element applicationservices.AXUIElementRef) error {
	attrs := copyAXAttributes(element, []string{"AXRole", "AXTitle", "AXValue", "AXDescription", "AXEnabled", "AXFocused"})
	attrText, err := json.MarshalIndent(attrs, "", "  ")
	if err != nil {
		return err
	}
	attrText = append(attrText, '\n')
	if err := h.appleFS.WriteFile("ax/element/"+id+"/attrs", attrText); err != nil {
		return err
	}
	actions, err := copyAXActionNames(element)
	if err != nil {
		actions = nil
	}
	actionText, err := json.MarshalIndent(actions, "", "  ")
	if err != nil {
		return err
	}
	actionText = append(actionText, '\n')
	if err := h.appleFS.WriteFile("ax/element/"+id+"/actions", actionText); err != nil {
		return err
	}
	frame, err := copyAXAttribute(element, "AXFrame")
	if err == nil {
		frameText, err := json.MarshalIndent(frame, "", "  ")
		if err != nil {
			return err
		}
		frameText = append(frameText, '\n')
		if err := h.appleFS.WriteFile("ax/element/"+id+"/frame", frameText); err != nil {
			return err
		}
	}
	return nil
}

func (h *Host) setAXElement(id string, element applicationservices.AXUIElementRef) {
	h.axMu.Lock()
	old := h.axElements[id]
	h.axElements[id] = element
	h.axMu.Unlock()
	if old != 0 {
		corefoundation.CFRelease(cfPtr(old))
	}
}

func (h *Host) axElement(id string) (applicationservices.AXUIElementRef, bool) {
	h.axMu.Lock()
	defer h.axMu.Unlock()
	element := h.axElements[id]
	return element, element != 0
}

func (h *Host) deleteAXElement(id string) {
	h.axMu.Lock()
	element := h.axElements[id]
	delete(h.axElements, id)
	h.axMu.Unlock()
	if element != 0 {
		corefoundation.CFRelease(cfPtr(element))
	}
}

func (h *Host) refreshAXApp(id string) error {
	if err := h.refreshAXStatus(); err != nil {
		return err
	}
	pidText := strings.TrimSpace(readAppleFSString(h, "ax/app/"+id+"/pid"))
	if pidText == "" {
		err := fmt.Errorf("missing pid")
		_ = h.appleFS.WriteFile("ax/app/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	pid64, err := strconv.ParseInt(pidText, 10, 32)
	if err != nil {
		_ = h.appleFS.WriteFile("ax/app/"+id+"/status", []byte("status error\nerror parse pid: "+err.Error()+"\n"))
		return fmt.Errorf("parse pid: %w", err)
	}
	app := applicationservices.AXUIElementCreateApplication(int32(pid64))
	if app == 0 {
		err := fmt.Errorf("create application element")
		_ = h.appleFS.WriteFile("ax/app/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	defer corefoundation.CFRelease(cfPtr(app))
	focused, err := copyAXAttribute(app, "AXFocusedUIElement")
	if err != nil {
		_ = h.appleFS.WriteFile("ax/app/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	text, err := json.MarshalIndent(focused, "", "  ")
	if err != nil {
		return err
	}
	text = append(text, '\n')
	if err := h.appleFS.WriteFile("ax/app/"+id+"/focused", text); err != nil {
		return err
	}
	tree := copyAXTree(app, axTreeMaxDepth, axTreeMaxNodes)
	treeText, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return err
	}
	treeText = append(treeText, '\n')
	if err := h.appleFS.WriteFile("ax/app/"+id+"/tree", treeText); err != nil {
		return err
	}
	return h.appleFS.WriteFile("ax/app/"+id+"/status", []byte("status attached\npid "+pidText+"\n"))
}

func copyAXTree(element applicationservices.AXUIElementRef, maxDepth, maxNodes int) axTreeNode {
	remaining := maxNodes
	return copyAXTreeNode(element, 0, maxDepth, &remaining)
}

func copyAXTreeNode(element applicationservices.AXUIElementRef, depth, maxDepth int, remaining *int) axTreeNode {
	if element == 0 || *remaining <= 0 {
		return axTreeNode{}
	}
	*remaining--
	node := axTreeNode{
		Attrs: copyAXAttributes(element, []string{"AXRole", "AXTitle", "AXDescription", "AXValue", "AXEnabled"}),
	}
	if actions, err := copyAXActionNames(element); err == nil && len(actions) > 0 {
		node.Actions = actions
	}
	if depth >= maxDepth || *remaining <= 0 {
		return node
	}
	children, err := copyAXChildren(element)
	if err != nil {
		node.Error = err.Error()
		return node
	}
	for _, child := range children {
		if *remaining <= 0 {
			break
		}
		node.Children = append(node.Children, copyAXTreeNode(child, depth+1, maxDepth, remaining))
	}
	return node
}

func copyAXChildren(element applicationservices.AXUIElementRef) ([]applicationservices.AXUIElementRef, error) {
	value, err := copyAXAttributeValue(element, "AXChildren")
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	defer corefoundation.CFRelease(value)
	children := corefoundation.CFArrayRef(uintptr(value))
	n := corefoundation.CFArrayGetCount(children)
	out := make([]applicationservices.AXUIElementRef, 0, n)
	for i := 0; i < n; i++ {
		child := corefoundation.CFArrayGetValueAtIndex(children, i)
		if child == nil {
			continue
		}
		out = append(out, applicationservices.AXUIElementRef(uintptr(child)))
	}
	return out, nil
}

func copyAXAttribute(element applicationservices.AXUIElementRef, name string) (map[string]string, error) {
	value, err := copyAXAttributeValue(element, name)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return map[string]string{"attribute": name, "value": ""}, nil
	}
	defer corefoundation.CFRelease(value)
	return describeAXValue(name, value), nil
}

func copyAXAttributeValue(element applicationservices.AXUIElementRef, name string) (corefoundation.CFTypeRef, error) {
	attr := cfString(name)
	defer corefoundation.CFRelease(cfPtr(attr))

	var value corefoundation.CFTypeRef
	if axerr := applicationservices.AXUIElementCopyAttributeValue(element, attr, &value); axerr != 0 {
		return nil, fmt.Errorf("copy %s: ax error %d", name, axerr)
	}
	return value, nil
}

func describeAXValue(name string, value corefoundation.CFTypeRef) map[string]string {
	desc := corefoundation.CFCopyDescription(value)
	if desc == 0 {
		return map[string]string{"attribute": name, "value": ""}
	}
	defer corefoundation.CFRelease(cfPtr(desc))
	return map[string]string{"attribute": name, "value": cfStringValue(desc)}
}

func copyAXAttributes(element applicationservices.AXUIElementRef, names []string) map[string]string {
	out := make(map[string]string)
	for _, name := range names {
		attr, err := copyAXAttribute(element, name)
		if err != nil {
			continue
		}
		out[name] = attr["value"]
	}
	return out
}

func copyAXActionNames(element applicationservices.AXUIElementRef) ([]string, error) {
	var names corefoundation.CFArrayRef
	if axerr := applicationservices.AXUIElementCopyActionNames(element, &names); axerr != 0 {
		return nil, fmt.Errorf("copy actions: ax error %d", axerr)
	}
	if names == 0 {
		return nil, nil
	}
	defer corefoundation.CFRelease(cfPtr(names))
	var out []string
	for i, n := 0, corefoundation.CFArrayGetCount(names); i < n; i++ {
		value := corefoundation.CFArrayGetValueAtIndex(names, i)
		if value == nil {
			continue
		}
		out = append(out, cfStringValue(corefoundation.CFStringRef(uintptr(value))))
	}
	return out, nil
}

func cfString(s string) corefoundation.CFStringRef {
	return corefoundation.CFStringCreateWithCString(
		corefoundation.KCFAllocatorDefault,
		s,
		uint32(corefoundation.KCFStringEncodingUTF8),
	)
}

func cfStringValue(s corefoundation.CFStringRef) string {
	buf := make([]byte, 4096)
	if !corefoundation.CFStringGetCString(s, &buf[0], len(buf), uint32(corefoundation.KCFStringEncodingUTF8)) {
		return ""
	}
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return string(buf[:n])
}

func quoteJSON(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(data)
}
