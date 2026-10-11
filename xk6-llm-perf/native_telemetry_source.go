package llmperf

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// These types mirror the additive lineage envelope at nvtelemetry revision
// 404d70ca. The mapping's capitalized JSON keys are the producer wire contract.
// Validation establishes diagnostic structure and byte identity, not acquisition
// time, canonical conversion accuracy or authenticated physical provenance.
type nativeManagedMapping struct {
	Metric, Name, Type     string
	Numerator, Denominator int64
	Integer                bool
	SourceUnit             string
}

type nativeManagedLineage struct {
	Native *struct {
		Version               uint32 `json:"version"`
		FieldID               uint16 `json:"field_id"`
		FieldName             string `json:"field_name"`
		FieldType             uint16 `json:"field_type"`
		Status                int32  `json:"status"`
		RawBits               uint64 `json:"raw_bits"`
		TimestampMicroseconds int64  `json:"timestamp_microseconds"`
		RawText               string `json:"raw_text,omitempty"`
	} `json:"native,omitempty"`
	Mapping                 nativeManagedMapping `json:"mapping"`
	MappingDigest           string               `json:"mapping_digest"`
	Schema                  string               `json:"schema"`
	SourceID                string               `json:"source_id"`
	Producer                string               `json:"producer"`
	ProducerVersion         string               `json:"producer_version"`
	NativeOrigin            string               `json:"native_origin"`
	FieldID                 string               `json:"field_id"`
	RawValue                string               `json:"raw_value"`
	Labels                  map[string]string    `json:"labels,omitempty"`
	ExpositionTimestampUnix int64                `json:"exposition_timestamp_unix,omitempty"`
	HasExpositionTimestamp  bool                 `json:"has_exposition_timestamp,omitempty"`
	RawTimestamp            string               `json:"raw_timestamp,omitempty"`
	TimestampUnit           string               `json:"timestamp_unit,omitempty"`
	CounterReset            bool                 `json:"counter_reset,omitempty"`
}

func nativeManagedSource(raw []byte, reading NativeTelemetryReading) error {
	if len(raw) > 16384 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("native telemetry invalid managed lineage size or null")
	}
	if err := nativeManagedNoNull(raw); err != nil {
		return err
	}
	var source nativeManagedLineage
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&source); err != nil {
		return fmt.Errorf("native telemetry managed lineage: %w", err)
	}
	if source.Schema != "nvtelemetry.managed.v1" {
		return fmt.Errorf("native telemetry unsupported managed lineage schema")
	}
	m := source.Mapping
	for _, v := range []string{source.SourceID, source.Producer, source.ProducerVersion, source.NativeOrigin, source.FieldID, source.RawValue, m.Metric, m.Name, m.SourceUnit} {
		if v == "" || len(v) > 256 {
			return fmt.Errorf("native telemetry invalid managed lineage identity")
		}
	}
	if (m.Type == "counter" || m.Name == "total_energy_millijoules" || m.Name == "pcie_rx_bytes" || m.Name == "pcie_tx_bytes") && !m.Integer {
		return fmt.Errorf("native telemetry inexact managed counter mapping")
	}
	encoded, _ := json.Marshal(m)
	if m.Numerator <= 0 || m.Denominator <= 0 || m.Metric != source.FieldID || m.Name != reading.Name || source.MappingDigest != fmt.Sprintf("sha256:%x", sha256.Sum256(encoded)) || (m.Type != "" && m.Type != "gauge" && m.Type != "counter") {
		return fmt.Errorf("native telemetry invalid managed mapping provenance")
	}
	value, err := strconv.ParseFloat(source.RawValue, 64)
	if err != nil || len(source.Labels) > 16 {
		return fmt.Errorf("native telemetry invalid managed raw value or labels")
	}
	// Preserve producer non-finite raw text only as unavailable diagnostics.
	// It cannot supply a canonical value or qualify a managed measurement.
	nonFinite := math.IsNaN(value) || math.IsInf(value, 0)
	if !nonFinite && reading.UnavailableReason == "non_finite" {
		return fmt.Errorf("native telemetry contradictory finite managed lineage")
	}
	if nonFinite && (reading.Evidence != "unavailable" || reading.UnavailableReason != "non_finite" || reading.Value != 0 || reading.IntValue != 0 || reading.IsInt) {
		return fmt.Errorf("native telemetry contradictory non-finite managed lineage")
	}
	for k, v := range source.Labels {
		if !nativeManagedLabel(k) || len(k)+len(v) > 256 {
			return fmt.Errorf("native telemetry invalid managed label")
		}
	}
	if len(source.RawTimestamp) > 256 || (source.HasExpositionTimestamp && (source.RawTimestamp == "" || (source.TimestampUnit != "milliseconds" && source.TimestampUnit != "seconds"))) || (!source.HasExpositionTimestamp && (source.ExpositionTimestampUnix != 0 || source.RawTimestamp != "" || source.TimestampUnit != "")) {
		return fmt.Errorf("native telemetry invalid managed publication timestamp")
	}
	if source.HasExpositionTimestamp {
		text := source.RawTimestamp
		if !json.Valid([]byte(text)) || (text[0] != '-' && (text[0] < '0' || text[0] > '9')) {
			return fmt.Errorf("native telemetry invalid decimal publication timestamp")
		}
		if i := strings.IndexAny(text, "eE"); i >= 0 {
			e, err := strconv.ParseInt(text[i+1:], 10, 32)
			if err != nil || e < -4096 || e > 4096 {
				return fmt.Errorf("native telemetry publication timestamp exponent exceeds limit")
			}
		}
		timestamp, ok := new(big.Rat).SetString(text)
		if !ok {
			return fmt.Errorf("native telemetry invalid publication timestamp")
		}
		scale := int64(1000000)
		if source.TimestampUnit == "seconds" {
			scale = 1000000000
		}
		timestamp.Mul(timestamp, big.NewRat(scale, 1))
		if !timestamp.IsInt() || !timestamp.Num().IsInt64() || timestamp.Num().Int64() != source.ExpositionTimestampUnix {
			return fmt.Errorf("native telemetry contradictory publication timestamp")
		}
	}
	if source.Native != nil && (source.Native.FieldName == "" || len(source.Native.FieldName) > 256 || len(source.Native.RawText) > 256) {
		return fmt.Errorf("native telemetry invalid native DCGM lineage")
	}
	return nil
}

// An explicitly null scalar cannot become a measured zero or absent flag.
func nativeManagedNoNull(raw []byte) error {
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		return err
	}
	var check func(any) error
	check = func(v any) error {
		switch v := v.(type) {
		case nil:
			return fmt.Errorf("native telemetry null managed lineage evidence")
		case map[string]any:
			for _, child := range v {
				if err := check(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := check(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return check(value)
}

func nativeManagedLabel(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}
