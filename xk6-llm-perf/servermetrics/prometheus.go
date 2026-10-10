package servermetrics

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Sample is a single labeled reading. Nonfinite values are unavailable rather
// than valid zero. Timestamp is the optional exporter-supplied Unix millisecond
// timestamp; it does not replace the scrape query window.
type Sample struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Type        string            `json:"type"`
	Value       *float64          `json:"value"`
	Timestamp   *int64            `json:"timestamp"`
	Unavailable string            `json:"unavailable,omitempty"`
}

// Scrape retains the original bounded exposition and parsed samples. Raw data
// and labels may contain server-authored metadata; retain them deliberately.
type Scrape struct {
	SchemaVersion int       `json:"schema_version"`
	Target        string    `json:"target"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Hash          string    `json:"sha256"`
	Raw           string    `json:"raw"`
	Samples       []Sample  `json:"samples"`
}

// Read parses Prometheus text format, including escaped label values and
// optional sample timestamps. It rejects duplicate identities and inconsistent
// cumulative histogram buckets. Exemplars and OpenMetrics EOF are unsupported.
func Read(r io.Reader) ([]Sample, error) {
	const limit = 8 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read metrics: %w", err)
	}
	if len(data) > limit {
		return nil, fmt.Errorf("metrics exceed 8 MiB")
	}
	types := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var samples []Sample
	identities := map[string]bool{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			fields := strings.Fields(line)
			if len(fields) > 1 && fields[1] == "TYPE" {
				if len(fields) != 4 || !validName(fields[2], true) || !oneOf(fields[3], "counter", "gauge", "histogram", "summary", "untyped") {
					return nil, fmt.Errorf("invalid metric type declaration")
				}
				if old, ok := types[fields[2]]; ok && old != fields[3] {
					return nil, fmt.Errorf("conflicting metric type")
				}
				types[fields[2]] = fields[3]
			}
			continue
		}
		if len(samples) >= 100000 {
			return nil, fmt.Errorf("more than 100000 metric samples")
		}
		s, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		s.Type = types[s.Name]
		for _, suffix := range []string{"_bucket", "_sum", "_count"} {
			if strings.HasSuffix(s.Name, suffix) && s.Type == "" {
				s.Type = types[strings.TrimSuffix(s.Name, suffix)]
			}
		}
		if s.Type == "" {
			s.Type = "untyped"
		}
		id := identity(s.Name, s.Labels)
		if identities[id] {
			return nil, fmt.Errorf("duplicate metric identity %q", s.Name)
		}
		identities[id] = true
		samples = append(samples, s)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan metrics: %w", err)
	}
	if err := validateHistograms(samples); err != nil {
		return nil, err
	}
	return samples, nil
}

func parseLine(line string) (Sample, error) {
	var s Sample
	s.Labels = map[string]string{}
	n := strings.IndexAny(line, "{ \t")
	if n < 0 {
		return s, fmt.Errorf("metric sample has no value")
	}
	s.Name = line[:n]
	if !validName(s.Name, true) {
		return s, fmt.Errorf("invalid metric name")
	}
	rest := line[n:]
	if rest[0] == '{' {
		rest = rest[1:]
		for {
			rest = strings.TrimSpace(rest)
			if rest == "" {
				return s, fmt.Errorf("unterminated metric labels")
			}
			if rest[0] == '}' {
				rest = rest[1:]
				break
			}
			eq := strings.IndexByte(rest, '=')
			if eq < 0 {
				return s, fmt.Errorf("invalid label assignment")
			}
			key := strings.TrimSpace(rest[:eq])
			rest = strings.TrimSpace(rest[eq+1:])
			if !validName(key, false) || len(key) > 128 || len(rest) == 0 || rest[0] != '"' {
				return s, fmt.Errorf("invalid label name or value")
			}
			end := 1
			for end < len(rest) {
				if rest[end] == '\\' {
					end += 2
					continue
				}
				if rest[end] == '"' {
					break
				}
				end++
			}
			if end >= len(rest) {
				return s, fmt.Errorf("unterminated quoted label")
			}
			value, err := strconv.Unquote(rest[:end+1])
			if err != nil || len(value) > 1024 {
				return s, fmt.Errorf("invalid label escape or size")
			}
			if _, ok := s.Labels[key]; ok || len(s.Labels) >= 128 {
				return s, fmt.Errorf("duplicate label or label budget exhausted")
			}
			s.Labels[key] = value
			rest = strings.TrimSpace(rest[end+1:])
			if rest == "" {
				return s, fmt.Errorf("unterminated metric labels")
			}
			if rest[0] == ',' {
				rest = rest[1:]
				continue
			}
			if rest[0] != '}' {
				return s, fmt.Errorf("invalid label separator")
			}
		}
	}
	fields := strings.Fields(rest)
	if len(fields) < 1 || len(fields) > 2 {
		return s, fmt.Errorf("invalid metric sample fields")
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return s, fmt.Errorf("invalid metric value")
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		s.Unavailable = "nonfinite exporter value"
	} else {
		s.Value = &v
	}
	if len(fields) == 2 {
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return s, fmt.Errorf("invalid sample timestamp")
		}
		s.Timestamp = &v
	}
	return s, nil
}

func validName(s string, colon bool) bool {
	if s == "" || len(s) > 256 {
		return false
	}
	for i, c := range s {
		if c == '_' || colon && c == ':' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && unicode.IsDigit(c) && c < 128 {
			continue
		}
		return false
	}
	return true
}
func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func identity(name string, labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	for _, k := range keys {
		b.WriteByte('|')
		b.WriteString(strconv.Quote(k))
		b.WriteByte('=')
		b.WriteString(strconv.Quote(labels[k]))
	}
	return b.String()
}

type bucket struct{ bound, count float64 }

func validateHistograms(samples []Sample) error {
	groups := map[string][]bucket{}
	counts := map[string]float64{}
	for _, s := range samples {
		if s.Type != "histogram" {
			continue
		}
		labels := map[string]string{}
		for k, v := range s.Labels {
			if k != "le" {
				labels[k] = v
			}
		}
		if strings.HasSuffix(s.Name, "_count") {
			if s.Value == nil || *s.Value < 0 || math.Trunc(*s.Value) != *s.Value {
				return fmt.Errorf("invalid histogram count")
			}
			counts[identity(strings.TrimSuffix(s.Name, "_count"), labels)] = *s.Value
		}
		if !strings.HasSuffix(s.Name, "_bucket") {
			continue
		}
		raw, ok := s.Labels["le"]
		bound, err := strconv.ParseFloat(raw, 64)
		if !ok || err != nil || math.IsNaN(bound) || math.IsInf(bound, -1) || s.Value == nil || *s.Value < 0 || math.Trunc(*s.Value) != *s.Value {
			return fmt.Errorf("invalid histogram bucket")
		}
		id := identity(strings.TrimSuffix(s.Name, "_bucket"), labels)
		groups[id] = append(groups[id], bucket{bound, *s.Value})
	}
	for id, bs := range groups {
		sort.Slice(bs, func(i, j int) bool { return bs[i].bound < bs[j].bound })
		for i := 1; i < len(bs); i++ {
			if bs[i].bound == bs[i-1].bound || bs[i].count < bs[i-1].count {
				return fmt.Errorf("histogram buckets are not cumulative")
			}
		}
		last := bs[len(bs)-1]
		count, ok := counts[id]
		if !math.IsInf(last.bound, 1) || !ok || last.count != count {
			return fmt.Errorf("histogram requires matching +Inf and count")
		}
	}
	return nil
}

// Fetch scrapes an explicit HTTP(S) target using the caller's client and context.
// Discovery and authentication remain caller-owned. A response is bounded at
// eight MiB; HTTP failures do not become successful empty scrapes.
func Fetch(ctx context.Context, client *http.Client, target string) (Scrape, error) {
	var s Scrape
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || !oneOf(u.Scheme, "http", "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || client == nil {
		return s, fmt.Errorf("invalid scrape target or client")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return s, err
	}
	s = Scrape{SchemaVersion: 1, Target: target, StartedAt: time.Now()}
	resp, err := client.Do(req)
	if err != nil {
		return s, fmt.Errorf("scrape target: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return s, fmt.Errorf("scrape status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
	s.FinishedAt = time.Now()
	if err != nil {
		return s, fmt.Errorf("read scrape: %w", err)
	}
	if len(data) > 8*1024*1024 {
		return s, fmt.Errorf("scrape exceeds 8 MiB")
	}
	s.Raw = string(data)
	hash := sha256.Sum256(data)
	s.Hash = hex.EncodeToString(hash[:])
	s.Samples, err = Read(strings.NewReader(s.Raw))
	return s, err
}

// CounterRate derives a rate from compatible monotonically increasing counter
// readings. Exporter timestamps must both be present or both absent. Reset or
// changed label/type identity cannot be repaired by silently assuming rollover.
func CounterRate(before, after Sample, elapsed, maxGap time.Duration) (float64, error) {
	if before.Type != "counter" || after.Type != "counter" || identity(before.Name, before.Labels) != identity(after.Name, after.Labels) || before.Value == nil || after.Value == nil {
		return 0, fmt.Errorf("counter identity or availability mismatch")
	}
	if (before.Timestamp == nil) != (after.Timestamp == nil) {
		return 0, fmt.Errorf("counter timestamp basis changed")
	}
	if before.Timestamp != nil {
		elapsed = time.Duration(*after.Timestamp-*before.Timestamp) * time.Millisecond
	}
	if elapsed <= 0 || maxGap <= 0 || elapsed > maxGap {
		return 0, fmt.Errorf("counter interval invalid or exceeds gap bound")
	}
	if *before.Value < 0 || *after.Value < *before.Value {
		return 0, fmt.Errorf("counter reset or negative value")
	}
	return (*after.Value - *before.Value) / elapsed.Seconds(), nil
}
