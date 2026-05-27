package webkithost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/apple/appkit"
	"github.com/tmc/apple/localauthentication"
	"github.com/tmc/apple/objc"
	"github.com/tmc/apple/usernotifications"
)

type nativeParams struct {
	ID      string `json:"id"`
	Value   string `json:"value"`
	Command string `json:"command"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Style   string `json:"style"`
	Buttons string `json:"buttons"`
}

type nativeFSParams struct {
	Path string `json:"path"`
	Data string `json:"data"`
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

func (h *Host) handleNativeFS(msg Message) {
	var p nativeFSParams
	if len(msg.Params) > 0 {
		_ = json.Unmarshal(msg.Params, &p)
	}
	var value any
	var err error
	switch msg.Method {
	case "readFile":
		var data []byte
		data, err = h.appleFS.ReadFile(p.Path)
		value = base64.StdEncoding.EncodeToString(data)
	case "writeFile":
		var data []byte
		data, err = base64.StdEncoding.DecodeString(p.Data)
		if err == nil {
			err = h.appleFS.WriteFile(p.Path, data)
			if err != nil && h.appleFS.EnsureClonePath(p.Path) {
				err = h.appleFS.WriteFile(p.Path, data)
			}
		}
		value = "ok"
	case "readDir":
		value, err = h.appleFS.ReadDir(p.Path)
	case "isDir":
		_, err = h.appleFSEntries(p.Path)
		value = err == nil
		err = nil
	default:
		err = fmt.Errorf("native fs: unknown method %q", msg.Method)
	}
	h.replyNativeFS(msg.ID, value, err)
}

type nativeAppleHost struct {
	h *Host
}

func (n nativeAppleHost) Apply(name string, data []byte) error {
	return n.h.applyAppleFSWrite(name, data)
}

func (h *Host) applyAppleFSWrite(name string, data []byte) error {
	verb := strings.TrimSpace(string(data))
	switch {
	case name == "appkit/app/ctl":
		return h.applyAppCtl(verb)
	case name == "appkit/window/title":
		return h.applyWindowTitle(strings.TrimRight(string(data), "\n"))
	case name == "appkit/window/ctl":
		return h.applyWindowCtl(verb)
	case name == "appkit/pasteboard/text":
		return h.applyPasteboardText(string(data))
	case name == "appkit/pasteboard/ctl" && verb == "clear":
		return h.clearPasteboard()
	case strings.HasPrefix(name, "appkit/indicator/") && strings.HasSuffix(name, "/ctl"):
		return h.applyIndicatorSession(name, verb)
	case strings.HasPrefix(name, "appkit/indicator/") && strings.HasSuffix(name, "/text"):
		return h.applyIndicatorText(name, strings.TrimRight(string(data), "\n"))
	case strings.HasPrefix(name, "appkit/alert/") && strings.HasSuffix(name, "/ctl"):
		return h.applyAlertSession(name, verb)
	case name == "notify/ctl":
		return h.applyNotifyCtl(verb)
	case strings.HasPrefix(name, "notify/") && strings.HasSuffix(name, "/ctl"):
		return h.applyNotifySession(name, verb)
	case name == "touchid/ctl":
		return h.applyTouchIDCtl(verb)
	case strings.HasPrefix(name, "touchid/") && strings.HasSuffix(name, "/ctl") && verb == "evaluate":
		return h.evaluateTouchID(name)
	case name == "speech/ctl":
		return h.applySpeechCtl(verb)
	case strings.HasPrefix(name, "speech/") && strings.HasSuffix(name, "/ctl"):
		return h.applySpeechSession(name, verb)
	case strings.HasPrefix(name, "vision/") && strings.HasSuffix(name, "/ctl"):
		return h.applyVisionSession(name, verb)
	case strings.HasPrefix(name, "appkit/picker/") && strings.HasSuffix(name, "/ctl"):
		return h.applyPickerSession(name, verb)
	case strings.HasPrefix(name, "document/") && strings.HasSuffix(name, "/ctl"):
		return h.applyDocumentSession(name, verb)
	case name == "mic/ctl":
		return h.applyMicCtl(verb)
	case strings.HasPrefix(name, "mic/") && strings.HasSuffix(name, "/ctl"):
		return h.applyMicSession(name, verb)
	case strings.HasPrefix(name, "image/") && strings.HasSuffix(name, "/ctl"):
		return h.applyImageSession(name, verb)
	case strings.HasPrefix(name, "keychain/") && strings.HasSuffix(name, "/ctl"):
		return h.applyKeychainSession(name, verb)
	case name == "reachability/ctl":
		return h.applyReachabilityCtl(verb)
	case strings.HasPrefix(name, "reachability/") && strings.HasSuffix(name, "/ctl"):
		return h.applyReachabilitySession(name, verb)
	case name == "screen/ctl":
		return h.applyScreenCtl(verb)
	case strings.HasPrefix(name, "screen/") && strings.HasSuffix(name, "/ctl"):
		return h.applyScreenSession(name, verb)
	case strings.HasPrefix(name, "vz/") && strings.HasSuffix(name, "/ctl"):
		return h.applyVZSession(name, verb)
	case name == "ax/ctl":
		return h.applyAXCtl(verb)
	case strings.HasPrefix(name, "ax/app/") && strings.HasSuffix(name, "/ctl"):
		return h.applyAXAppSession(name, verb)
	case strings.HasPrefix(name, "ax/element/") && strings.HasSuffix(name, "/ctl"):
		return h.applyAXElementSession(name, verb)
	default:
		return nil
	}
}

func (h *Host) applyAppCtl(verb string) error {
	switch verb {
	case "activate":
		appkit.GetNSApplicationClass().SharedApplication().Activate()
		return h.appleFS.WriteFile("appkit/app/status", []byte("status active\n"))
	case "quit":
		appkit.GetNSApplicationClass().SharedApplication().Terminate(nil)
		return h.appleFS.WriteFile("appkit/app/status", []byte("status terminating\n"))
	default:
		return nil
	}
}

func (h *Host) applyWindowTitle(title string) error {
	if h.window.GetID() == 0 {
		return nil
	}
	h.window.SetTitle(title)
	return nil
}

func (h *Host) applyWindowCtl(verb string) error {
	if h.window.GetID() == 0 {
		return nil
	}
	switch verb {
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
		return fmt.Errorf("macos window ctl: unknown command %q", verb)
	}
	return h.appleFS.WriteFile("appkit/window/status", []byte("status "+verb+"\n"))
}

func (h *Host) applyPasteboardText(text string) error {
	pb := appkit.GetNSPasteboardClass().GeneralPasteboard()
	pb.ClearContents()
	if !pb.SetStringForType(text, appkit.NSPasteboardTypes.String) {
		return fmt.Errorf("macos pasteboard: set text failed")
	}
	return nil
}

func (h *Host) clearPasteboard() error {
	appkit.GetNSPasteboardClass().GeneralPasteboard().ClearContents()
	return h.appleFS.WriteFile("appkit/pasteboard/text", []byte(""))
}

func (h *Host) applyNotifyCtl(verb string) error {
	switch verb {
	case "request-auth":
		return h.requestNotificationAuth()
	case "refresh":
		return h.refreshNotificationStatus()
	default:
		return nil
	}
}

func (h *Host) requestNotificationAuth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	center := usernotifications.GetUNUserNotificationCenterClass().CurrentNotificationCenter()
	granted, err := center.RequestAuthorizationWithOptions(ctx, usernotifications.UNAuthorizationOptionAlert|usernotifications.UNAuthorizationOptionSound)
	if err != nil {
		_ = h.appleFS.WriteFile("notify/status", []byte("api notify\nstatus error\nerror "+err.Error()+"\n"))
		return err
	}
	settings, err := center.GetNotificationSettings(ctx)
	if err == nil && settings != nil {
		return h.writeNotificationStatus(settings.AuthorizationStatus())
	}
	return h.appleFS.WriteFile("notify/status", []byte("api notify\nstatus "+notifyStatusTextFromGrant(granted)+"\nauthorized "+fmt.Sprint(granted)+"\n"))
}

func (h *Host) refreshNotificationStatus() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	center := usernotifications.GetUNUserNotificationCenterClass().CurrentNotificationCenter()
	settings, err := center.GetNotificationSettings(ctx)
	if err != nil {
		_ = h.appleFS.WriteFile("notify/status", []byte("api notify\nstatus error\nerror "+err.Error()+"\n"))
		return fmt.Errorf("notification settings: %w", err)
	}
	if settings == nil {
		err := fmt.Errorf("notification settings unavailable")
		_ = h.appleFS.WriteFile("notify/status", []byte("api notify\nstatus error\nerror "+err.Error()+"\n"))
		return err
	}
	return h.writeNotificationStatus(settings.AuthorizationStatus())
}

func (h *Host) writeNotificationStatus(status usernotifications.UNAuthorizationStatus) error {
	text := notifyStatusText(status)
	return h.appleFS.WriteFile("notify/status", []byte("api notify\nstatus "+text+"\nauthorized "+fmt.Sprint(notifyAuthorized(status))+"\n"))
}

func notifyStatusTextFromGrant(granted bool) string {
	if granted {
		return "authorized"
	}
	return "denied"
}

func notifyStatusText(status usernotifications.UNAuthorizationStatus) string {
	switch status {
	case usernotifications.UNAuthorizationStatusAuthorized:
		return "authorized"
	case usernotifications.UNAuthorizationStatusDenied:
		return "denied"
	case usernotifications.UNAuthorizationStatusEphemeral:
		return "ephemeral"
	case usernotifications.UNAuthorizationStatusNotDetermined:
		return "not-determined"
	case usernotifications.UNAuthorizationStatusProvisional:
		return "provisional"
	default:
		return fmt.Sprintf("unknown-%d", status)
	}
}

func notifyAuthorized(status usernotifications.UNAuthorizationStatus) bool {
	switch status {
	case usernotifications.UNAuthorizationStatusAuthorized, usernotifications.UNAuthorizationStatusEphemeral, usernotifications.UNAuthorizationStatusProvisional:
		return true
	default:
		return false
	}
}

func (h *Host) applyNotifySession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	switch verb {
	case "schedule":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		center := usernotifications.GetUNUserNotificationCenterClass().CurrentNotificationCenter()
		content := usernotifications.NewUNMutableNotificationContent()
		title := strings.TrimSpace(readAppleFSString(h, "notify/"+id+"/title"))
		body := strings.TrimSpace(readAppleFSString(h, "notify/"+id+"/body"))
		subtitle := strings.TrimSpace(readAppleFSString(h, "notify/"+id+"/subtitle"))
		if title == "" {
			title = "Wanix"
		}
		objc.Send[struct{}](content.GetID(), objc.Sel("setTitle:"), objc.String(title))
		objc.Send[struct{}](content.GetID(), objc.Sel("setSubtitle:"), objc.String(subtitle))
		objc.Send[struct{}](content.GetID(), objc.Sel("setBody:"), objc.String(body))
		sound := notifySound(strings.TrimSpace(readAppleFSString(h, "notify/"+id+"/sound")))
		if sound.GetID() != 0 {
			objc.Send[struct{}](content.GetID(), objc.Sel("setSound:"), sound)
		}
		delay, err := notifyDelay(strings.TrimSpace(readAppleFSString(h, "notify/"+id+"/delay")))
		if err != nil {
			_ = h.appleFS.WriteFile("notify/"+id+"/result", []byte("status error\nerror "+err.Error()+"\n"))
			return err
		}
		trigger := usernotifications.NewUNTimeIntervalNotificationTriggerWithTimeIntervalRepeats(delay.Seconds(), false)
		identifier := notifyRequestID(id)
		request := usernotifications.NewUNNotificationRequestWithIdentifierContentTrigger(identifier, content, trigger)
		if err := center.AddNotificationRequest(ctx, request); err != nil {
			_ = h.appleFS.WriteFile("notify/"+id+"/result", []byte("status error\nerror "+err.Error()+"\n"))
			return err
		}
		return h.appleFS.WriteFile("notify/"+id+"/result", []byte(fmt.Sprintf("status queued\nid %s\ndelay %s\n", identifier, delay)))
	case "cancel":
		center := usernotifications.GetUNUserNotificationCenterClass().CurrentNotificationCenter()
		identifier := notifyRequestID(id)
		center.RemovePendingNotificationRequestsWithIdentifiers([]string{identifier})
		center.RemoveDeliveredNotificationsWithIdentifiers([]string{identifier})
		return h.appleFS.WriteFile("notify/"+id+"/result", []byte("status cancelled\n"))
	default:
		return nil
	}
}

func notifyRequestID(id string) string {
	return "wanix-" + id
}

func notifyDelay(text string) (time.Duration, error) {
	if text == "" {
		return time.Second, nil
	}
	d, err := time.ParseDuration(text)
	if err != nil {
		return 0, fmt.Errorf("parse delay: %w", err)
	}
	if d <= 0 {
		return time.Second, nil
	}
	return d, nil
}

func notifySound(text string) usernotifications.UNNotificationSound {
	switch text {
	case "", "default":
		return usernotifications.GetUNNotificationSoundClass().DefaultSound()
	case "none", "silent":
		return usernotifications.UNNotificationSound{}
	default:
		return usernotifications.NewUNNotificationSoundNamed(usernotifications.UNNotificationSoundName(text))
	}
}

func (h *Host) applyTouchIDCtl(verb string) error {
	switch verb {
	case "refresh":
		return h.refreshTouchIDStatus()
	default:
		return nil
	}
}

func (h *Host) refreshTouchIDStatus() error {
	ctx := localauthentication.NewLAContext()
	ownerOK, ownerErr := ctx.CanEvaluatePolicyError(localauthentication.LAPolicyDeviceOwnerAuthentication)
	bioOK, bioErr := ctx.CanEvaluatePolicyError(localauthentication.LAPolicyDeviceOwnerAuthenticationWithBiometrics)
	status := "available"
	if !ownerOK {
		status = "unavailable"
	}
	text := fmt.Sprintf("api touchid\nstatus %s\navailable %t\nbiometry %s\nbiometry-available %t\npolicy device-owner-authentication\n",
		status, ownerOK, biometryText(ctx.BiometryType()), bioOK)
	if ownerErr != nil {
		text += "error " + ownerErr.Error() + "\n"
	} else if bioErr != nil {
		text += "biometry-error " + bioErr.Error() + "\n"
	}
	return h.appleFS.WriteFile("touchid/status", []byte(text))
}

func biometryText(kind localauthentication.LABiometryType) string {
	switch kind {
	case localauthentication.LABiometryTypeTouchID:
		return "touchid"
	case localauthentication.LABiometryTypeFaceID:
		return "faceid"
	case localauthentication.LABiometryTypeOpticID:
		return "opticid"
	case localauthentication.LABiometryTypeNone:
		return "none"
	default:
		return fmt.Sprintf("unknown-%d", kind)
	}
}

func (h *Host) evaluateTouchID(name string) error {
	id := strings.Split(name, "/")[1]
	reason := strings.TrimSpace(readAppleFSString(h, "touchid/"+id+"/reason"))
	if reason == "" {
		reason = "Authenticate Wanix"
	}
	ctx := localauthentication.NewLAContext()
	policy := localauthentication.LAPolicyDeviceOwnerAuthentication
	ok, err := ctx.CanEvaluatePolicyError(policy)
	if err != nil || !ok {
		text := "ok false\nerror unavailable\n"
		if err != nil {
			text = "ok false\nerror " + err.Error() + "\n"
		}
		return h.appleFS.WriteFile("touchid/"+id+"/result", []byte(text))
	}
	deadline, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	accepted, err := ctx.EvaluatePolicyLocalizedReasonReplySync(deadline, policy, reason)
	if err != nil {
		return h.appleFS.WriteFile("touchid/"+id+"/result", []byte("ok false\nerror "+err.Error()+"\n"))
	}
	return h.appleFS.WriteFile("touchid/"+id+"/result", []byte("ok "+fmt.Sprint(accepted)+"\n"))
}

func readAppleFSString(h *Host, name string) string {
	data, err := h.appleFS.ReadFile(name)
	if err != nil {
		return ""
	}
	return string(data)
}

type appleFSEntry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
}

func (h *Host) appleFSEntries(path string) ([]appleFSEntry, error) {
	entries, err := h.appleFS.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := make([]appleFSEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, appleFSEntry{Name: entry.Name, Dir: entry.Dir})
	}
	return out, nil
}

func (h *Host) replyNativeFS(id string, value any, err error) {
	if id == "" {
		return
	}
	payload := map[string]any{"ok": err == nil, "value": value}
	if err != nil {
		payload["error"] = err.Error()
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		data, _ = json.Marshal(map[string]any{"ok": false, "error": marshalErr.Error()})
	}
	idJS, idErr := jsString(id)
	if idErr != nil {
		return
	}
	payloadJS, payloadErr := jsString(string(data))
	if payloadErr != nil {
		return
	}
	h.performOnMain(func() {
		h.webView.EvaluateJavaScriptCompletionHandler(
			`window.__wanixNativeFSReply && window.__wanixNativeFSReply(`+idJS+`, JSON.parse(`+payloadJS+`))`,
			nil,
		)
	})
}

func (h *Host) dispatchNative(method string, p nativeParams) error {
	switch method {
	case "app.activate":
		return h.applyAppCtl("activate")
	case "app.quit":
		return h.applyAppCtl("quit")
	case "window.title":
		return h.applyWindowTitle(p.Value)
	case "window.ctl":
		return h.applyWindowCtl(strings.TrimSpace(p.Command))
	case "pasteboard.text":
		return h.applyPasteboardText(p.Value)
	default:
		return fmt.Errorf("macos: unknown method %q", method)
	}
}

func (h *Host) applyAlertSession(name, verb string) error {
	if verb != "show" {
		return nil
	}
	id := strings.Split(name, "/")[2]
	if id == "" {
		return fmt.Errorf("macos alert: missing id")
	}
	alert := appkit.NewNSAlert()
	alert.SetMessageText(defaultString(strings.TrimSpace(readAppleFSString(h, "appkit/alert/"+id+"/title")), "Wanix"))
	alert.SetInformativeText(strings.TrimSpace(readAppleFSString(h, "appkit/alert/"+id+"/message")))
	alert.SetAlertStyle(alertStyle(readAppleFSString(h, "appkit/alert/"+id+"/style")))
	buttons := alertButtons(readAppleFSString(h, "appkit/alert/"+id+"/buttons"))
	for _, button := range buttons {
		alert.AddButtonWithTitle(button)
	}
	result := alert.RunModal()
	index := int(result) - 1000
	if index < 0 || index >= len(buttons) {
		index = -1
	}
	text := fmt.Sprintf("button \nindex %d\nresponse %d\n", index, int(result))
	if index >= 0 {
		text = fmt.Sprintf("button %s\nindex %d\nresponse %d\n", buttons[index], index, int(result))
	}
	return h.appleFS.WriteFile("appkit/alert/"+id+"/result", []byte(text))
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
