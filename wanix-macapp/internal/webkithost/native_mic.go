package webkithost

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/tmc/apple/avfaudio"
	"github.com/tmc/apple/avfoundation"
	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/foundation"
)

type micDevice struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ModelID      string `json:"modelID,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Type         string `json:"type,omitempty"`
	Position     string `json:"position,omitempty"`
	Connected    bool   `json:"connected"`
	Default      bool   `json:"default,omitempty"`
}

func (h *Host) applyMicCtl(verb string) error {
	switch verb {
	case "request-auth":
		return h.requestMicAuth()
	case "refresh":
		return h.refreshMicStatus()
	default:
		return nil
	}
}

func (h *Host) applyMicSession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	switch {
	case strings.HasPrefix(verb, "duration "):
		d := strings.TrimSpace(strings.TrimPrefix(verb, "duration "))
		if _, err := time.ParseDuration(d); err != nil {
			return fmt.Errorf("parse mic duration: %w", err)
		}
		if err := h.appleFS.WriteFile("mic/"+id+"/duration", []byte(d+"\n")); err != nil {
			return err
		}
		return h.appleFS.WriteFile("mic/"+id+"/status", []byte("status configured\n"))
	case strings.HasPrefix(verb, "device "), strings.HasPrefix(verb, "format "):
		return h.appleFS.WriteFile("mic/"+id+"/status", []byte("status configured\n"))
	case verb == "oneshot":
		return h.captureMicOneShot(id)
	case verb == "start":
		return h.startMicStream(id)
	case verb == "stop", verb == "destroy":
		h.stopMicStream(id)
		return h.appleFS.WriteFile("mic/"+id+"/status", []byte("status idle\n"))
	default:
		return nil
	}
}

func (h *Host) requestMicAuth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	class := avfoundation.GetAVCaptureDeviceClass()
	granted, err := class.RequestAccessForMediaType(ctx, avfoundation.AVMediaTypes.Audio)
	if err != nil {
		_ = h.appleFS.WriteFile("mic/status", []byte("api mic\nstatus error\nauthorized false\nerror "+err.Error()+"\n"))
		return fmt.Errorf("request mic auth: %w", err)
	}
	return h.writeMicState(micAuthStatusText(class.AuthorizationStatusForMediaType(avfoundation.AVMediaTypes.Audio)), granted)
}

func (h *Host) refreshMicStatus() error {
	class := avfoundation.GetAVCaptureDeviceClass()
	status := class.AuthorizationStatusForMediaType(avfoundation.AVMediaTypes.Audio)
	return h.writeMicState(micAuthStatusText(status), status == avfoundation.AVAuthorizationStatusAuthorized)
}

func (h *Host) writeMicState(status string, authorized bool) error {
	devices := micDevices()
	data, err := formatMicJSON(devices)
	if err != nil {
		return err
	}
	if err := h.appleFS.WriteFile("mic/devices", data); err != nil {
		return err
	}
	text := fmt.Sprintf("api mic\nstatus %s\nauthorized %t\ndevices %d\n", status, authorized, len(devices))
	return h.appleFS.WriteFile("mic/status", []byte(text))
}

func (h *Host) captureMicOneShot(id string) error {
	d, err := time.ParseDuration(strings.TrimSpace(readAppleFSString(h, "mic/"+id+"/duration")))
	if err != nil {
		_ = h.appleFS.WriteFile("mic/"+id+"/status", []byte("status error\nerror parse duration: "+err.Error()+"\n"))
		return fmt.Errorf("parse mic duration: %w", err)
	}
	data, err := captureMicPCM16(d)
	if err != nil {
		_ = h.appleFS.WriteFile("mic/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	if err := h.appleFS.WriteFile("mic/"+id+"/data", data); err != nil {
		return err
	}
	frames := len(data) / 2
	return h.appleFS.WriteFile("mic/"+id+"/status", []byte(fmt.Sprintf("status done\nbytes %d\nframes %d\n", len(data), frames)))
}

func captureMicPCM16(d time.Duration) ([]byte, error) {
	if d <= 0 {
		return nil, fmt.Errorf("mic duration must be positive")
	}
	const outRate = 24000

	engine := avfaudio.NewAVAudioEngine()
	bus := avfaudio.AVAudioNodeBus(0)
	input := engine.InputNode()
	inFmt := input.InputFormatForBus(bus)
	outFmt := avfaudio.NewAudioFormatWithCommonFormatSampleRateChannelsInterleaved(
		avfaudio.AVAudioPCMFormatInt16, float64(outRate), 1, true,
	)
	conv := avfaudio.NewAudioConverterFromFormatToFormat(inFmt, outFmt)

	var mu sync.Mutex
	var pcm []byte
	tap := func(in avfaudio.AVAudioPCMBuffer, _ avfaudio.AVAudioTime) {
		out := convertMicBuffer(in, inFmt, outFmt, conv)
		if len(out) == 0 {
			return
		}
		mu.Lock()
		pcm = append(pcm, out...)
		mu.Unlock()
	}

	const tapFrames avfaudio.AVAudioFrameCount = 4096
	input.RemoveTapOnBus(bus)
	releaseTap := avfaudio.InstallTapOnBus(input, bus, tapFrames, inFmt, tap)
	defer func() {
		input.RemoveTapOnBus(bus)
		releaseTap()
	}()

	if ok, err := engine.StartAndReturnError(); !ok || err != nil {
		return nil, fmt.Errorf("start mic capture: %w", err)
	}
	defer engine.Stop()

	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		corefoundation.CFRunLoopRunInMode(corefoundation.KCFRunLoopDefaultMode, 0.05, false)
	}

	mu.Lock()
	defer mu.Unlock()
	out := make([]byte, len(pcm))
	copy(out, pcm)
	return out, nil
}

func (h *Host) startMicStream(id string) error {
	h.stopMicStream(id)
	if err := h.appleFS.WriteFile("mic/"+id+"/data", nil); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.micMu.Lock()
	h.micStreams[id] = cancel
	h.micMu.Unlock()

	if err := h.appleFS.WriteFile("mic/"+id+"/status", []byte("status running\nbytes 0\nframes 0\n")); err != nil {
		h.stopMicStream(id)
		return err
	}
	go h.runMicStream(ctx, id)
	return nil
}

func (h *Host) stopMicStream(id string) {
	h.micMu.Lock()
	cancel := h.micStreams[id]
	delete(h.micStreams, id)
	h.micMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (h *Host) runMicStream(ctx context.Context, id string) {
	var bytes, frames int
	err := streamMicPCM16(ctx, func(chunk []byte) {
		if len(chunk) == 0 {
			return
		}
		if err := appendAppleFSFile(h, "mic/"+id+"/data", chunk); err != nil {
			return
		}
		bytes += len(chunk)
		frames += len(chunk) / 2
		_ = h.appleFS.WriteFile("mic/"+id+"/status", []byte(fmt.Sprintf("status running\nbytes %d\nframes %d\n", bytes, frames)))
	})
	if err != nil && !errorsIsCanceled(err) {
		_ = h.appleFS.WriteFile("mic/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return
	}
	_ = h.appleFS.WriteFile("mic/"+id+"/status", []byte(fmt.Sprintf("status stopped\nbytes %d\nframes %d\n", bytes, frames)))
}

func streamMicPCM16(ctx context.Context, onChunk func([]byte)) error {
	const outRate = 24000

	engine := avfaudio.NewAVAudioEngine()
	bus := avfaudio.AVAudioNodeBus(0)
	input := engine.InputNode()
	inFmt := input.InputFormatForBus(bus)
	outFmt := avfaudio.NewAudioFormatWithCommonFormatSampleRateChannelsInterleaved(
		avfaudio.AVAudioPCMFormatInt16, float64(outRate), 1, true,
	)
	conv := avfaudio.NewAudioConverterFromFormatToFormat(inFmt, outFmt)

	tap := func(in avfaudio.AVAudioPCMBuffer, _ avfaudio.AVAudioTime) {
		onChunk(convertMicBuffer(in, inFmt, outFmt, conv))
	}
	const tapFrames avfaudio.AVAudioFrameCount = 4096
	input.RemoveTapOnBus(bus)
	releaseTap := avfaudio.InstallTapOnBus(input, bus, tapFrames, inFmt, tap)
	defer func() {
		input.RemoveTapOnBus(bus)
		releaseTap()
	}()

	if ok, err := engine.StartAndReturnError(); !ok || err != nil {
		return fmt.Errorf("start mic capture: %w", err)
	}
	defer engine.Stop()

	for ctx.Err() == nil {
		corefoundation.CFRunLoopRunInMode(corefoundation.KCFRunLoopDefaultMode, 0.05, false)
	}
	return ctx.Err()
}

func appendAppleFSFile(h *Host, name string, chunk []byte) error {
	return h.appleFS.AppendFile(name, chunk)
}

func errorsIsCanceled(err error) bool {
	return err == nil || err == context.Canceled
}

func convertMicBuffer(in avfaudio.AVAudioPCMBuffer, inFmt, outFmt avfaudio.IAVAudioFormat, conv avfaudio.AVAudioConverter) []byte {
	inFrames := uint32(in.FrameLength())
	ratio := float64(24000) / inFmt.SampleRate()
	out := avfaudio.NewAudioPCMBufferWithPCMFormatFrameCapacity(
		outFmt, avfaudio.AVAudioFrameCount(float64(inFrames)*ratio)+1024,
	)
	consumed := false
	inputBlock := func(_ uint32, status *avfaudio.AVAudioConverterInputStatus) avfaudio.AVAudioBuffer {
		if consumed {
			*status = avfaudio.AVAudioConverterInputStatus_NoDataNow
			return avfaudio.AVAudioBuffer{}
		}
		consumed = true
		*status = avfaudio.AVAudioConverterInputStatus_HaveData
		return in.AVAudioBuffer
	}
	var nsErr foundation.NSError
	conv.ConvertToBufferErrorWithInputFromBlock(out, nsErr, inputBlock)

	nFrames := int(out.FrameLength())
	if nFrames == 0 {
		return nil
	}
	channels := (**int16)(out.Int16ChannelData())
	if channels == nil || *channels == nil {
		return nil
	}
	src := unsafe.Slice(*channels, nFrames)
	pcm := make([]byte, nFrames*2)
	for i, sample := range src {
		pcm[i*2] = byte(sample)
		pcm[i*2+1] = byte(sample >> 8)
	}
	return pcm
}

func clampStreamFPS(n int) int {
	return int(math.Max(1, math.Min(30, float64(n))))
}

func micDevices() []micDevice {
	class := avfoundation.GetAVCaptureDeviceClass()
	defaultID := ""
	if d := class.DefaultDeviceWithMediaType(avfoundation.AVMediaTypes.Audio); d.ID != 0 {
		defaultID = d.UniqueID()
	}
	deviceTypes := []string{
		string(avfoundation.AVCaptureDeviceTypes.BuiltInMicrophone),
		string(avfoundation.AVCaptureDeviceTypes.Microphone),
		string(avfoundation.AVCaptureDeviceTypes.External),
		string(avfoundation.AVCaptureDeviceTypes.ExternalUnknown),
	}
	var filtered []string
	for _, typ := range deviceTypes {
		if typ != "" {
			filtered = append(filtered, typ)
		}
	}
	session := avfoundation.NewCaptureDeviceDiscoverySessionWithDeviceTypesMediaTypePosition(
		filtered,
		avfoundation.AVMediaTypes.Audio,
		avfoundation.AVCaptureDevicePositionUnspecified,
	)
	devices := session.Devices()
	out := make([]micDevice, 0, len(devices))
	for _, d := range devices {
		id := d.UniqueID()
		out = append(out, micDevice{
			ID:           id,
			Name:         d.LocalizedName(),
			ModelID:      d.ModelID(),
			Manufacturer: d.Manufacturer(),
			Type:         string(d.DeviceType()),
			Position:     micPositionText(d.Position()),
			Connected:    d.IsConnected(),
			Default:      id != "" && id == defaultID,
		})
	}
	return out
}

func micAuthStatusText(status avfoundation.AVAuthorizationStatus) string {
	switch status {
	case avfoundation.AVAuthorizationStatusAuthorized:
		return "authorized"
	case avfoundation.AVAuthorizationStatusDenied:
		return "denied"
	case avfoundation.AVAuthorizationStatusRestricted:
		return "restricted"
	case avfoundation.AVAuthorizationStatusNotDetermined:
		return "not-determined"
	default:
		return fmt.Sprintf("unknown-%d", status)
	}
}

func micPositionText(pos avfoundation.AVCaptureDevicePosition) string {
	switch pos {
	case avfoundation.AVCaptureDevicePositionBack:
		return "back"
	case avfoundation.AVCaptureDevicePositionFront:
		return "front"
	case avfoundation.AVCaptureDevicePositionUnspecified:
		return "unspecified"
	default:
		return fmt.Sprintf("unknown-%d", pos)
	}
}

func formatMicJSON(v any) ([]byte, error) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
