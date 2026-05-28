//go:build darwin

package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/tmc/apple/avfaudio"
)

type nativePCMPlayer struct {
	mu         sync.Mutex
	sampleRate int
	engine     avfaudio.AVAudioEngine
	node       avfaudio.AVAudioPlayerNode
	format     avfaudio.AVAudioFormat
}

func newNativePCMPlayer(sampleRate int) (pcmPlayer, error) {
	if sampleRate <= 0 {
		return nil, errors.New("audio sample rate must be positive")
	}
	return &nativePCMPlayer{sampleRate: sampleRate}, nil
}

func (p *nativePCMPlayer) Play(ctx context.Context, audio []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(audio) == 0 {
		return nil
	}
	if err := p.ensureReadyLocked(); err != nil {
		return err
	}
	buffer, frames, err := p.newPCMBufferLocked(audio)
	if err != nil {
		return err
	}
	defer buffer.Release()

	done := make(chan struct{}, 1)
	p.node.ScheduleBufferCompletionCallbackTypeCompletionHandler(
		buffer,
		avfaudio.AVAudioPlayerNodeCompletionDataPlayedBack,
		func() {
			select {
			case done <- struct{}{}:
			default:
			}
		},
	)
	p.node.PrepareWithFrameCount(frames)
	p.node.Play()

	timer := time.NewTimer(p.playbackTimeout(len(audio)))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			p.node.Stop()
			return ctx.Err()
		case <-done:
			return nil
		case <-timer.C:
			if !p.engine.IsRunning() {
				return errors.New("avaudio engine stopped before playback completed")
			}
			p.node.Stop()
			return nil
		}
	}
}

func (p *nativePCMPlayer) Cleanup() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releaseLocked()
	return nil
}

func (p *nativePCMPlayer) EstimatedLatency() time.Duration {
	return 20 * time.Millisecond
}

func (p *nativePCMPlayer) ensureReadyLocked() error {
	if p.engine.ID == 0 {
		if err := p.createEngineLocked(); err != nil {
			p.releaseLocked()
			return err
		}
	}
	if p.engine.IsRunning() {
		return nil
	}
	p.engine.Prepare()
	if _, err := p.engine.StartAndReturnError(); err != nil {
		return fmt.Errorf("start avaudio engine: %w", err)
	}
	return nil
}

func (p *nativePCMPlayer) createEngineLocked() error {
	p.engine = avfaudio.NewAVAudioEngine()
	if p.engine.ID == 0 {
		return errors.New("create avaudio engine")
	}
	p.node = avfaudio.NewAVAudioPlayerNode()
	if p.node.ID == 0 {
		return errors.New("create avaudio player node")
	}
	p.format = avfaudio.NewAudioFormatWithCommonFormatSampleRateChannelsInterleaved(
		avfaudio.AVAudioPCMFormatInt16,
		float64(p.sampleRate),
		avfaudio.AVAudioChannelCount(audioChannels),
		false,
	)
	if p.format.ID == 0 {
		return errors.New("create avaudio format")
	}
	p.engine.AttachNode(p.node)
	p.engine.ConnectToFormat(p.node, p.engine.MainMixerNode(), p.format)
	return nil
}

func (p *nativePCMPlayer) newPCMBufferLocked(audio []byte) (avfaudio.AVAudioPCMBuffer, avfaudio.AVAudioFrameCount, error) {
	bytesPerFrame := audioChannels * audioBitsPerSample / 8
	if len(audio)%bytesPerFrame != 0 {
		return avfaudio.AVAudioPCMBuffer{}, 0, fmt.Errorf("pcm payload has %d trailing bytes", len(audio)%bytesPerFrame)
	}
	frames := avfaudio.AVAudioFrameCount(len(audio) / bytesPerFrame)
	if frames == 0 {
		return avfaudio.AVAudioPCMBuffer{}, 0, errors.New("cannot play empty pcm buffer")
	}
	buffer := avfaudio.NewAudioPCMBufferWithPCMFormatFrameCapacity(p.format, frames)
	if buffer.ID == 0 {
		return avfaudio.AVAudioPCMBuffer{}, 0, errors.New("create avaudio pcm buffer")
	}
	buffer.SetFrameLength(frames)
	if err := copyPCM16Mono(buffer, audio, int(frames)); err != nil {
		buffer.Release()
		return avfaudio.AVAudioPCMBuffer{}, 0, err
	}
	return buffer, frames, nil
}

func copyPCM16Mono(buffer avfaudio.AVAudioPCMBuffer, audio []byte, frames int) error {
	channelData := buffer.Int16ChannelData()
	if channelData == nil {
		return errors.New("avaudio pcm buffer has no int16 channel data")
	}
	channels := unsafe.Slice((**int16)(channelData), audioChannels)
	if len(channels) == 0 || channels[0] == nil {
		return errors.New("avaudio pcm buffer returned empty channel data")
	}
	samples := unsafe.Slice(channels[0], frames)
	for i := range frames {
		samples[i] = int16(binary.LittleEndian.Uint16(audio[i*2:]))
	}
	return nil
}

func (p *nativePCMPlayer) playbackTimeout(n int) time.Duration {
	timeout := p.estimatedDuration(n) + 2*time.Second
	if timeout < 2*time.Second {
		return 2 * time.Second
	}
	return timeout
}

func (p *nativePCMPlayer) estimatedDuration(n int) time.Duration {
	bytesPerFrame := audioChannels * audioBitsPerSample / 8
	if bytesPerFrame <= 0 || p.sampleRate <= 0 {
		return 0
	}
	frames := float64(n) / float64(bytesPerFrame)
	return time.Duration(frames / float64(p.sampleRate) * float64(time.Second))
}

func (p *nativePCMPlayer) releaseLocked() {
	if p.node.ID != 0 {
		p.node.Stop()
		p.node.Release()
		p.node = avfaudio.AVAudioPlayerNode{}
	}
	if p.format.ID != 0 {
		p.format.Release()
		p.format = avfaudio.AVAudioFormat{}
	}
	if p.engine.ID != 0 {
		p.engine.Stop()
		p.engine.Release()
		p.engine = avfaudio.AVAudioEngine{}
	}
}
