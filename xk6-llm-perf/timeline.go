package llmperf

import (
	"fmt"
	"math"
	"sort"
)

// Weighted describes a duration-weighted concurrency distribution.
type Weighted struct {
	Mean     float64 `json:"mean"`
	P50      float64 `json:"p50"`
	P95      float64 `json:"p95"`
	P99      float64 `json:"p99"`
	Duration float64 `json:"duration"`
}

// Concurrency separates whole-window and busy-window distributions.
type Concurrency struct {
	Effective Weighted `json:"effective"`
	Active    Weighted `json:"active"`
}

// Timeline describes observed request concurrency, not server GPU concurrency.
type Timeline struct {
	Requests        Concurrency `json:"requests"`
	Prefill         Concurrency `json:"prefill"`
	Decode          Concurrency `json:"decode"`
	UnsplitRequests int         `json:"unsplit_requests"`
	PhasePolicy     string      `json:"phase_policy"`
}

type interval struct{ start, end float64 }
type edge struct {
	at    float64
	delta int
}
type segment struct{ value, duration float64 }

func concurrency(intervals []interval, start, end float64) Concurrency {
	edges := []edge{{start, 0}, {end, 0}}
	for _, i := range intervals {
		a, b := math.Max(start, i.start), math.Min(end, i.end)
		if a < b {
			edges = append(edges, edge{a, 1}, edge{b, -1})
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].at < edges[j].at })
	var all, active []segment
	count := 0
	prev := start
	for _, e := range edges {
		if e.at > prev {
			s := segment{float64(count), (e.at - prev) / 1000}
			all = append(all, s)
			if count > 0 {
				active = append(active, s)
			}
		}
		count += e.delta
		prev = e.at
	}
	return Concurrency{Effective: weighted(all), Active: weighted(active)}
}

func weighted(segs []segment) Weighted {
	var w Weighted
	for _, s := range segs {
		w.Duration += s.duration
		w.Mean += s.value * s.duration
	}
	if w.Duration == 0 {
		return w
	}
	w.Mean /= w.Duration
	sort.Slice(segs, func(i, j int) bool { return segs[i].value < segs[j].value })
	quantile := func(p float64) float64 {
		sum := 0.0
		for _, s := range segs {
			sum += s.duration
			if sum >= p*w.Duration {
				return s.value
			}
		}
		return segs[len(segs)-1].value
	}
	w.P50, w.P95, w.P99 = quantile(.5), quantile(.95), quantile(.99)
	return w
}

// AnalyzeTimeline reconstructs concurrency from retained request records.
// It refuses incomplete exports rather than presenting partial distributions.
func AnalyzeTimeline(run RunSummary) (Timeline, error) {
	if !finite(run.StartedAt) || !finite(run.FinishedAt) || !finite(run.Duration) || run.Attempts < 0 || run.DroppedRecords < 0 || run.SchemaVersion != 1 || run.FinishedAt <= run.StartedAt || run.Duration <= 0 {
		return Timeline{}, fmt.Errorf("invalid run window or schema")
	}
	if run.DroppedRecords > 0 || len(run.Records) != run.Attempts {
		return Timeline{}, fmt.Errorf("timeline requires all attempted request records")
	}
	var total, prefill, decode []interval
	t := Timeline{PhasePolicy: "request: dispatch to full response; prefill: dispatch to first content; decode: first to last content; nonstreaming phases unavailable"}
	for _, m := range run.Records {
		if !finite(m.DispatchedAt) || !finite(m.FinishedAt) || !finite(m.FullResponseLatency) || m.FullResponseLatency < 0 || m.SchemaVersion != 1 || !m.Attempted || !m.Recorded || m.Phase != "measurement" || m.DispatchedAt < run.StartedAt || m.FinishedAt > run.FinishedAt || m.FinishedAt < m.DispatchedAt {
			return Timeline{}, fmt.Errorf("record outside measurement window or invalid")
		}
		total = append(total, interval{m.DispatchedAt, m.FinishedAt})
		if m.TTFT == nil || m.LastContentLatency == nil {
			t.UnsplitRequests++
			continue
		}
		if !finite(*m.TTFT) || !finite(*m.LastContentLatency) || *m.TTFT < 0 || *m.LastContentLatency < *m.TTFT || *m.LastContentLatency > m.FullResponseLatency {
			return Timeline{}, fmt.Errorf("invalid content timing")
		}
		first, last := m.DispatchedAt+*m.TTFT, m.DispatchedAt+*m.LastContentLatency
		prefill = append(prefill, interval{m.DispatchedAt, first})
		decode = append(decode, interval{first, last})
	}
	t.Requests = concurrency(total, run.StartedAt, run.FinishedAt)
	t.Prefill = concurrency(prefill, run.StartedAt, run.FinishedAt)
	t.Decode = concurrency(decode, run.StartedAt, run.FinishedAt)
	return t, nil
}
