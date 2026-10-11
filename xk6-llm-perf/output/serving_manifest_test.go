package output

import (
	"encoding/json"
	llm "github.com/tmc/misc/xk6-llm-perf"
	"strings"
	"testing"
)

func TestServingManifestReceipt(t *testing.T) {
	r := runFixture()
	r.ServingManifest = &llm.ServingManifest{SchemaVersion: 1, Arm: llm.ServingArm{EndpointIdentity: "private-endpoint"}}
	d, err := Summarize(r)
	if err != nil {
		t.Fatal(err)
	}
	if d.ServingManifest == nil || len(d.ServingManifest.ManifestDigest) != 64 || d.ServingManifest.WorkHash != "" || d.ServingManifest.WorkUnavailable == "" {
		t.Fatalf("receipt=%+v", d.ServingManifest)
	}
	data, _ := json.Marshal(d)
	if strings.Contains(string(data), "private-endpoint") {
		t.Fatal("copied arbitrary endpoint identity into sanitized export")
	}
	first := d.ServingManifest.ManifestDigest
	r.ServingManifest.Arm.Backend = "different"
	d, err = Summarize(r)
	if err != nil || d.ServingManifest.ManifestDigest == first {
		t.Fatal("manifest change not bound")
	}
	r.ServingManifest.SchemaVersion = 2
	if _, err := Summarize(r); err == nil {
		t.Fatal("invalid manifest exported")
	}
	r.ServingManifest = nil
	d, err = Summarize(r)
	if err != nil || d.ServingManifest != nil {
		t.Fatal("legacy run changed")
	}
}
