//go:build darwin

// Native macOS audio path, purego only (no cgo). Default on darwin.
//
// Playback: AudioToolbox AudioQueueServices, loaded via purego. PCM bytes are
// copied into a mutex-protected ring buffer; the queue's output callback (a
// purego trampoline) drains the ring into recycled buffers and re-enqueues
// them. AudioQueue's C API is a clean purego target — no Objective-C, no
// Block_t, no NSError boxes.
//
// Capture: AVAudioEngine input-node tap + AVAudioConverter via tmc/apple
// (purego). Tap callbacks fire on a normal dispatch queue and work fine.

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/tmc/apple/avfaudio"
	"github.com/tmc/apple/foundation"
	"go.uber.org/zap"
)

// AudioStreamBasicDescription mirrors the CoreAudio struct.
type audioStreamBasicDescription struct {
	SampleRate       float64
	FormatID         uint32
	FormatFlags      uint32
	BytesPerPacket   uint32
	FramesPerPacket  uint32
	BytesPerFrame    uint32
	ChannelsPerFrame uint32
	BitsPerChannel   uint32
	Reserved         uint32
}

// AudioQueueBuffer mirrors the CoreAudio struct prefix we touch (data ptr,
// capacity, size). The struct is larger; we only access the first three
// fields, which are stable.
type audioQueueBuffer struct {
	AudioDataBytesCapacity uint32
	_pad                   uint32 // alignment for 64-bit pointer
	AudioData              unsafe.Pointer
	AudioDataByteSize      uint32
	// (more fields follow, ignored)
}

const (
	kAudioFormatLinearPCM       uint32 = 0x6c70636d // 'lpcm'
	kAudioFormatFlagIsSignedInt uint32 = 0x4
	kAudioFormatFlagIsPacked    uint32 = 0x8
	kAudioQueueNumBuffers              = 3
	kAudioQueueBufferFrames            = 2400 // 0.1s at 24kHz
	kAudioQueueRingCap                 = 24000 * 2 * 4
)

var (
	audioToolboxOnce   sync.Once
	audioToolboxLoaded bool

	audioQueueNewOutput      func(fmt_ *audioStreamBasicDescription, callback uintptr, userData unsafe.Pointer, callbackRunLoop uintptr, callbackRunLoopMode uintptr, flags uint32, outAQ *uintptr) int32
	audioQueueAllocateBuffer func(aq uintptr, size uint32, outBuf **audioQueueBuffer) int32
	audioQueueEnqueueBuffer  func(aq uintptr, buf *audioQueueBuffer, nPackets uint32, packetDescs unsafe.Pointer) int32
	audioQueueStart          func(aq uintptr, startTime unsafe.Pointer) int32
	audioQueueStop           func(aq uintptr, immediate bool) int32
	audioQueueDispose        func(aq uintptr, immediate bool) int32
)

func loadAudioToolbox() error {
	var loadErr error
	audioToolboxOnce.Do(func() {
		h, err := purego.Dlopen("/System/Library/Frameworks/AudioToolbox.framework/AudioToolbox", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = fmt.Errorf("dlopen AudioToolbox: %w", err)
			return
		}
		purego.RegisterLibFunc(&audioQueueNewOutput, h, "AudioQueueNewOutput")
		purego.RegisterLibFunc(&audioQueueAllocateBuffer, h, "AudioQueueAllocateBuffer")
		purego.RegisterLibFunc(&audioQueueEnqueueBuffer, h, "AudioQueueEnqueueBuffer")
		purego.RegisterLibFunc(&audioQueueStart, h, "AudioQueueStart")
		purego.RegisterLibFunc(&audioQueueStop, h, "AudioQueueStop")
		purego.RegisterLibFunc(&audioQueueDispose, h, "AudioQueueDispose")
		audioToolboxLoaded = true
	})
	if loadErr != nil {
		return loadErr
	}
	if !audioToolboxLoaded {
		return fmt.Errorf("AudioToolbox not loaded")
	}
	return nil
}

// audioQueuePlayer is package-global because the C-callback trampoline takes
// no closure capture; it dereferences the active player by reading this var.
type audioQueuePlayer struct {
	mu      sync.Mutex
	queue   uintptr
	bufs    [kAudioQueueNumBuffers]*audioQueueBuffer
	ring    [kAudioQueueRingCap]byte
	head    int
	tail    int
	size    int
	started bool
}

var (
	gAQ             audioQueuePlayer
	gAQCallbackPtr  uintptr
	gAQCallbackOnce sync.Once
)

func aqRingWrite(p []byte) int {
	wrote := 0
	for wrote < len(p) && gAQ.size < kAudioQueueRingCap {
		chunk := len(p) - wrote
		if space := kAudioQueueRingCap - gAQ.size; chunk > space {
			chunk = space
		}
		if tailSpace := kAudioQueueRingCap - gAQ.tail; chunk > tailSpace {
			chunk = tailSpace
		}
		copy(gAQ.ring[gAQ.tail:gAQ.tail+chunk], p[wrote:wrote+chunk])
		gAQ.tail = (gAQ.tail + chunk) % kAudioQueueRingCap
		gAQ.size += chunk
		wrote += chunk
	}
	return wrote
}

func aqRingReadInto(dst unsafe.Pointer, n int) int {
	read := 0
	dstSlice := unsafe.Slice((*byte)(dst), n)
	for read < n && gAQ.size > 0 {
		chunk := n - read
		if chunk > gAQ.size {
			chunk = gAQ.size
		}
		if hs := kAudioQueueRingCap - gAQ.head; chunk > hs {
			chunk = hs
		}
		copy(dstSlice[read:read+chunk], gAQ.ring[gAQ.head:gAQ.head+chunk])
		gAQ.head = (gAQ.head + chunk) % kAudioQueueRingCap
		gAQ.size -= chunk
		read += chunk
	}
	return read
}

// aqCallback is the AudioQueueOutputCallback. Signature:
//
//	void cb(void *userData, AudioQueueRef aq, AudioQueueBufferRef buf)
//
// It fires on CoreAudio's IO thread. Keep work minimal: lock, drain ring into
// the buffer's data pointer, pad with silence on underrun, re-enqueue.
func aqCallback(userData uintptr, aq uintptr, buf *audioQueueBuffer) uintptr {
	cap_ := int(buf.AudioDataBytesCapacity)
	gAQ.mu.Lock()
	got := aqRingReadInto(buf.AudioData, cap_)
	gAQ.mu.Unlock()
	if got < cap_ {
		// Pad with silence so playback timing stays smooth on underrun.
		silence := unsafe.Slice((*byte)(unsafe.Add(buf.AudioData, got)), cap_-got)
		for i := range silence {
			silence[i] = 0
		}
	}
	buf.AudioDataByteSize = uint32(cap_)
	audioQueueEnqueueBuffer(aq, buf, 0, nil)
	return 0
}

func aqStart(sampleRate int) error {
	if err := loadAudioToolbox(); err != nil {
		return err
	}
	gAQCallbackOnce.Do(func() {
		gAQCallbackPtr = purego.NewCallback(aqCallback)
	})

	gAQ.mu.Lock()
	defer gAQ.mu.Unlock()
	if gAQ.started {
		return nil
	}

	asbd := audioStreamBasicDescription{
		SampleRate:       float64(sampleRate),
		FormatID:         kAudioFormatLinearPCM,
		FormatFlags:      kAudioFormatFlagIsSignedInt | kAudioFormatFlagIsPacked,
		BytesPerPacket:   2,
		FramesPerPacket:  1,
		BytesPerFrame:    2,
		ChannelsPerFrame: 1,
		BitsPerChannel:   16,
	}

	var q uintptr
	if rc := audioQueueNewOutput(&asbd, gAQCallbackPtr, nil, 0, 0, 0, &q); rc != 0 {
		return fmt.Errorf("AudioQueueNewOutput: OSStatus=%d", rc)
	}
	gAQ.queue = q

	const byteSize = uint32(kAudioQueueBufferFrames * 2)
	for i := 0; i < kAudioQueueNumBuffers; i++ {
		var buf *audioQueueBuffer
		if rc := audioQueueAllocateBuffer(q, byteSize, &buf); rc != 0 {
			return fmt.Errorf("AudioQueueAllocateBuffer: OSStatus=%d", rc)
		}
		// Prime with silence.
		zeros := unsafe.Slice((*byte)(buf.AudioData), byteSize)
		for j := range zeros {
			zeros[j] = 0
		}
		buf.AudioDataByteSize = byteSize
		gAQ.bufs[i] = buf
		if rc := audioQueueEnqueueBuffer(q, buf, 0, nil); rc != 0 {
			return fmt.Errorf("AudioQueueEnqueueBuffer: OSStatus=%d", rc)
		}
	}

	if rc := audioQueueStart(q, nil); rc != 0 {
		return fmt.Errorf("AudioQueueStart: OSStatus=%d", rc)
	}
	gAQ.started = true
	return nil
}

func aqWrite(data []byte) int {
	gAQ.mu.Lock()
	defer gAQ.mu.Unlock()
	if !gAQ.started {
		return 0
	}
	return aqRingWrite(data)
}

func aqStop() {
	gAQ.mu.Lock()
	defer gAQ.mu.Unlock()
	if !gAQ.started {
		return
	}
	audioQueueStop(gAQ.queue, true)
	audioQueueDispose(gAQ.queue, true)
	gAQ.queue = 0
	for i := range gAQ.bufs {
		gAQ.bufs[i] = nil
	}
	gAQ.head, gAQ.tail, gAQ.size = 0, 0, 0
	gAQ.started = false
}

// NativePlayer plays via AudioQueueServices and records via AVAudioEngine.
type NativePlayer struct {
	mu         sync.Mutex
	state      *AppState
	sampleRate int
	closed     bool

	engine      avfaudio.AVAudioEngine
	recCallback func([]byte)
	recConv     avfaudio.AVAudioConverter
	recInFormat avfaudio.IAVAudioFormat
	recOutFmt   avfaudio.AVAudioFormat
	tapRelease  func()
}

func NewNativePlayer(state *AppState) *NativePlayer {
	return &NativePlayer{state: state}
}

func NewAudioPlayer(state *AppState) AudioEngine {
	return NewNativePlayer(state)
}

func checkAudioDependencies() error { return nil }

func (p *NativePlayer) Start(ctx context.Context, state *AppState, sampleRate int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = state
	p.sampleRate = sampleRate

	if err := aqStart(sampleRate); err != nil {
		return err
	}

	p.engine = avfaudio.NewAVAudioEngine()
	_ = p.engine.MainMixerNode()
	// Engine is started lazily in StartRecording so the mic tap is installed
	// before the engine pulls from the input node (which is when macOS issues
	// the TCC microphone permission prompt).

	logInfo("Native audio started", zap.Int("sampleRate", sampleRate))
	return nil
}

func (p *NativePlayer) Write(data []byte) (int, error) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return 0, fmt.Errorf("audio player is closed")
	}
	if len(data) == 0 {
		return 0, nil
	}
	return aqWrite(data), nil
}

func (p *NativePlayer) IsClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *NativePlayer) StartRecording(ctx context.Context, sampleRate int, callback func([]byte)) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.engine.Object.ID == 0 {
		return fmt.Errorf("audio engine not started")
	}

	bus := avfaudio.AVAudioNodeBus(0)
	input := p.engine.InputNode()
	p.recInFormat = input.InputFormatForBus(bus)

	p.recOutFmt = avfaudio.NewAudioFormatWithCommonFormatSampleRateChannelsInterleaved(
		avfaudio.AVAudioPCMFormatInt16, float64(sampleRate), 1, true,
	)
	p.recConv = avfaudio.NewAudioConverterFromFormatToFormat(p.recInFormat, p.recOutFmt)
	if p.recConv.Object.ID == 0 {
		return fmt.Errorf("AVAudioConverter init failed")
	}
	p.recCallback = callback

	input.RemoveTapOnBus(bus)
	const tapFrames avfaudio.AVAudioFrameCount = 4096

	var tapCalls, tapEmpty, tapBytes uint64
	var tapPeak int16
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			c := atomic.LoadUint64(&tapCalls)
			e := atomic.LoadUint64(&tapEmpty)
			b := atomic.LoadUint64(&tapBytes)
			pk := atomic.LoadInt32((*int32)(unsafe.Pointer(&tapPeak)))
			logInfo("recording stats", zap.Uint64("calls", c), zap.Uint64("emptyConvert", e), zap.Uint64("bytes", b), zap.Int32("peak", pk))
			atomic.StoreInt32((*int32)(unsafe.Pointer(&tapPeak)), 0)
		}
	}()

	tap := func(in avfaudio.AVAudioPCMBuffer, _ avfaudio.AVAudioTime) {
		atomic.AddUint64(&tapCalls, 1)
		inFrames := uint32(in.FrameLength())
		ratio := float64(sampleRate) / p.recInFormat.SampleRate()
		outCap := avfaudio.AVAudioFrameCount(float64(inFrames)*ratio) + 1024
		out := avfaudio.NewAudioPCMBufferWithPCMFormatFrameCapacity(p.recOutFmt, outCap)

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
		p.recConv.ConvertToBufferErrorWithInputFromBlock(out, nsErr, inputBlock)

		nFrames := int(out.FrameLength())
		if nFrames == 0 {
			atomic.AddUint64(&tapEmpty, 1)
			return
		}
		channels := (**int16)(out.Int16ChannelData())
		src := unsafe.Slice(*channels, nFrames)
		bytes := make([]byte, nFrames*2)
		var peak int16
		for i, s := range src {
			binary.LittleEndian.PutUint16(bytes[i*2:], uint16(s))
			a := s
			if a < 0 {
				a = -a
			}
			if a > peak {
				peak = a
			}
		}
		atomic.AddUint64(&tapBytes, uint64(len(bytes)))
		for {
			cur := atomic.LoadInt32((*int32)(unsafe.Pointer(&tapPeak)))
			if int32(peak) <= cur {
				break
			}
			if atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&tapPeak)), cur, int32(peak)) {
				break
			}
		}
		callback(bytes)
	}

	p.tapRelease = avfaudio.InstallTapOnBus(input, bus, tapFrames, p.recInFormat, tap)
	if ok, err := p.engine.StartAndReturnError(); !ok || err != nil {
		return fmt.Errorf("AVAudioEngine start: %w", err)
	}
	logInfo("Native recording started",
		zap.Float64("hwRate", p.recInFormat.SampleRate()),
		zap.Int("outRate", sampleRate))
	return nil
}

func (p *NativePlayer) StopRecording() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.engine.Object.ID != 0 {
		p.engine.InputNode().RemoveTapOnBus(0)
	}
	if p.tapRelease != nil {
		p.tapRelease()
		p.tapRelease = nil
	}
	p.recCallback = nil
	return nil
}

func (p *NativePlayer) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	if p.engine.Object.ID != 0 {
		p.engine.InputNode().RemoveTapOnBus(0)
		p.engine.Stop()
	}
	if p.tapRelease != nil {
		p.tapRelease()
		p.tapRelease = nil
	}
	p.mu.Unlock()

	aqStop()
	return nil
}
