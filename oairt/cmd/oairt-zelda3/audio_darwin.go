//go:build darwin

package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

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

type audioQueueBuffer struct {
	AudioDataBytesCapacity uint32
	_                      uint32
	AudioData              unsafe.Pointer
	AudioDataByteSize      uint32
}

const (
	kAudioFormatLinearPCM       uint32 = 0x6c70636d
	kAudioFormatFlagIsSignedInt uint32 = 0x4
	kAudioFormatFlagIsPacked    uint32 = 0x8

	audioQueueNumBuffers   = 3
	audioQueueBufferFrames = 2400
	audioQueueRingCap      = 24000 * 2 * 6
)

var (
	audioToolboxOnce   sync.Once
	audioToolboxLoaded bool

	audioQueueNewOutput      func(*audioStreamBasicDescription, uintptr, unsafe.Pointer, uintptr, uintptr, uint32, *uintptr) int32
	audioQueueAllocateBuffer func(uintptr, uint32, **audioQueueBuffer) int32
	audioQueueEnqueueBuffer  func(uintptr, *audioQueueBuffer, uint32, unsafe.Pointer) int32
	audioQueueStart          func(uintptr, unsafe.Pointer) int32
	audioQueueStop           func(uintptr, bool) int32
	audioQueueDispose        func(uintptr, bool) int32

	globalAudioQueue      audioQueuePlayer
	globalAudioCallback   uintptr
	globalAudioCallbackMu sync.Once
)

type nativePCMPlayer struct {
	sampleRate int
}

type audioQueuePlayer struct {
	mu      sync.Mutex
	queue   uintptr
	bufs    [audioQueueNumBuffers]*audioQueueBuffer
	ring    [audioQueueRingCap]byte
	head    int
	tail    int
	size    int
	started bool
}

func newNativePCMPlayer(sampleRate int) (pcmPlayer, error) {
	if sampleRate <= 0 {
		return nil, errors.New("audio sample rate must be positive")
	}
	return &nativePCMPlayer{sampleRate: sampleRate}, nil
}

func (p *nativePCMPlayer) Play(ctx context.Context, audio []byte) error {
	if len(audio) == 0 {
		return nil
	}
	if err := audioQueueStartPlayback(p.sampleRate); err != nil {
		return err
	}
	for len(audio) > 0 {
		n := audioQueueWrite(audio)
		audio = audio[n:]
		if len(audio) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	return nil
}

func (p *nativePCMPlayer) Cleanup() error {
	audioQueueStopPlayback()
	return nil
}

func (p *nativePCMPlayer) EstimatedLatency() time.Duration {
	return 20 * time.Millisecond
}

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
		return errors.New("audiotoolbox not loaded")
	}
	return nil
}

func audioQueueCallback(_ uintptr, aq uintptr, buf *audioQueueBuffer) uintptr {
	capacity := int(buf.AudioDataBytesCapacity)
	globalAudioQueue.mu.Lock()
	n := globalAudioQueue.readInto(buf.AudioData, capacity)
	globalAudioQueue.mu.Unlock()
	if n < capacity {
		silence := unsafe.Slice((*byte)(unsafe.Add(buf.AudioData, n)), capacity-n)
		for i := range silence {
			silence[i] = 0
		}
	}
	buf.AudioDataByteSize = uint32(capacity)
	audioQueueEnqueueBuffer(aq, buf, 0, nil)
	return 0
}

func audioQueueStartPlayback(sampleRate int) error {
	if err := loadAudioToolbox(); err != nil {
		return err
	}
	globalAudioCallbackMu.Do(func() {
		globalAudioCallback = purego.NewCallback(audioQueueCallback)
	})

	globalAudioQueue.mu.Lock()
	defer globalAudioQueue.mu.Unlock()
	if globalAudioQueue.started {
		return nil
	}

	asbd := audioStreamBasicDescription{
		SampleRate:       float64(sampleRate),
		FormatID:         kAudioFormatLinearPCM,
		FormatFlags:      kAudioFormatFlagIsSignedInt | kAudioFormatFlagIsPacked,
		BytesPerPacket:   2,
		FramesPerPacket:  1,
		BytesPerFrame:    2,
		ChannelsPerFrame: uint32(audioChannels),
		BitsPerChannel:   audioBitsPerSample,
	}

	var queue uintptr
	if rc := audioQueueNewOutput(&asbd, globalAudioCallback, nil, 0, 0, 0, &queue); rc != 0 {
		return fmt.Errorf("AudioQueueNewOutput: OSStatus=%d", rc)
	}
	globalAudioQueue.queue = queue

	const bufferBytes = uint32(audioQueueBufferFrames * audioChannels * audioBitsPerSample / 8)
	for i := range globalAudioQueue.bufs {
		var buf *audioQueueBuffer
		if rc := audioQueueAllocateBuffer(queue, bufferBytes, &buf); rc != 0 {
			audioQueueDispose(queue, true)
			globalAudioQueue.queue = 0
			return fmt.Errorf("AudioQueueAllocateBuffer: OSStatus=%d", rc)
		}
		zeros := unsafe.Slice((*byte)(buf.AudioData), bufferBytes)
		for j := range zeros {
			zeros[j] = 0
		}
		buf.AudioDataByteSize = bufferBytes
		globalAudioQueue.bufs[i] = buf
		if rc := audioQueueEnqueueBuffer(queue, buf, 0, nil); rc != 0 {
			audioQueueDispose(queue, true)
			globalAudioQueue.queue = 0
			return fmt.Errorf("AudioQueueEnqueueBuffer: OSStatus=%d", rc)
		}
	}
	if rc := audioQueueStart(queue, nil); rc != 0 {
		audioQueueDispose(queue, true)
		globalAudioQueue.queue = 0
		return fmt.Errorf("AudioQueueStart: OSStatus=%d", rc)
	}
	globalAudioQueue.started = true
	return nil
}

func audioQueueWrite(p []byte) int {
	globalAudioQueue.mu.Lock()
	defer globalAudioQueue.mu.Unlock()
	if !globalAudioQueue.started {
		return 0
	}
	return globalAudioQueue.write(p)
}

func audioQueueStopPlayback() {
	globalAudioQueue.mu.Lock()
	defer globalAudioQueue.mu.Unlock()
	if !globalAudioQueue.started {
		return
	}
	audioQueueStop(globalAudioQueue.queue, true)
	audioQueueDispose(globalAudioQueue.queue, true)
	globalAudioQueue.queue = 0
	for i := range globalAudioQueue.bufs {
		globalAudioQueue.bufs[i] = nil
	}
	globalAudioQueue.head = 0
	globalAudioQueue.tail = 0
	globalAudioQueue.size = 0
	globalAudioQueue.started = false
}

func (p *audioQueuePlayer) write(src []byte) int {
	wrote := 0
	for wrote < len(src) && p.size < audioQueueRingCap {
		n := len(src) - wrote
		if space := audioQueueRingCap - p.size; n > space {
			n = space
		}
		if tailSpace := audioQueueRingCap - p.tail; n > tailSpace {
			n = tailSpace
		}
		copy(p.ring[p.tail:p.tail+n], src[wrote:wrote+n])
		p.tail = (p.tail + n) % audioQueueRingCap
		p.size += n
		wrote += n
	}
	return wrote
}

func (p *audioQueuePlayer) readInto(dst unsafe.Pointer, n int) int {
	read := 0
	out := unsafe.Slice((*byte)(dst), n)
	for read < n && p.size > 0 {
		chunk := n - read
		if chunk > p.size {
			chunk = p.size
		}
		if headSpace := audioQueueRingCap - p.head; chunk > headSpace {
			chunk = headSpace
		}
		copy(out[read:read+chunk], p.ring[p.head:p.head+chunk])
		p.head = (p.head + chunk) % audioQueueRingCap
		p.size -= chunk
		read += chunk
	}
	return read
}
