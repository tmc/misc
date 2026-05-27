package webkithost

import (
	"strings"
	"testing"

	"github.com/tmc/apple/avfaudio"
	"github.com/tmc/misc/wanix-macapp/internal/applefs"
)

func TestSpeechVoiceText(t *testing.T) {
	if got := speechVoiceQualityText(avfaudio.AVSpeechSynthesisVoiceQualityPremium); got != "premium" {
		t.Fatalf("quality = %q", got)
	}
	if got := speechVoiceGenderText(avfaudio.AVSpeechSynthesisVoiceGenderFemale); got != "female" {
		t.Fatalf("gender = %q", got)
	}
}

func TestFormatSpeechVoices(t *testing.T) {
	got, err := formatSpeechVoices([]speechVoice{{ID: "id", Name: "Name", Language: "en-US", Quality: "default", Gender: "unspecified"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"language": "en-US"`) || got[len(got)-1] != '\n' {
		t.Fatalf("voices = %q", got)
	}
}

func TestStopSpeechWithoutSynth(t *testing.T) {
	h := &Host{
		appleFS: applefs.NewRoot(),
		speech:  make(map[string]speechSynth),
	}
	id := strings.TrimSpace(readAppleFSString(h, "speech/clone"))
	if err := h.stopSpeech(id); err != nil {
		t.Fatal(err)
	}
	if got := readAppleFSString(h, "speech/"+id+"/status"); got != "status stopped\n" {
		t.Fatalf("status = %q", got)
	}
}

func TestSpeechConfig(t *testing.T) {
	h := &Host{
		appleFS: applefs.NewRoot(),
		speech:  make(map[string]speechSynth),
	}
	id := strings.TrimSpace(readAppleFSString(h, "speech/clone"))
	for name, value := range map[string]string{
		"text":      "hello\n",
		"ssml":      "<speak>hello</speak>\n",
		"voice":     "en-US\n",
		"rate":      "0.5\n",
		"pitch":     "1.1\n",
		"volume":    "0.8\n",
		"assistive": "true\n",
	} {
		if err := h.appleFS.WriteFile("speech/"+id+"/"+name, []byte(value)); err != nil {
			t.Fatal(err)
		}
	}
	config := h.speechConfig(id)
	if config.Text != "hello" || config.SSML == "" || config.Pitch != "1.1" || config.Assistive != "true" {
		t.Fatalf("config = %+v", config)
	}
}

func TestAppendSpeechEvent(t *testing.T) {
	h := &Host{
		appleFS: applefs.NewRoot(),
		speech:  make(map[string]speechSynth),
	}
	id := strings.TrimSpace(readAppleFSString(h, "speech/clone"))
	h.appendSpeechEvent(id, "start\n")
	h.appendSpeechEvent(id, "finish\n")
	if got := readAppleFSString(h, "speech/"+id+"/event"); got != "start\nfinish\n" {
		t.Fatalf("event = %q", got)
	}
}

func TestSetSpeechFloat(t *testing.T) {
	var got float32
	if err := setSpeechFloat("rate", "0.75", func(v float32) { got = v }); err != nil {
		t.Fatal(err)
	}
	if got != 0.75 {
		t.Fatalf("value = %v", got)
	}
	if err := setSpeechFloat("rate", "bad", func(float32) {}); err == nil {
		t.Fatal("bad rate succeeded")
	}
}
