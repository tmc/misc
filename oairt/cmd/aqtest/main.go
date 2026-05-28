//go:build darwin

// Standalone purego AudioQueueServices smoke test. Plays a 440 Hz sine wave
// for 3 seconds at 24kHz mono PCM16. If you hear the tone, native purego
// playback works.

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

type asbd struct {
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

type aqBuf struct {
	Capacity uint32
	_pad     uint32
	Data     unsafe.Pointer
	ByteSize uint32
	// trailing fields ignored
}

const (
	formatLPCM    uint32 = 0x6c70636d
	flagSignedInt uint32 = 0x4
	flagPacked    uint32 = 0x8
	numBuffers           = 3
	bufFrames            = 2400
	ringCap              = 24000 * 2 * 4
)

var (
	audioQueueNewOutput      func(*asbd, uintptr, unsafe.Pointer, uintptr, uintptr, uint32, *uintptr) int32
	audioQueueAllocateBuffer func(uintptr, uint32, **aqBuf) int32
	audioQueueEnqueueBuffer  func(uintptr, *aqBuf, uint32, unsafe.Pointer) int32
	audioQueueStart          func(uintptr, unsafe.Pointer) int32
	audioQueueStop           func(uintptr, bool) int32
	audioQueueDispose        func(uintptr, bool) int32
)

var (
	mu               sync.Mutex
	ring             [ringCap]byte
	head, tail, size int
	queue            uintptr
)

func ringWrite(p []byte) int {
	wrote := 0
	for wrote < len(p) && size < ringCap {
		c := len(p) - wrote
		if s := ringCap - size; c > s {
			c = s
		}
		if ts := ringCap - tail; c > ts {
			c = ts
		}
		copy(ring[tail:tail+c], p[wrote:wrote+c])
		tail = (tail + c) % ringCap
		size += c
		wrote += c
	}
	return wrote
}

func ringRead(dst unsafe.Pointer, n int) int {
	read := 0
	d := unsafe.Slice((*byte)(dst), n)
	for read < n && size > 0 {
		c := n - read
		if c > size {
			c = size
		}
		if hs := ringCap - head; c > hs {
			c = hs
		}
		copy(d[read:read+c], ring[head:head+c])
		head = (head + c) % ringCap
		size -= c
		read += c
	}
	return read
}

func cb(_ uintptr, aq uintptr, buf *aqBuf) uintptr {
	cap_ := int(buf.Capacity)
	mu.Lock()
	got := ringRead(buf.Data, cap_)
	mu.Unlock()
	if got < cap_ {
		sl := unsafe.Slice((*byte)(unsafe.Add(buf.Data, got)), cap_-got)
		for i := range sl {
			sl[i] = 0
		}
	}
	buf.ByteSize = uint32(cap_)
	audioQueueEnqueueBuffer(aq, buf, 0, nil)
	return 0
}

func main() {
	h, err := purego.Dlopen("/System/Library/Frameworks/AudioToolbox.framework/AudioToolbox", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dlopen:", err)
		os.Exit(1)
	}
	purego.RegisterLibFunc(&audioQueueNewOutput, h, "AudioQueueNewOutput")
	purego.RegisterLibFunc(&audioQueueAllocateBuffer, h, "AudioQueueAllocateBuffer")
	purego.RegisterLibFunc(&audioQueueEnqueueBuffer, h, "AudioQueueEnqueueBuffer")
	purego.RegisterLibFunc(&audioQueueStart, h, "AudioQueueStart")
	purego.RegisterLibFunc(&audioQueueStop, h, "AudioQueueStop")
	purego.RegisterLibFunc(&audioQueueDispose, h, "AudioQueueDispose")

	cbPtr := purego.NewCallback(cb)

	desc := asbd{
		SampleRate:       24000,
		FormatID:         formatLPCM,
		FormatFlags:      flagSignedInt | flagPacked,
		BytesPerPacket:   2,
		FramesPerPacket:  1,
		BytesPerFrame:    2,
		ChannelsPerFrame: 1,
		BitsPerChannel:   16,
	}
	if rc := audioQueueNewOutput(&desc, cbPtr, nil, 0, 0, 0, &queue); rc != 0 {
		fmt.Fprintf(os.Stderr, "AudioQueueNewOutput: %d\n", rc)
		os.Exit(1)
	}
	const byteSize uint32 = bufFrames * 2
	for i := 0; i < numBuffers; i++ {
		var b *aqBuf
		if rc := audioQueueAllocateBuffer(queue, byteSize, &b); rc != 0 {
			fmt.Fprintf(os.Stderr, "AllocateBuffer: %d\n", rc)
			os.Exit(1)
		}
		zeros := unsafe.Slice((*byte)(b.Data), byteSize)
		for j := range zeros {
			zeros[j] = 0
		}
		b.ByteSize = byteSize
		if rc := audioQueueEnqueueBuffer(queue, b, 0, nil); rc != 0 {
			fmt.Fprintf(os.Stderr, "EnqueueBuffer: %d\n", rc)
			os.Exit(1)
		}
	}
	if rc := audioQueueStart(queue, nil); rc != 0 {
		fmt.Fprintf(os.Stderr, "Start: %d\n", rc)
		os.Exit(1)
	}

	const dur = 3 * time.Second
	const freq = 440.0
	const rate = 24000
	totalSamples := int(dur.Seconds() * rate)
	chunk := 1200 // 50ms
	pcm := make([]byte, chunk*2)
	phase := 0.0
	step := 2 * math.Pi * freq / rate
	written := 0
	fmt.Fprintln(os.Stderr, "playing 440Hz sine for 3s...")
	for written < totalSamples {
		for i := 0; i < chunk; i++ {
			s := int16(math.Sin(phase) * 0.3 * 32767)
			binary.LittleEndian.PutUint16(pcm[i*2:], uint16(s))
			phase += step
		}
		mu.Lock()
		n := ringWrite(pcm)
		mu.Unlock()
		written += n / 2
		if n < len(pcm) {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	audioQueueStop(queue, true)
	audioQueueDispose(queue, true)
	fmt.Fprintln(os.Stderr, "done")
}
