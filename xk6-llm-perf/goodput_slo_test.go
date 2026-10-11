package llmperf

import (
	"encoding/json"
	"fmt"
	"go.k6.io/k6/v2/metrics"
	"math"
	"testing"
	"time"
)

func TestGoodputSimultaneousSLO(t *testing.T) {
	for _, tc := range []struct {
		name       string
		config     Config
		tokens     int
		firstChunk *int
		outcome    string
		want       bool
	}{
		{"all at boundary", Config{MaxTTFT: 100, MaxTPOT: 50, MaxE2E: 300}, 6, intPtr(2), "success", true},
		{"first chunk fallback", Config{MaxTPOT: 40}, 6, nil, "success", true},
		{"tpot fails", Config{MaxTPOT: 49}, 6, intPtr(2), "success", false},
		{"e2e fails", Config{MaxE2E: 299}, 6, intPtr(2), "success", false},
		{"ttft fails", Config{MaxTTFT: 99}, 6, intPtr(2), "success", false},
		{"one token unavailable", Config{MaxTPOT: 50}, 1, nil, "success", false},
		{"zero tokens unavailable", Config{MaxTPOT: 50}, 0, nil, "success", false},
		{"zero disabled", Config{}, 0, nil, "success", true},
		{"failure never good", Config{}, 6, nil, "failure", false},
		{"cancelled never good", Config{}, 6, nil, "cancelled", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			origin := time.Unix(1000, 0)
			c := &Client{config: tc.config}
			m := &Measurement{Attempted: true, requestStart: origin, DispatchedAt: millis(origin), first: origin.Add(100 * time.Millisecond), last: origin.Add(300 * time.Millisecond), responseEnd: origin.Add(900 * time.Millisecond), reported: Usage{CompletionTokens: tc.tokens, completionPresent: true}, counts: &TokenCounts{FirstChunkTokens: tc.firstChunk}, Outcome: tc.outcome}
			c.finishMeasurement(m)
			if m.Good != tc.want {
				t.Fatalf("good=%v; measurement=%+v", m.Good, m)
			}
			if m.SLO.MaxTPOT == nil || *m.SLO.MaxTPOT != tc.config.MaxTPOT || m.SLO.MaxE2E == nil || *m.SLO.MaxE2E != tc.config.MaxE2E {
				t.Fatal("applied limits unavailable")
			}
		})
	}
}

func TestGoodputUnavailableAndInvalidOperands(t *testing.T) {
	c := &Client{config: Config{MaxTPOT: 50, MaxE2E: 300}}
	for _, value := range []*float64{nil, floatPtr(math.NaN()), floatPtr(math.Inf(1)), floatPtr(-1), floatPtr(51)} {
		if c.measurementGood(&Measurement{ITL: value, LastContentLatency: floatPtr(300)}) {
			t.Fatal("invalid TPOT admitted")
		}
	}
	for _, config := range []Config{{MaxTPOT: -1}, {MaxE2E: -1}} {
		if validateConfig(config) == nil {
			t.Fatal("negative limit admitted")
		}
	}
}

func TestSLOReceiptPresenceAndValueEquality(t *testing.T) {
	var legacy, explicit, null SLOReceipt
	for data, dst := range map[string]*SLOReceipt{`{"max_ttft_ms":0,"max_ttfo_ms":0,"max_chunk_latency_ms":0}`: &legacy, `{"max_tpot_ms":0,"max_e2e_ms":0}`: &explicit, `{"max_tpot_ms":null,"max_e2e_ms":null}`: &null} {
		if err := json.Unmarshal([]byte(data), dst); err != nil {
			t.Fatal(err)
		}
	}
	if legacy.MaxTPOT != nil || null.MaxTPOT != nil || explicit.MaxTPOT == nil || !legacy.Equal(null) || legacy.Equal(explicit) {
		t.Fatal("availability fabricated")
	}
	data, _ := json.Marshal(legacy)
	if string(data) != `{"max_ttft_ms":0,"max_ttfo_ms":0,"max_chunk_latency_ms":0}` {
		t.Fatalf("legacy canonical receipt changed: %s", data)
	}
	copied := SLOReceipt{MaxTPOT: floatPtr(0), MaxE2E: floatPtr(0)}
	if !explicit.Equal(copied) {
		t.Fatal("equal values depend on pointer identity")
	}
	copied.MaxTPOT = floatPtr(1)
	if explicit.Equal(copied) {
		t.Fatal("distinct applied TPOT equal")
	}
}

func ExampleSLOReceipt_Equal() {
	zero := 0.0
	a := SLOReceipt{MaxTPOT: &zero, MaxE2E: &zero}
	fmt.Println(a.Equal(SLOReceipt{}))
	fmt.Println(a.Equal(a))
	// Output:
	// false
	// true
}

func TestFractionalSLOBoundaries(t *testing.T) {
	for _, metric := range []string{"ttft", "tpot", "e2e"} {
		for _, tc := range []struct {
			name  string
			delta int
			want  bool
		}{{"below", -1, true}, {"equal", 0, true}, {"above", 1, false}} {
			t.Run(metric+"/"+tc.name, func(t *testing.T) {
				limit := .25
				value := limit
				if tc.delta < 0 {
					value = math.Nextafter(limit, 0)
				} else if tc.delta > 0 {
					value = math.Nextafter(limit, math.Inf(1))
				}
				c := &Client{}
				m := &Measurement{}
				switch metric {
				case "ttft":
					c.config.MaxTTFT = limit
					m.TTFT = &value
				case "tpot":
					c.config.MaxTPOT = limit
					m.ITL = &value
				case "e2e":
					c.config.MaxE2E = limit
					m.LastContentLatency = &value
				}
				if c.measurementGood(m) != tc.want {
					t.Fatal("fractional threshold rounded")
				}
			})
		}
	}
	for _, invalid := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, config := range []Config{{MaxTTFT: invalid}, {MaxTTFO: invalid}, {MaxTokenLatency: invalid}, {MaxTPOT: invalid}, {MaxE2E: invalid}} {
			if validateConfig(config) == nil {
				t.Fatal("nonfinite or negative threshold accepted")
			}
		}
	}
	// Limits are compared as milliseconds without a duration conversion.
	if !validSLOLimit(math.MaxFloat64) {
		t.Fatal("finite bound unexpectedly converted to duration")
	}
	var c Config
	if json.Unmarshal([]byte(`{"maxTPOT":1e309}`), &c) == nil {
		t.Fatal("overflowing JSON threshold accepted")
	}
}

func TestReplayExtendedSLOAccounting(t *testing.T) {
	for _, metric := range []string{"tpot", "e2e"} {
		r := formulaRun()
		m := &r.Records[0]
		m.SLO = &SLOReceipt{MaxTPOT: floatPtr(0), MaxE2E: floatPtr(0)}
		m.Good = true
		r.GoodRequests = 1
		r.Goodput = 1
		if metric == "tpot" {
			*m.SLO.MaxTPOT = 20
		} else {
			*m.SLO.MaxE2E = 80
		}
		if _, err := AnalyzeRequests(r); err != nil {
			t.Fatal("boundary replay refused", err)
		}
		if metric == "tpot" {
			*m.SLO.MaxTPOT = math.Nextafter(20, 0)
		} else {
			*m.SLO.MaxE2E = math.Nextafter(80, 0)
		}
		if _, err := AnalyzeRequests(r); err == nil {
			t.Fatal("false good replay admitted")
		}
		m.Good = false
		r.GoodRequests = 0
		r.Goodput = 0
		if _, err := AnalyzeRequests(r); err != nil {
			t.Fatal("valid failed SLO replay refused", err)
		}
	}
}

func TestGoodputCountersRetainExplicitZero(t *testing.T) {
	for _, tc := range []struct {
		name, outcome          string
		limit                  float64
		good, success, failure float64
	}{
		{"all good", "success", 0, 1, 1, 0},
		{"all failure", "failure", 0, 0, 0, 1},
		{"zero good success", "success", 1, 0, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := setupTestClient(t, "http://example.test")
			c.config.MaxTPOT = tc.limit
			samples := make(chan metrics.SampleContainer, 100)
			c.metrics.samples = samples
			origin := time.Unix(1000, 0)
			c.finishMeasurement(&Measurement{Attempted: true, Recorded: true, requestStart: origin, DispatchedAt: millis(origin), responseEnd: origin.Add(time.Millisecond), Outcome: tc.outcome})
			values := metricValues(samples)
			for name, want := range map[string]float64{"llm_attempts": 1, "llm_good_requests": tc.good, "llm_successes": tc.success, "llm_failures": tc.failure, "llm_slo_compliance": tc.good, "llm_good_request": tc.good} {
				got := values[name]
				if len(got) != 1 || got[0] != want {
					t.Fatalf("%s=%v; want explicit [%g]", name, got, want)
				}
			}
		})
	}
}
