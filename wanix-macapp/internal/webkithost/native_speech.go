package webkithost

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/tmc/apple/avfaudio"
	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/foundation"
)

type speechVoice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Language string `json:"language"`
	Quality  string `json:"quality"`
	Gender   string `json:"gender"`
	Traits   uint   `json:"traits,omitempty"`
}

type speechSynth struct {
	synth avfaudio.AVSpeechSynthesizer
}

type speechConfig struct {
	Text      string
	SSML      string
	Voice     string
	Rate      string
	Pitch     string
	Volume    string
	Assistive string
}

func (h *Host) applySpeechCtl(verb string) error {
	switch verb {
	case "refresh":
		return h.refreshSpeechVoices()
	default:
		return nil
	}
}

func (h *Host) refreshSpeechVoices() error {
	data, err := formatSpeechVoices(speechVoices())
	if err != nil {
		return err
	}
	return h.appleFS.WriteFile("speech/voices", data)
}

func (h *Host) applySpeechSession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	switch {
	case verb == "speak":
		config := h.speechConfig(id)
		go h.speakText(id, config)
	case verb == "pause":
		return h.pauseSpeech(id)
	case verb == "resume":
		return h.resumeSpeech(id)
	case verb == "stop":
		return h.stopSpeech(id)
	case strings.HasPrefix(verb, "rate "):
		return h.appleFS.WriteFile("speech/"+id+"/rate", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "rate "))+"\n"))
	case strings.HasPrefix(verb, "voice "):
		return h.appleFS.WriteFile("speech/"+id+"/voice", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "voice "))+"\n"))
	case strings.HasPrefix(verb, "pitch "):
		return h.appleFS.WriteFile("speech/"+id+"/pitch", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "pitch "))+"\n"))
	case strings.HasPrefix(verb, "volume "):
		return h.appleFS.WriteFile("speech/"+id+"/volume", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "volume "))+"\n"))
	case strings.HasPrefix(verb, "assistive "):
		return h.appleFS.WriteFile("speech/"+id+"/assistive", []byte(strings.TrimSpace(strings.TrimPrefix(verb, "assistive "))+"\n"))
	}
	return nil
}

func (h *Host) speechConfig(id string) speechConfig {
	return speechConfig{
		Text:      strings.TrimSpace(readAppleFSString(h, "speech/"+id+"/text")),
		SSML:      strings.TrimSpace(readAppleFSString(h, "speech/"+id+"/ssml")),
		Voice:     strings.TrimSpace(readAppleFSString(h, "speech/"+id+"/voice")),
		Rate:      strings.TrimSpace(readAppleFSString(h, "speech/"+id+"/rate")),
		Pitch:     strings.TrimSpace(readAppleFSString(h, "speech/"+id+"/pitch")),
		Volume:    strings.TrimSpace(readAppleFSString(h, "speech/"+id+"/volume")),
		Assistive: strings.TrimSpace(readAppleFSString(h, "speech/"+id+"/assistive")),
	}
}

func (h *Host) speakText(id string, config speechConfig) {
	if config.Text == "" && config.SSML == "" {
		_ = h.appleFS.WriteFile("speech/"+id+"/status", []byte("status error\nerror empty text\n"))
		return
	}
	_ = h.appleFS.WriteFile("speech/"+id+"/status", []byte("status speaking\n"))
	_ = h.appleFS.WriteFile("speech/"+id+"/event", []byte(""))
	if err := speakWithAVSpeech(config, func(event string) {
		h.appendSpeechEvent(id, event)
	}, func(synth avfaudio.AVSpeechSynthesizer) {
		h.setSpeechSynth(id, synth)
	}, func() {
		h.deleteSpeechSynth(id)
	}); err != nil {
		_ = h.appleFS.WriteFile("speech/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return
	}
	if strings.TrimSpace(readAppleFSString(h, "speech/"+id+"/status")) == "status stopped" {
		return
	}
	_ = h.appleFS.WriteFile("speech/"+id+"/status", []byte("status finished\n"))
}

func (h *Host) appendSpeechEvent(id, event string) {
	name := "speech/" + id + "/event"
	data := readAppleFSString(h, name)
	_ = h.appleFS.WriteFile(name, []byte(data+event))
}

func (h *Host) setSpeechSynth(id string, synth avfaudio.AVSpeechSynthesizer) {
	h.speechMu.Lock()
	h.speech[id] = speechSynth{synth: synth}
	h.speechMu.Unlock()
}

func (h *Host) speechSynth(id string) (avfaudio.AVSpeechSynthesizer, bool) {
	h.speechMu.Lock()
	defer h.speechMu.Unlock()
	session, ok := h.speech[id]
	return session.synth, ok && session.synth.GetID() != 0
}

func (h *Host) deleteSpeechSynth(id string) {
	h.speechMu.Lock()
	delete(h.speech, id)
	h.speechMu.Unlock()
}

func (h *Host) pauseSpeech(id string) error {
	synth, ok := h.speechSynth(id)
	if !ok {
		return h.appleFS.WriteFile("speech/"+id+"/status", []byte("status idle\n"))
	}
	if !synth.PauseSpeakingAtBoundary(avfaudio.AVSpeechBoundaryImmediate) {
		return fmt.Errorf("speech pause failed")
	}
	return h.appleFS.WriteFile("speech/"+id+"/status", []byte("status paused\n"))
}

func (h *Host) resumeSpeech(id string) error {
	synth, ok := h.speechSynth(id)
	if !ok {
		return h.appleFS.WriteFile("speech/"+id+"/status", []byte("status idle\n"))
	}
	if !synth.ContinueSpeaking() {
		return fmt.Errorf("speech resume failed")
	}
	return h.appleFS.WriteFile("speech/"+id+"/status", []byte("status speaking\n"))
}

func (h *Host) stopSpeech(id string) error {
	synth, ok := h.speechSynth(id)
	if !ok {
		return h.appleFS.WriteFile("speech/"+id+"/status", []byte("status stopped\n"))
	}
	if !synth.StopSpeakingAtBoundary(avfaudio.AVSpeechBoundaryImmediate) {
		return fmt.Errorf("speech stop failed")
	}
	h.deleteSpeechSynth(id)
	return h.appleFS.WriteFile("speech/"+id+"/status", []byte("status stopped\n"))
}

func speechVoices() []speechVoice {
	voices := avfaudio.GetAVSpeechSynthesisVoiceClass().SpeechVoices()
	out := make([]speechVoice, 0, len(voices))
	for _, voice := range voices {
		out = append(out, speechVoice{
			ID:       voice.Identifier(),
			Name:     voice.Name(),
			Language: voice.Language(),
			Quality:  speechVoiceQualityText(voice.Quality()),
			Gender:   speechVoiceGenderText(voice.Gender()),
			Traits:   uint(voice.VoiceTraits()),
		})
	}
	return out
}

func formatSpeechVoices(voices []speechVoice) ([]byte, error) {
	data, err := json.MarshalIndent(voices, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func speechVoiceQualityText(quality avfaudio.AVSpeechSynthesisVoiceQuality) string {
	switch quality {
	case avfaudio.AVSpeechSynthesisVoiceQualityDefault:
		return "default"
	case avfaudio.AVSpeechSynthesisVoiceQualityEnhanced:
		return "enhanced"
	case avfaudio.AVSpeechSynthesisVoiceQualityPremium:
		return "premium"
	default:
		return fmt.Sprintf("unknown-%d", quality)
	}
}

func speechVoiceGenderText(gender avfaudio.AVSpeechSynthesisVoiceGender) string {
	switch gender {
	case avfaudio.AVSpeechSynthesisVoiceGenderFemale:
		return "female"
	case avfaudio.AVSpeechSynthesisVoiceGenderMale:
		return "male"
	case avfaudio.AVSpeechSynthesisVoiceGenderUnspecified:
		return "unspecified"
	default:
		return fmt.Sprintf("unknown-%d", gender)
	}
}

func speakWithAVSpeech(config speechConfig, event func(string), register func(avfaudio.AVSpeechSynthesizer), unregister func()) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	synth := avfaudio.NewAVSpeechSynthesizer()
	utterance, err := newSpeechUtterance(config)
	if err != nil {
		return err
	}

	voice := strings.TrimSpace(config.Voice)
	if voice != "" && voice != "default" {
		v := avfaudio.NewSpeechSynthesisVoiceWithLanguage(voice)
		if v.ID == 0 {
			return fmt.Errorf("unknown voice %q", voice)
		}
		utterance.SetVoice(v)
	}
	if err := setSpeechFloat("rate", config.Rate, utterance.SetRate); err != nil {
		return err
	}
	if err := setSpeechFloat("pitch", config.Pitch, utterance.SetPitchMultiplier); err != nil {
		return err
	}
	if err := setSpeechFloat("volume", config.Volume, utterance.SetVolume); err != nil {
		return err
	}
	if assistive := strings.TrimSpace(config.Assistive); assistive != "" && assistive != "default" {
		value, err := strconv.ParseBool(assistive)
		if err != nil {
			return fmt.Errorf("parse assistive: %w", err)
		}
		utterance.SetPrefersAssistiveTechnologySettings(value)
	}

	done := make(chan struct{})
	var doneOnce sync.Once
	delegate := avfaudio.NewAVSpeechSynthesizerDelegate(avfaudio.AVSpeechSynthesizerDelegateConfig{
		SpeechSynthesizerDidStartSpeechUtterance: func(_ avfaudio.AVSpeechSynthesizer, _ avfaudio.AVSpeechUtterance) {
			if event != nil {
				event("start\n")
			}
		},
		SpeechSynthesizerDidPauseSpeechUtterance: func(_ avfaudio.AVSpeechSynthesizer, _ avfaudio.AVSpeechUtterance) {
			if event != nil {
				event("pause\n")
			}
		},
		SpeechSynthesizerDidContinueSpeechUtterance: func(_ avfaudio.AVSpeechSynthesizer, _ avfaudio.AVSpeechUtterance) {
			if event != nil {
				event("resume\n")
			}
		},
		SpeechSynthesizerWillSpeakRangeOfSpeechStringUtterance: func(_ avfaudio.AVSpeechSynthesizer, r foundation.NSRange, _ avfaudio.AVSpeechUtterance) {
			if event != nil {
				event(fmt.Sprintf("range %d %d\n", r.Location, r.Length))
			}
		},
		SpeechSynthesizerDidFinishSpeechUtterance: func(_ avfaudio.AVSpeechSynthesizer, _ avfaudio.AVSpeechUtterance) {
			if event != nil {
				event("finish\n")
			}
			doneOnce.Do(func() { close(done) })
		},
		SpeechSynthesizerDidCancelSpeechUtterance: func(_ avfaudio.AVSpeechSynthesizer, _ avfaudio.AVSpeechUtterance) {
			if event != nil {
				event("cancel\n")
			}
			doneOnce.Do(func() { close(done) })
		},
	})
	synth.SetDelegate(delegate)
	if register != nil {
		register(synth)
	}
	defer func() {
		if unregister != nil {
			unregister()
		}
	}()
	synth.SpeakUtterance(utterance)
	for {
		select {
		case <-done:
			runtime.KeepAlive(delegate)
			runtime.KeepAlive(synth)
			return nil
		default:
			corefoundation.CFRunLoopRunInMode(corefoundation.KCFRunLoopDefaultMode, 0.1, false)
		}
	}
}

func newSpeechUtterance(config speechConfig) (avfaudio.AVSpeechUtterance, error) {
	if config.SSML != "" {
		u := avfaudio.NewSpeechUtteranceWithSSMLRepresentation(config.SSML)
		if u.ID == 0 {
			return avfaudio.AVSpeechUtterance{}, fmt.Errorf("invalid ssml")
		}
		return u, nil
	}
	return avfaudio.NewSpeechUtteranceWithString(config.Text), nil
}

func setSpeechFloat(name, text string, set func(float32)) error {
	text = strings.TrimSpace(text)
	if text == "" || text == "default" {
		return nil
	}
	value, err := strconv.ParseFloat(text, 32)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	set(float32(value))
	return nil
}
