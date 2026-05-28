package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"sync/atomic"

	oairt "github.com/tmc/misc/oairt"
)

const micSampleRate = 24000

type micRecorderAPI interface {
	Start(context.Context) error
	Stop() error
}

var newMicRecorder = func(sampleRate int, callback func([]byte)) micRecorderAPI {
	return newNativeMicRecorder(sampleRate, callback)
}

type micSession struct {
	recorder micRecorderAPI
	sender   realtimeSender
	bytes    atomic.Int64

	mu  sync.Mutex
	err error
}

func startMicSession(ctx context.Context, sender realtimeSender) (*micSession, error) {
	s := &micSession{sender: sender}
	s.recorder = newMicRecorder(micSampleRate, func(data []byte) {
		if len(data) == 0 {
			return
		}
		if err := sendInputAudio(sender, data); err != nil {
			s.setErr(err)
			return
		}
		s.bytes.Add(int64(len(data)))
	})
	if err := s.recorder.Start(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *micSession) Stop(commit bool) error {
	if s == nil {
		return nil
	}
	if s.recorder != nil {
		if err := s.recorder.Stop(); err != nil {
			s.setErr(err)
		}
	}
	if commit {
		if err := commitInputAudio(s.sender); err != nil {
			s.setErr(err)
		}
	} else {
		if err := clearInputAudio(s.sender); err != nil {
			s.setErr(err)
		}
	}
	return s.Err()
}

func (s *micSession) Bytes() int64 {
	if s == nil {
		return 0
	}
	return s.bytes.Load()
}

func (s *micSession) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *micSession) setErr(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

func sendInputAudio(sender realtimeSender, data []byte) error {
	if sender == nil {
		return fmt.Errorf("send input audio: sender is nil")
	}
	return sender.Send(oairt.Event{
		Type:  oairt.EventInputAudioBufferAppend,
		Audio: base64.StdEncoding.EncodeToString(data),
	})
}

func commitInputAudio(sender realtimeSender) error {
	if err := sender.Send(oairt.Event{Type: oairt.EventInputAudioBufferCommit}); err != nil {
		return fmt.Errorf("commit input audio: %w", err)
	}
	if err := sender.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		return fmt.Errorf("create response after input audio: %w", err)
	}
	return nil
}

func clearInputAudio(sender realtimeSender) error {
	if err := sender.Send(oairt.Event{Type: oairt.EventInputAudioBufferClear}); err != nil {
		return fmt.Errorf("clear input audio: %w", err)
	}
	return nil
}
