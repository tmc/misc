package llmperf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// ServingSetting separates requested values from effective values observed by a
// server-side probe. EvidenceDigest identifies the probe receipt, not a claim
// that its contents or the server have been independently verified.
type ServingSetting struct {
	Configured     json.RawMessage `json:"configured,omitempty"`
	Effective      json.RawMessage `json:"effective,omitempty"`
	EvidenceDigest string          `json:"evidence_digest,omitempty"`
}

// ServingArm identifies the artifact under comparison. These fields are omitted
// from ServingWorkHash so different implementations can serve comparable work.
type ServingArm struct {
	BinaryDigest     string `json:"binary_digest,omitempty"`
	SourceRevision   string `json:"source_revision,omitempty"`
	Backend          string `json:"backend,omitempty"`
	EndpointIdentity string `json:"endpoint_identity,omitempty"`
}

// ServingManifest binds effective serving settings to content identities.
// Empty identities and absent effective settings mean unknown, never defaults.
// Digests are lowercase SHA-256 hex; contents must exclude credentials and raw
// prompts. EnvironmentDigest covers comparable hardware and software conditions.
type ServingManifest struct {
	SchemaVersion     int                       `json:"schema_version"`
	Arm               ServingArm                `json:"arm"`
	ModelDigest       string                    `json:"model_digest,omitempty"`
	TokenizerDigest   string                    `json:"tokenizer_digest,omitempty"`
	TemplateDigest    string                    `json:"template_digest,omitempty"`
	WorkloadDigest    string                    `json:"workload_digest,omitempty"`
	EnvironmentDigest string                    `json:"environment_digest,omitempty"`
	Settings          map[string]ServingSetting `json:"settings,omitempty"`
	Confounds         []string                  `json:"confounds,omitempty"`
}

var servingSettingNames = []string{"cache", "compile", "device", "dtype", "max_output_tokens", "resources", "scheduler", "seed", "temperature", "top_k", "top_p"}

func servingDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	b, err := hex.DecodeString(s)
	return err == nil && hex.EncodeToString(b) == s
}

// ValidateServingManifest checks syntax and bounds without qualifying unknown
// settings. Numeric settings use exact decimal semantics; nonzero exponents
// must be between -4096 and 4096, and magnitude cannot exceed math.MaxFloat64.
// Use ServingWorkHash when complete comparable-work evidence is needed.
func ValidateServingManifest(m ServingManifest) error {
	if m.SchemaVersion != 1 {
		return fmt.Errorf("unsupported serving manifest schema")
	}
	for _, s := range []string{m.ModelDigest, m.TokenizerDigest, m.TemplateDigest, m.WorkloadDigest, m.EnvironmentDigest, m.Arm.BinaryDigest} {
		if s != "" && !servingDigest(s) {
			return fmt.Errorf("invalid serving identity digest")
		}
	}
	for _, c := range m.Arm.EndpointIdentity {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return fmt.Errorf("endpoint identity must be an opaque identifier")
		}
	}
	for _, s := range []string{m.Arm.SourceRevision, m.Arm.Backend, m.Arm.EndpointIdentity} {
		if len(s) > 256 {
			return fmt.Errorf("serving arm identity exceeds limit")
		}
	}
	if len(m.Confounds) > 32 {
		return fmt.Errorf("too many serving confounds")
	}
	for _, s := range m.Confounds {
		if s == "" || len(s) > 1024 {
			return fmt.Errorf("invalid serving confound")
		}
	}
	for name, setting := range m.Settings {
		i := sort.SearchStrings(servingSettingNames, name)
		if i == len(servingSettingNames) || servingSettingNames[i] != name {
			return fmt.Errorf("unknown serving setting %q", name)
		}
		if setting.EvidenceDigest != "" && !servingDigest(setting.EvidenceDigest) {
			return fmt.Errorf("invalid setting evidence digest")
		}
		for _, v := range []json.RawMessage{setting.Configured, setting.Effective} {
			if len(v) > 4096 {
				return fmt.Errorf("serving setting exceeds limit")
			}
			if len(v) > 0 {
				if err := servingJSON(v); err != nil {
					return fmt.Errorf("setting %s: %w", name, err)
				}
				if err := servingSettingValue(name, v); err != nil {
					return err
				}
			}
		}
		if len(setting.Effective) == 0 && setting.EvidenceDigest != "" {
			return fmt.Errorf("setting evidence has no effective value")
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(data) > 65536 {
		return fmt.Errorf("serving manifest exceeds 64 KiB")
	}
	return nil
}

// ReadServingManifest reads one strict, bounded version-1 manifest.
func ReadServingManifest(r io.Reader) (*ServingManifest, error) {
	data, err := io.ReadAll(io.LimitReader(r, 65537))
	if err != nil {
		return nil, fmt.Errorf("read serving manifest: %w", err)
	}
	if len(data) > 65536 {
		return nil, fmt.Errorf("serving manifest exceeds 64 KiB")
	}
	var m ServingManifest
	if err := m.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return &m, nil
}

// UnmarshalJSON rejects duplicate keys, unknown fields and null evidence.
func (m *ServingManifest) UnmarshalJSON(data []byte) error {
	if len(data) > 65536 {
		return fmt.Errorf("serving manifest exceeds 64 KiB")
	}
	if err := servingJSON(data); err != nil {
		return err
	}
	type plain ServingManifest
	var v plain
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	if err := ValidateServingManifest(ServingManifest(v)); err != nil {
		return err
	}
	*m = ServingManifest(v)
	return nil
}

func servingJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 16 {
			return fmt.Errorf("serving JSON nesting exceeds limit")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		if t == nil {
			return fmt.Errorf("null serving evidence")
		}
		x, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch x {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				t, err := d.Token()
				if err != nil {
					return err
				}
				k := t.(string)
				if seen[k] {
					return fmt.Errorf("duplicate serving JSON key %q", k)
				}
				seen[k] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("invalid serving JSON")
		}
		_, err = d.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing serving JSON")
	}
	return nil
}

// ServingWorkHash returns the canonical SHA-256 comparable-work identity. It
// refuses unknown effective settings, missing evidence or declared confounds.
// Matching hashes establish matching declarations, not physical engine parity.
func ServingWorkHash(m ServingManifest) (string, error) {
	if err := ValidateServingManifest(m); err != nil {
		return "", err
	}
	for _, s := range []string{m.ModelDigest, m.TokenizerDigest, m.TemplateDigest, m.WorkloadDigest, m.EnvironmentDigest} {
		if s == "" {
			return "", fmt.Errorf("serving work identity unavailable")
		}
	}
	if len(m.Confounds) > 0 {
		return "", fmt.Errorf("serving comparison has declared confounds")
	}
	settings := map[string]json.RawMessage{}
	for _, name := range servingSettingNames {
		s, ok := m.Settings[name]
		if !ok || len(s.Effective) == 0 || s.EvidenceDigest == "" {
			return "", fmt.Errorf("effective setting %s unavailable", name)
		}
		var value any
		d := json.NewDecoder(bytes.NewReader(s.Effective))
		d.UseNumber()
		if err := d.Decode(&value); err != nil {
			return "", err
		}
		data, _ := json.Marshal(value)
		settings[name] = data
	}
	data, err := json.Marshal(struct {
		Version                                           int `json:"version"`
		Model, Tokenizer, Template, Workload, Environment string
		Settings                                          map[string]json.RawMessage
	}{1, m.ModelDigest, m.TokenizerDigest, m.TemplateDigest, m.WorkloadDigest, m.EnvironmentDigest, settings})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func servingSettingValue(name string, value json.RawMessage) error {
	switch name {
	case "temperature", "top_p", "top_k", "seed", "max_output_tokens":
		n, err := servingNumber(value)
		if err != nil {
			return fmt.Errorf("setting %s: %w", name, err)
		}
		if name == "seed" {
			limit := big.NewRat(9007199254740991, 1)
			magnitude := new(big.Rat).Abs(n)
			if !n.IsInt() || magnitude.Cmp(limit) > 0 {
				return fmt.Errorf("invalid serving seed")
			}
			return nil
		}
		if n.Sign() < 0 || name == "top_p" && n.Cmp(big.NewRat(1, 1)) > 0 || (name == "top_k" || name == "max_output_tokens") && (!n.IsInt() || n.Cmp(big.NewRat(1000000000, 1)) > 0) {
			return fmt.Errorf("invalid serving setting %s", name)
		}

	case "dtype", "device":
		var text string
		if err := json.Unmarshal(value, &text); err != nil || text == "" || len(text) > 256 {
			return fmt.Errorf("setting %s must be a bounded nonempty string", name)
		}
	}
	return nil
}

// servingNumber validates decimal JSON semantics before any float conversion.
// Bound exponents before constructing a rational so small inputs cannot allocate
// arbitrary powers of ten. Zero needs no power and remains valid at any exponent.
func servingNumber(value json.RawMessage) (*big.Rat, error) {
	if len(value) > 4096 {
		return nil, fmt.Errorf("numeric setting exceeds limit")
	}
	d := json.NewDecoder(bytes.NewReader(value))
	d.UseNumber()
	var decoded any
	if err := d.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("numeric setting must be a JSON number")
	}
	number, ok := decoded.(json.Number)
	if !ok {
		return nil, fmt.Errorf("numeric setting must be a JSON number")
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing numeric setting data")
	}
	text := number.String()
	mantissa, exponent := text, ""
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		mantissa, exponent = text[:i], text[i+1:]
	}
	if strings.Trim(mantissa, "-+.0") == "" {
		return new(big.Rat), nil
	}
	if exponent != "" {
		e, err := strconv.ParseInt(exponent, 10, 32)
		if err != nil || e < -4096 || e > 4096 {
			return nil, fmt.Errorf("numeric setting exponent exceeds limit")
		}
	}
	n, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, fmt.Errorf("invalid numeric setting")
	}
	// Preserve the original finite machine-number bound without using its rounded
	// representation for sign, integer or setting-specific range checks.
	max := new(big.Rat).SetFloat64(math.MaxFloat64)
	if new(big.Rat).Abs(n).Cmp(max) > 0 {
		return nil, fmt.Errorf("numeric setting exceeds finite range")
	}
	return n, nil
}
