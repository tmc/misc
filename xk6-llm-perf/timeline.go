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

// RateAnalysis describes a client phase-rate proxy in tokens per second.
// Missing includes failed, unsplit, zero-duration, or unavailable-count requests.
// Effective includes idle time; Active includes eligible zero-count phases and excludes idle intervals.
type RateAnalysis struct {
	Effective Weighted    `json:"effective"`
	Active    Weighted    `json:"active"`
	PerUser   Concurrency `json:"per_user"`
	Eligible  int         `json:"eligible_requests"`
	Missing   int         `json:"excluded_requests"`
	Policy    string      `json:"policy"`
}

// Timeline describes observed request concurrency, not server GPU concurrency.
type Timeline struct {
	Requests        Concurrency  `json:"requests"`
	Prefill         Concurrency  `json:"prefill"`
	Decode          Concurrency  `json:"decode"`
	UnsplitRequests int          `json:"unsplit_requests"`
	InputRate       RateAnalysis `json:"input_rate"`
	OutputRate      RateAnalysis `json:"output_rate"`
	TotalRate       RateAnalysis `json:"total_rate"`
	PhasePolicy     string       `json:"phase_policy"`
}

type interval struct{ start, end float64 }
type edge struct {
	at    float64
	delta float64
	mask  int
}
type segment struct{ value, duration float64 }

func concurrency(intervals []interval, start, end float64) Concurrency {
	rates := make([]rateInterval, 0, len(intervals))
	for _, i := range intervals {
		rates = append(rates, rateInterval{i.start, i.end, 1})
	}
	return rateDistribution(rates, start, end)
}

type rateInterval struct{ start, end, rate float64 }

func rateDistribution(intervals []rateInterval, start, end float64) Concurrency {
	total, _ := rateDistributions(intervals, start, end)
	return total
}
func rateDistributions(intervals []rateInterval, start, end float64) (Concurrency, Concurrency) {
	edges := []edge{{start, 0, 0}, {end, 0, 0}}
	for _, i := range intervals {
		a, b := math.Max(start, i.start), math.Min(end, i.end)
		if a < b {
			edges = append(edges, edge{a, i.rate, 1}, edge{b, -i.rate, -1})
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].at < edges[j].at })
	var all, active, users, activeUsers []segment
	mask := 0
	count, correction := 0.0, 0.0
	prev := start
	for _, e := range edges {
		if e.at > prev {
			s := segment{count + correction, (e.at - prev) / 1000}
			all = append(all, s)
			u := segment{0, s.duration}
			if mask > 0 {
				active = append(active, s)
				u.value = s.value / float64(mask)
				activeUsers = append(activeUsers, u)
			}
			users = append(users, u)
		}
		next := count + e.delta
		if math.Abs(count) >= math.Abs(e.delta) {
			correction += (count - next) + e.delta
		} else {
			correction += (e.delta - next) + count
		}
		count = next
		mask += e.mask
		if mask == 0 {
			count, correction = 0, 0
		}
		prev = e.at
	}
	return Concurrency{Effective: weighted(all), Active: weighted(active)}, Concurrency{Effective: weighted(users), Active: weighted(activeUsers)}
}

func weighted(segs []segment) Weighted {
	var w Weighted
	for _, s := range segs {
		w.Duration += s.duration

	}
	if w.Duration == 0 {
		return w
	}
	for _, s := range segs {
		w.Mean += s.value * (s.duration / w.Duration)
	}
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
	if run.AggregateError != "" {
		return Timeline{}, fmt.Errorf("incomplete aggregate: %s", run.AggregateError)
	}
	if !finite(run.StartedAt) || !finite(run.FinishedAt) || !finite(run.Duration) || run.Attempts < 0 || len(run.Records) > 100000 || run.DroppedRecords < 0 || run.SchemaVersion != 1 || run.FinishedAt <= run.StartedAt || run.Duration <= 0 {
		return Timeline{}, fmt.Errorf("invalid run window or schema")
	}
	if math.Abs((run.FinishedAt-run.StartedAt)/1000-run.Duration) > .001 {
		return Timeline{}, fmt.Errorf("run duration does not match window")
	}
	if run.DroppedRecords > 0 || len(run.Records) != run.Attempts {
		return Timeline{}, fmt.Errorf("timeline requires all attempted request records")
	}
	var total, prefill, decode []interval
	var inputRates, outputRates []rateInterval
	t := Timeline{PhasePolicy: "request: dispatch to full response; prefill: dispatch to first content; decode: first to last content; nonstreaming phases unavailable"}
	for _, m := range run.Records {
		if !finite(m.DispatchedAt) || !finite(m.FinishedAt) || !finite(m.FullResponseLatency) || m.FullResponseLatency < 0 || m.SchemaVersion != 1 || !m.Attempted || !m.Recorded || m.Phase != "measurement" || m.DispatchedAt < run.StartedAt || m.FinishedAt > run.FinishedAt || m.FinishedAt < m.DispatchedAt {
			return Timeline{}, fmt.Errorf("record outside measurement window or invalid")
		}
		for _, c := range []TokenCount{m.InputTokens, m.OutputTokens} {
			if c.Value != nil && *c.Value < 0 {
				return Timeline{}, fmt.Errorf("negative token count")
			}
		}
		if m.DecodeTokens != nil && *m.DecodeTokens < 0 {
			return Timeline{}, fmt.Errorf("negative decode token count")
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
	t.InputRate.Policy = "successful input tokens uniformly assigned to dispatch-to-first-content; client prefill proxy, tokens/s"
	t.OutputRate.Policy = "successful decode tokens uniformly assigned to first-to-last-content; client decode proxy, tokens/s"
	for _, m := range run.Records {
		if m.Outcome != "success" {
			continue
		}
		if m.TTFT != nil && *m.TTFT > 0 && m.InputTokens.Value != nil && *m.InputTokens.Value >= 0 {
			inputRates = append(inputRates, rateInterval{m.DispatchedAt, m.DispatchedAt + *m.TTFT, float64(*m.InputTokens.Value) * 1000 / *m.TTFT})
		}
		if m.TTFT != nil && m.LastContentLatency != nil && *m.LastContentLatency > *m.TTFT && m.DecodeTokens != nil && *m.DecodeTokens >= 0 && !m.MultiChoice {
			outputRates = append(outputRates, rateInterval{m.DispatchedAt + *m.TTFT, m.DispatchedAt + *m.LastContentLatency, float64(*m.DecodeTokens) * 1000 / (*m.LastContentLatency - *m.TTFT)})
		}
	}
	for _, i := range append(append([]rateInterval{}, inputRates...), outputRates...) {
		if !finite(i.rate) || i.end <= i.start {
			return Timeline{}, fmt.Errorf("phase rate exceeds timestamp or numeric resolution")
		}
	}
	in, inUser := rateDistributions(inputRates, run.StartedAt, run.FinishedAt)
	out, outUser := rateDistributions(outputRates, run.StartedAt, run.FinishedAt)
	t.InputRate.PerUser, t.OutputRate.PerUser = inUser, outUser
	combined := append(append([]rateInterval{}, inputRates...), outputRates...)
	totalRate, totalUser := rateDistributions(combined, run.StartedAt, run.FinishedAt)
	t.TotalRate = RateAnalysis{Effective: totalRate.Effective, Active: totalRate.Active, PerUser: totalUser, Policy: "sum of available successful input/decode phase proxies; active mask is union of eligible phases"}
	for _, m := range run.Records {
		if m.Outcome == "success" && ((m.TTFT != nil && *m.TTFT > 0 && m.InputTokens.Value != nil) || (m.TTFT != nil && m.LastContentLatency != nil && *m.LastContentLatency > *m.TTFT && m.DecodeTokens != nil && !m.MultiChoice)) {
			t.TotalRate.Eligible++
		}
	}
	t.TotalRate.Missing = len(run.Records) - t.TotalRate.Eligible
	t.InputRate.Effective, t.InputRate.Active = in.Effective, in.Active
	t.OutputRate.Effective, t.OutputRate.Active = out.Effective, out.Active
	t.InputRate.Eligible, t.InputRate.Missing = len(inputRates), len(run.Records)-len(inputRates)
	t.OutputRate.Eligible, t.OutputRate.Missing = len(outputRates), len(run.Records)-len(outputRates)
	t.Requests = concurrency(total, run.StartedAt, run.FinishedAt)
	t.Prefill = concurrency(prefill, run.StartedAt, run.FinishedAt)
	t.Decode = concurrency(decode, run.StartedAt, run.FinishedAt)
	return t, nil
}
