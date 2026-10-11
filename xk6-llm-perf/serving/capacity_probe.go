package serving

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

// CapacityProbe is a hash-bound functional null-server receipt. Its sampled
// resource peaks do not prove continuous resource headroom or parser overhead.
type CapacityProbe struct {
	VerifiedRequestBodyBytes            int64    `json:"-"`
	VerifiedResponseBodyBytes           int64    `json:"-"`
	VerifiedTransportProfile            string   `json:"-"`
	MaxConnsPerHost                     int      `json:"max_conns_per_host"`
	VerifiedPipelineAttempted           uint64   `json:"-"`
	VerifiedPipelineMaxWaitMilliseconds float64  `json:"-"`
	PeakFileDescriptors                 uint64   `json:"peak_file_descriptors"`
	FileDescriptorLimit                 *uint64  `json:"file_descriptor_limit"`
	ProducerSHA256                      string   `json:"probe_sha256"`
	TicksPerSecond                      float64  `json:"ticks_per_second"`
	PeakCPUPercent                      float64  `json:"peak_cpu_percent"`
	PeakRSSBytes                        uint64   `json:"peak_rss_bytes"`
	PeakSocketDescriptors               uint64   `json:"peak_socket_descriptors"`
	AllocatedVUs                        int      `json:"allocated_vus"`
	MaxDispatchLagMilliseconds          *float64 `json:"max_dispatch_lag_ms"`
	ParsingNanoseconds                  *float64 `json:"parsing_nanoseconds"`
	SerializationNanoseconds            *float64 `json:"serialization_nanoseconds"`
	WorkloadGeometry                    string   `json:"workload_geometry"`
	SchemaVersion                       int      `json:"schema_version"`
	Kind                                string   `json:"kind"`
	BinarySHA256                        string   `json:"binary_sha256"`
	ScriptSHA256                        string   `json:"script_sha256"`
	ResourcesSHA256                     string   `json:"resources_sha256"`
	WireSHA256                          string   `json:"wire_sha256"`
	RowsSHA256                          string   `json:"rows_sha256"`
	SummarySHA256                       string   `json:"summary_sha256"`
	ExitCode                            int      `json:"exit_code"`
	Rate                                int      `json:"rate"`
	Duration                            int      `json:"duration"`
	Attempted                           int      `json:"attempted"`
	Accepted                            int      `json:"accepted"`
	WireRequests                        int      `json:"wire_requests"`
}

// VerifyCapacityProbe checks archived raw observations, complete offered work,
// sample delivery and input hashes. It verifies this functional capacity cell;
// it does not promote caller-provided resource bounds to measured qualification.
func VerifyCapacityProbe(directory, expectedBinarySHA256 string) (CapacityProbe, error) {
	var p CapacityProbe
	read := func(name string, dst any) error {
		f, err := os.Open(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
		if err != nil {
			return err
		}
		if len(data) > 32<<20 {
			return fmt.Errorf("capacity probe: byte limit exceeded")
		}
		if err = validateCapacityJSON(data, false); err != nil {
			return err
		}
		d := json.NewDecoder(bytes.NewReader(data))
		if err = d.Decode(dst); err != nil {
			return err
		}
		var extra any
		if err = d.Decode(&extra); err != io.EOF {
			return fmt.Errorf("capacity probe: trailing JSON in %s", name)
		}
		return nil
	}
	if err := read("receipt.json", &p); err != nil {
		return p, err
	}
	if p.MaxConnsPerHost < 0 || p.MaxConnsPerHost > 256 || p.AllocatedVUs < 1 || p.AllocatedVUs > 256 {
		return p, fmt.Errorf("capacity probe: invalid transport declaration")
	}
	if p.SchemaVersion != 1 || p.Kind != "registered-k6-fixed-response-control" || p.ExitCode != 0 || p.BinarySHA256 != expectedBinarySHA256 || len(expectedBinarySHA256) != 64 || p.Rate <= 0 || p.Duration <= 0 || p.Rate > 1000 || p.Duration > 30 {
		return p, fmt.Errorf("capacity probe: invalid identity or outcome")
	}
	for _, x := range []struct{ name, hash string }{{"probe.js", p.ScriptSHA256}, {"resources.json", p.ResourcesSHA256}, {"wire.json", p.WireSHA256}, {"rows.json", p.RowsSHA256}, {"summary.json", p.SummarySHA256}} {
		if _, err := hex.DecodeString(x.hash); err != nil || len(x.hash) != 64 {
			return p, fmt.Errorf("capacity probe: invalid %s digest", x.name)
		}
		f, err := os.Open(filepath.Join(directory, x.name))
		if err != nil {
			return p, err
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(f, 32<<20+1))
		f.Close()
		if err != nil {
			return p, err
		}
		if n > 32<<20 || hex.EncodeToString(h.Sum(nil)) != x.hash {
			return p, fmt.Errorf("capacity probe: %s hash mismatch", x.name)
		}
	}
	declared := p.Rate * p.Duration
	expected := p.Attempted
	if expected < declared || expected > declared+1 {
		return p, fmt.Errorf("capacity probe: arrivals exceed declared inclusive-end boundary policy")
	}
	var wire []float64
	if err := read("wire.json", &wire); err != nil {
		return p, err
	}
	var rows []struct {
		ScheduledAt float64 `json:"scheduledAt"`
		Lag         float64 `json:"lag"`
		Start       float64 `json:"start"`
		Finish      float64 `json:"finish"`
		VU          int     `json:"vu"`
		Iteration   int     `json:"iteration"`
		Result      struct {
			Measurement struct {
				TransportProfile      string   `json:"transport_profile"`
				MaxConnectionsPerHost int      `json:"max_connections_per_host"`
				Stream                bool     `json:"stream"`
				RequestBodyBytes      int64    `json:"request_body_bytes"`
				ResponseBodyBytes     int64    `json:"response_body_bytes"`
				Protocol              string   `json:"protocol"`
				Outcome               string   `json:"outcome"`
				Parsing               *float64 `json:"parsing_latency_ms"`
				Serialization         *float64 `json:"serialization_latency_ms"`
				Work                  struct {
					Status               string   `json:"status"`
					ObservedModel        string   `json:"observed_model"`
					ExactUsage           bool     `json:"exact_usage"`
					ReportedOutputTokens *int     `json:"reported_output_tokens"`
					FinishReasons        []string `json:"finish_reasons"`
					Expectation          struct {
						ExpectedModel        string   `json:"expectedModel"`
						RequireExactUsage    bool     `json:"requireExactUsage"`
						MinOutputTokens      *int     `json:"minOutputTokens"`
						MaxOutputTokens      *int     `json:"maxOutputTokens"`
						AllowedFinishReasons []string `json:"allowedFinishReasons"`
					} `json:"expectation"`
				} `json:"work"`
			} `json:"measurement"`
		} `json:"result"`
		Pipeline struct {
			MaxWaitNanoseconds              *uint64 `json:"max_wait_nanoseconds"`
			Generation                      uint64  `json:"generation"`
			Attempted, Delivered, Cancelled uint64
			Inflight                        int64
		} `json:"pipeline"`
	}
	if err := read("rows.json", &rows); err != nil {
		return p, err
	}
	if len(wire) != expected || len(rows) != expected || p.Attempted != expected || p.Accepted != expected || p.WireRequests != expected {
		return p, fmt.Errorf("capacity probe: incomplete offered population")
	}
	parsingSum, serializationSum := 0., 0.
	maxLag := 0.
	scheduleOrigin := 0.
	seen := map[int]bool{}
	last := map[int]int{}
	for i, r := range rows {
		if seen[r.Iteration] || r.VU <= 0 || r.Iteration < 0 || r.Iteration >= expected || r.Result.Measurement.Outcome != "success" || r.Result.Measurement.Work.Status != "accepted" || r.Pipeline.Generation != 1 || r.Pipeline.Attempted == 0 || r.Pipeline.Attempted != r.Pipeline.Delivered || r.Pipeline.Cancelled != 0 || r.Pipeline.Inflight != 0 {
			return p, fmt.Errorf("capacity probe: invalid accepted-work or pipeline row")
		}
		if r.Start <= 0 || r.Finish < r.Start || r.ScheduledAt <= 0 || math.IsNaN(r.Start) || math.IsInf(r.Start, 0) || math.IsNaN(r.Finish) || math.IsInf(r.Finish, 0) || math.IsNaN(r.ScheduledAt) || math.IsInf(r.ScheduledAt, 0) {
			return p, fmt.Errorf("capacity probe: invalid operation timestamps")
		}
		origin := r.ScheduledAt - float64(r.Iteration)*1000/float64(p.Rate)
		if i == 0 {
			scheduleOrigin = origin
		}
		if math.Abs(origin-scheduleOrigin) > .002 {
			return p, fmt.Errorf("capacity probe: offered schedule contradiction")
		}
		lag := math.Max(0, r.Start-r.ScheduledAt)
		if math.Abs(lag-r.Lag) > .002 {
			return p, fmt.Errorf("capacity probe: dispatch lag contradiction")
		}
		if lag > maxLag {
			maxLag = lag
		}
		measurement := r.Result.Measurement
		if p.MaxConnsPerHost > 0 {
			if measurement.MaxConnectionsPerHost != p.MaxConnsPerHost || measurement.TransportProfile != "shared-http2-enabled-v1" || measurement.Stream || measurement.Protocol != "openai-chat" || measurement.RequestBodyBytes <= 0 || measurement.ResponseBodyBytes <= 0 {
				return p, fmt.Errorf("capacity probe: effective transport or wire geometry mismatch")
			}
			if i == 0 {
				p.VerifiedRequestBodyBytes = measurement.RequestBodyBytes
				p.VerifiedResponseBodyBytes = measurement.ResponseBodyBytes
				p.VerifiedTransportProfile = measurement.TransportProfile
			}
			if measurement.RequestBodyBytes != p.VerifiedRequestBodyBytes || measurement.ResponseBodyBytes != p.VerifiedResponseBodyBytes {
				return p, fmt.Errorf("capacity probe: wire geometry changed")
			}
		}
		w := r.Result.Measurement.Work
		if w.ObservedModel != "capacity-fixture" || !w.ExactUsage || w.ReportedOutputTokens == nil || *w.ReportedOutputTokens != 1 || len(w.FinishReasons) != 1 || w.FinishReasons[0] != "stop" || w.Expectation.ExpectedModel != "capacity-fixture" || !w.Expectation.RequireExactUsage || w.Expectation.MinOutputTokens == nil || *w.Expectation.MinOutputTokens != 1 || w.Expectation.MaxOutputTokens == nil || *w.Expectation.MaxOutputTokens != 1 || len(w.Expectation.AllowedFinishReasons) != 1 || w.Expectation.AllowedFinishReasons[0] != "stop" {
			return p, fmt.Errorf("capacity probe: missing fixed-response work policy or evidence")
		}
		for _, cost := range []*float64{r.Result.Measurement.Parsing, r.Result.Measurement.Serialization} {
			if cost == nil || *cost < 0 || math.IsNaN(*cost) || math.IsInf(*cost, 0) {
				return p, fmt.Errorf("capacity probe: missing measured processing cost")
			}
		}
		parsingSum += *r.Result.Measurement.Parsing * 1e6
		serializationSum += *r.Result.Measurement.Serialization * 1e6
		if previous, ok := last[r.VU]; ok && r.Pipeline.Attempted <= rows[previous].Pipeline.Attempted {
			return p, fmt.Errorf("capacity probe: pipeline counters did not advance")
		}
		seen[r.Iteration] = true
		last[r.VU] = i
	}
	if p.ParsingNanoseconds == nil || p.SerializationNanoseconds == nil || math.Abs(parsingSum-*p.ParsingNanoseconds) > .001 || math.Abs(serializationSum-*p.SerializationNanoseconds) > .001 {
		return p, fmt.Errorf("capacity probe: processing cost coverage or sum disagreement")
	}
	if p.MaxDispatchLagMilliseconds == nil || math.Abs(maxLag-*p.MaxDispatchLagMilliseconds) > .002 {
		return p, fmt.Errorf("capacity probe: lag peak disagreement")
	}
	for _, row := range rows {
		if row.Pipeline.MaxWaitNanoseconds == nil {
			return p, fmt.Errorf("capacity probe: pipeline maximum wait unavailable")
		}
	}
	for _, index := range last {
		pipeline := rows[index].Pipeline
		p.VerifiedPipelineAttempted += pipeline.Attempted
		wait := float64(*pipeline.MaxWaitNanoseconds) / 1e6
		if wait > p.VerifiedPipelineMaxWaitMilliseconds {
			p.VerifiedPipelineMaxWaitMilliseconds = wait
		}
	}
	var summary struct {
		Metrics map[string]struct {
			Values struct {
				Count float64 `json:"count"`
			} `json:"values"`
		} `json:"metrics"`
	}
	if err := read("summary.json", &summary); err != nil {
		return p, err
	}
	if summary.Metrics["dropped_iterations"].Values.Count != 0 || summary.Metrics["iterations"].Values.Count != float64(expected) {
		return p, fmt.Errorf("capacity probe: incomplete executor population")
	}
	type resourceObservation struct {
		At                                           float64
		Ticks, StartTicks, RSS, Sockets, Descriptors uint64
		DescriptorLimit                              *uint64
	}
	var observations []struct {
		At              *float64 `json:"at"`
		Ticks           *uint64  `json:"ticks"`
		StartTicks      *uint64  `json:"start_ticks"`
		RSS             *uint64  `json:"rss"`
		Sockets         *uint64  `json:"sockets"`
		Descriptors     *uint64  `json:"descriptors"`
		DescriptorLimit *uint64  `json:"descriptor_limit"`
	}
	if err := read("resources.json", &observations); err != nil {
		return p, err
	}
	resources := make([]resourceObservation, len(observations))
	for i, r := range observations {
		// Missing observations must not become available zero measurements.
		// Descriptor limits may be unavailable; sampled occupancy may not.
		if r.At == nil || r.Ticks == nil || r.StartTicks == nil || r.RSS == nil || r.Sockets == nil || r.Descriptors == nil {
			return p, fmt.Errorf("capacity probe: resource observation %d missing required numeric field", i)
		}
		resources[i] = resourceObservation{*r.At, *r.Ticks, *r.StartTicks, *r.RSS, *r.Sockets, *r.Descriptors, r.DescriptorLimit}
	}
	if len(resources) < 2 || p.TicksPerSecond <= 0 || math.IsNaN(p.TicksPerSecond) || math.IsInf(p.TicksPerSecond, 0) {
		return p, fmt.Errorf("capacity probe: missing resource observations")
	}
	peakCPU := 0.
	peakRSS, peakSockets := uint64(0), uint64(0)
	peakFD := uint64(0)
	for i, r := range resources {
		if math.IsNaN(r.At) || math.IsInf(r.At, 0) || r.StartTicks != resources[0].StartTicks {
			return p, fmt.Errorf("capacity probe: resource identity or timestamp invalid")
		}
		if r.Descriptors > peakFD {
			peakFD = r.Descriptors
		}
		if p.FileDescriptorLimit != nil && (r.DescriptorLimit == nil || *r.DescriptorLimit != *p.FileDescriptorLimit || r.Descriptors > *r.DescriptorLimit) {
			return p, fmt.Errorf("capacity probe: descriptor limit or occupancy inconsistent")
		}
		if r.RSS > peakRSS {
			peakRSS = r.RSS
		}
		if r.Sockets > peakSockets {
			peakSockets = r.Sockets
		}
		if i > 0 {
			previous := resources[i-1]
			delta := r.At - previous.At
			if delta <= 0 || delta > .25 || r.Ticks < previous.Ticks {
				return p, fmt.Errorf("capacity probe: resource sampling gap or counter reversal")
			}
			cpu := float64(r.Ticks-previous.Ticks) / p.TicksPerSecond / delta * 100
			if math.IsNaN(cpu) || math.IsInf(cpu, 0) {
				return p, fmt.Errorf("capacity probe: invalid sampled CPU")
			}
			if cpu > peakCPU {
				peakCPU = cpu
			}
		}
	}
	for _, row := range rows {
		if row.Start/1000 < resources[0].At || row.Finish/1000 > resources[len(resources)-1].At {
			return p, fmt.Errorf("capacity probe: resource observations do not bracket load")
		}
	}
	if p.FileDescriptorLimit != nil && peakFD != p.PeakFileDescriptors {
		return p, fmt.Errorf("capacity probe: descriptor peak disagrees")
	}
	if math.Abs(peakCPU-p.PeakCPUPercent) > 1e-6 || peakRSS != p.PeakRSSBytes || peakSockets != p.PeakSocketDescriptors {
		return p, fmt.Errorf("capacity probe: derived resource peaks disagree")
	}

	return p, nil
}

// ProbeCapacityEvidence verifies an archived fixed-response cell and returns its
// independently observed fields. ConnectionLimit is available only when every
// record binds the declared cap to the shared transport profile.
func ProbeCapacityEvidence(directory, binarySHA256 string) (CapacityEvidence, error) {
	p, err := VerifyCapacityProbe(directory, binarySHA256)
	if err != nil {
		return CapacityEvidence{}, err
	}
	cpu, rss, connections := p.PeakCPUPercent, float64(p.PeakRSSBytes), float64(p.PeakSocketDescriptors)
	cap := float64(p.MaxConnsPerHost)
	var limit *float64
	if p.MaxConnsPerHost > 0 && p.AllocatedVUs > 0 {
		limit = &cap
	}
	return CapacityEvidence{SchemaVersion: 1, RunID: p.ScriptSHA256, ServerKind: "fixed-response", Parameters: map[string]string{"binary_sha256": p.BinarySHA256, "script_sha256": p.ScriptSHA256, "workload_geometry": p.WorkloadGeometry}, Preconditions: []string{"client PID observations bracket workload at finite sampling cadence", "owned fixed-response fixture shares host; parsing excludes I/O and framing"}, DurationSeconds: float64(p.Duration), OfferedPerSecond: float64(p.Rate), Scheduled: int64(p.Attempted), Attempts: int64(p.Attempted), Successes: int64(p.Accepted), MaxLagMilliseconds: p.MaxDispatchLagMilliseconds, PeakCPUPercent: &cpu, PeakRSSBytes: &rss, PeakConnections: &connections, ConnectionLimit: limit, PipelineAttempted: p.VerifiedPipelineAttempted, PipelineDelivered: p.VerifiedPipelineAttempted, PipelineMaxWaitMilliseconds: &p.VerifiedPipelineMaxWaitMilliseconds, ResourceCoverageComplete: true, ParsingNanoseconds: p.ParsingNanoseconds, SerializationNanoseconds: p.SerializationNanoseconds}, nil
}

// EvaluateVerifiedProbe checks declared resource bounds against verified raw
// observations. A geometry mismatch is an error. SAMPLED_CAPACITY qualifies
// only the declared finite-cadence protocol, not continuous resource maxima or
// another workload geometry. Missing required observations stay unqualified.
func EvaluateVerifiedProbe(directory, binarySHA256, geometry string, bounds CapacityBounds) (CapacityResult, error) {
	evidence, err := ProbeCapacityEvidence(directory, binarySHA256)
	if err != nil {
		return CapacityResult{}, err
	}
	if geometry == "" || evidence.Parameters["workload_geometry"] != geometry {
		return CapacityResult{}, fmt.Errorf("capacity probe: workload geometry mismatch")
	}
	result, err := QualifyCapacity(evidence, bounds)
	if err != nil {
		return result, err
	}
	if result.Status == "DIAGNOSTIC" && evidence.ConnectionLimit != nil && evidence.ResourceCoverageComplete && evidence.ParsingNanoseconds != nil && evidence.SerializationNanoseconds != nil {
		result.Status = "SAMPLED_CAPACITY"
		result.Reasons = []string{"sampled-capacity protocol: finite cadence resource observations bracket every operation; unsampled spikes remain unknown"}
	}

	return result, nil
}

// VerifyPinnedCapacityProbe additionally checks a producer hash declared before
// the run. Hashes establish archive identity, not authenticity of execution.
func VerifyPinnedCapacityProbe(directory, binarySHA256, producerSHA256 string) (CapacityProbe, error) {
	p, err := VerifyCapacityProbe(directory, binarySHA256)
	if err != nil {
		return p, err
	}
	if len(producerSHA256) != 64 || p.ProducerSHA256 != producerSHA256 {
		return p, fmt.Errorf("capacity probe: producer pin mismatch")
	}
	f, err := os.Open(filepath.Join(directory, "producer.py"))
	if err != nil {
		return p, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return p, err
	}
	if n > 1<<20 || hex.EncodeToString(h.Sum(nil)) != producerSHA256 {
		return p, fmt.Errorf("capacity probe: producer archive mismatch")
	}
	return p, nil
}
