//go:build darwin

package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/tmc/apple/avfaudio"
	"github.com/tmc/apple/foundation"
)

const micTapFrames avfaudio.AVAudioFrameCount = 4096

type micRecorder struct {
	mu         sync.Mutex
	sampleRate int
	callback   func([]byte)

	engine     avfaudio.AVAudioEngine
	converter  avfaudio.AVAudioConverter
	inputFmt   avfaudio.IAVAudioFormat
	outputFmt  avfaudio.AVAudioFormat
	tapRelease func()
	done       chan struct{}
	started    bool
}

func newNativeMicRecorder(sampleRate int, callback func([]byte)) *micRecorder {
	return &micRecorder{
		sampleRate: sampleRate,
		callback:   callback,
	}
}

func (r *micRecorder) Start(ctx context.Context) error {
	if r.sampleRate <= 0 {
		return errors.New("microphone sample rate must be positive")
	}
	if r.callback == nil {
		return errors.New("microphone callback is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return nil
	}
	if err := r.startLocked(ctx); err != nil {
		r.stopLocked()
		return err
	}
	done := r.done
	r.started = true
	go func() {
		select {
		case <-ctx.Done():
			_ = r.Stop()
		case <-done:
		}
	}()
	return nil
}

func (r *micRecorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
	return nil
}

func (r *micRecorder) startLocked(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.engine = avfaudio.NewAVAudioEngine()
	if r.engine.ID == 0 {
		return errors.New("create avaudio engine")
	}

	bus := avfaudio.AVAudioNodeBus(0)
	input := r.engine.InputNode()
	r.inputFmt = input.InputFormatForBus(bus)
	r.outputFmt = avfaudio.NewAudioFormatWithCommonFormatSampleRateChannelsInterleaved(
		avfaudio.AVAudioPCMFormatInt16,
		float64(r.sampleRate),
		avfaudio.AVAudioChannelCount(audioChannels),
		true,
	)
	if r.outputFmt.ID == 0 {
		return errors.New("create microphone output format")
	}
	r.converter = avfaudio.NewAudioConverterFromFormatToFormat(r.inputFmt, r.outputFmt)
	if r.converter.ID == 0 {
		return errors.New("create microphone audio converter")
	}
	r.done = make(chan struct{})

	input.RemoveTapOnBus(bus)
	r.tapRelease = avfaudio.InstallTapOnBus(input, bus, micTapFrames, r.inputFmt, r.convertTap)
	if ok, err := r.engine.StartAndReturnError(); !ok || err != nil {
		return fmt.Errorf("start microphone avaudio engine: %w", err)
	}
	return nil
}

func (r *micRecorder) convertTap(in avfaudio.AVAudioPCMBuffer, _ avfaudio.AVAudioTime) {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	out, err := r.convertBuffer(in)
	callback := r.callback
	r.mu.Unlock()
	if err != nil || len(out) == 0 {
		return
	}
	callback(out)
}

func (r *micRecorder) convertBuffer(in avfaudio.AVAudioPCMBuffer) ([]byte, error) {
	inFrames := uint32(in.FrameLength())
	if inFrames == 0 {
		return nil, nil
	}
	outCap := avfaudio.AVAudioFrameCount(float64(inFrames)*float64(r.sampleRate)/r.inputFmt.SampleRate()) + 1024
	out := avfaudio.NewAudioPCMBufferWithPCMFormatFrameCapacity(r.outputFmt, outCap)
	if out.ID == 0 {
		return nil, errors.New("create microphone pcm buffer")
	}
	defer out.Release()

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
	r.converter.ConvertToBufferErrorWithInputFromBlock(out, nsErr, inputBlock)
	frames := int(out.FrameLength())
	if frames == 0 {
		return nil, nil
	}
	channelData := out.Int16ChannelData()
	if channelData == nil {
		return nil, errors.New("microphone pcm buffer has no int16 data")
	}
	channels := unsafe.Slice((**int16)(channelData), audioChannels)
	if len(channels) == 0 || channels[0] == nil {
		return nil, errors.New("microphone pcm buffer returned empty channel data")
	}
	samples := unsafe.Slice(channels[0], frames)
	pcm := make([]byte, frames*audioChannels*audioBitsPerSample/8)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(sample))
	}
	return pcm, nil
}

func (r *micRecorder) stopLocked() {
	if r.engine.ID != 0 {
		r.engine.InputNode().RemoveTapOnBus(0)
		r.engine.Stop()
	}
	if r.tapRelease != nil {
		r.tapRelease()
		r.tapRelease = nil
	}
	if r.converter.ID != 0 {
		r.converter.Release()
		r.converter = avfaudio.AVAudioConverter{}
	}
	if r.outputFmt.ID != 0 {
		r.outputFmt.Release()
		r.outputFmt = avfaudio.AVAudioFormat{}
	}
	if r.engine.ID != 0 {
		r.engine.Release()
		r.engine = avfaudio.AVAudioEngine{}
	}
	if r.done != nil {
		close(r.done)
		r.done = nil
	}
	r.inputFmt = nil
	r.started = false
}
