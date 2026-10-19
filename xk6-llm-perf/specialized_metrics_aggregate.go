package llmperf

import "fmt"

// SpecializedAggregateOptions selects aggregate availability rules.
type SpecializedAggregateOptions struct {
	// AIPerf sums each independently present operand, matching the pinned
	// upstream scalar sums. Partial populations and count provenance remain
	// explicit; a computed ratio does not establish complete-population coverage.
	AIPerf bool
}

// SpecializedTokenAggregate retains exact observed integer counts and coverage.
// Value is nil for an empty population or, by default, any missing operand.
// AIPerf mode exposes an observed sum when at least one value is present.
type SpecializedTokenAggregate struct {
	Unit          string         `json:"unit"`
	Value         *int64         `json:"value"`
	ObservedTotal int64          `json:"observed_total"`
	Samples       int            `json:"samples"`
	Missing       int            `json:"missing"`
	Sources       map[string]int `json:"sources"`
	Complete      bool           `json:"complete"`
	Unavailable   string         `json:"unavailable,omitempty"`
}

// SpecializedAnalysis describes aggregates over successful retained measurement
// requests. It does not establish upstream endpoint-capability or credit-phase
// equivalence, media counts, physical cache hits or server prefill execution.
type SpecializedAnalysis struct {
	RunID                     string                    `json:"run_id"`
	Policy                    string                    `json:"policy"`
	Population                string                    `json:"population"`
	Attempts                  int                       `json:"attempts"`
	Selected                  int                       `json:"selected"`
	ExcludedFailures          int                       `json:"excluded_failures"`
	OverallThinkingEfficiency SpecializedMetric         `json:"overall_thinking_efficiency"`
	ThinkingQualification     string                    `json:"thinking_qualification"`
	ReasoningSubsetViolations int                       `json:"reasoning_subset_violations"`
	ReasoningTokens           SpecializedTokenAggregate `json:"reasoning_tokens"`
	OutputTokens              SpecializedTokenAggregate `json:"output_tokens"`
	CacheReadTokens           SpecializedTokenAggregate `json:"cache_read_tokens"`
	CacheWriteTokens          SpecializedTokenAggregate `json:"cache_write_tokens"`
}

// AnalyzeSpecialized validates a complete retained run before summing available
// token fields. The default requires complete operands and reported output for
// thinking efficiency. AIPerf mode derives sum(reasoning)/sum(output) over their
// independently present populations, not the mean or paired-subset ratio.
//
// Both policies preserve missing values and count provenance. Output zero makes
// the ratio unavailable; present zero reasoning and cache counts remain values.
// Failures stay in accounting but supply no successful specialized operands.
func AnalyzeSpecialized(run RunSummary, opts SpecializedAggregateOptions) (SpecializedAnalysis, error) {
	if _, err := AnalyzeTimeline(run); err != nil {
		return SpecializedAnalysis{}, err
	}
	fresh := func() SpecializedTokenAggregate {
		return SpecializedTokenAggregate{Unit: "tokens", Sources: map[string]int{}}
	}
	result := SpecializedAnalysis{RunID: run.RunID, Attempts: run.Attempts, ExcludedFailures: run.Failures,
		Policy:          "complete operands required; thinking additionally requires reported output and consistent reasoning subset",
		Population:      "successful recorded attempted local measurement requests; complete retained run; no upstream credit-phase or endpoint-capability equivalence",
		ReasoningTokens: fresh(), OutputTokens: fresh(), CacheReadTokens: fresh(), CacheWriteTokens: fresh(),
		OverallThinkingEfficiency: SpecializedMetric{Unit: "ratio", Unavailable: "no complete eligible operand population"}}
	if opts.AIPerf {
		result.Policy = "AIPerf independent observed scalar sums; partial operands and all validated output count sources retained; values do not imply complete coverage"
	}
	reportedOutputOnly := true
	for _, record := range run.Records {
		if !specializedPopulation(record) {
			continue
		}
		result.Selected++
		if err := addSpecializedTokens(&result.ReasoningTokens, record.ReasoningTokens, "reported"); err != nil {
			return SpecializedAnalysis{}, fmt.Errorf("reasoning tokens: %w", err)
		}
		if err := addSpecializedTokens(&result.OutputTokens, record.OutputTokens.Value, record.OutputTokens.Source); err != nil {
			return SpecializedAnalysis{}, fmt.Errorf("output tokens: %w", err)
		}
		if record.OutputTokens.Value != nil && record.OutputTokens.Source != "reported" {
			reportedOutputOnly = false
		}
		if record.ReasoningTokens != nil && record.OutputTokens.Value != nil && *record.ReasoningTokens > *record.OutputTokens.Value {
			result.ReasoningSubsetViolations++
		}
		if err := addSpecializedTokens(&result.CacheReadTokens, record.CachedTokens, "reported"); err != nil {
			return SpecializedAnalysis{}, fmt.Errorf("cache read tokens: %w", err)
		}
		if err := addSpecializedTokens(&result.CacheWriteTokens, record.CacheWriteTokens, "reported"); err != nil {
			return SpecializedAnalysis{}, fmt.Errorf("cache write tokens: %w", err)
		}
	}
	for _, aggregate := range []*SpecializedTokenAggregate{&result.ReasoningTokens, &result.OutputTokens, &result.CacheReadTokens, &result.CacheWriteTokens} {
		finishSpecializedTokens(aggregate, result.Selected, opts)
	}
	result.ThinkingQualification = "partial_or_mixed_operand_population"
	complete := result.ReasoningTokens.Complete && result.OutputTokens.Complete && reportedOutputOnly
	if complete {
		result.ThinkingQualification = "complete_reported_operands"
	}
	if result.ReasoningSubsetViolations > 0 {
		result.ThinkingQualification = "inconsistent_reasoning_subset"
	}
	if result.Selected == 0 {
		result.ThinkingQualification = "empty_population"
		return result, nil
	}
	if result.OutputTokens.ObservedTotal == 0 {
		result.ThinkingQualification = "zero_output_denominator"
		result.OverallThinkingEfficiency.Unavailable = "observed output sum is zero"
		return result, nil
	}
	eligible := complete && result.ReasoningSubsetViolations == 0
	if opts.AIPerf {
		eligible = result.ReasoningTokens.Samples > 0 && result.OutputTokens.Samples > 0
	}
	if eligible {
		source := "complete reported operands"
		if opts.AIPerf {
			source = "AIPerf independent observed sums; output sources retained separately"
		}
		result.OverallThinkingEfficiency = specializedValue("ratio", source, float64(result.ReasoningTokens.ObservedTotal)/float64(result.OutputTokens.ObservedTotal))
	}
	return result, nil
}

func addSpecializedTokens(aggregate *SpecializedTokenAggregate, value *int, source string) error {
	if value == nil {
		aggregate.Missing++
		return nil
	}
	if *value < 0 || int64(*value) > (1<<63-1)-aggregate.ObservedTotal {
		return fmt.Errorf("observed integer sum overflow or negative count")
	}
	aggregate.ObservedTotal += int64(*value)
	aggregate.Samples++
	aggregate.Sources[source]++
	return nil
}

func finishSpecializedTokens(aggregate *SpecializedTokenAggregate, selected int, opts SpecializedAggregateOptions) {
	aggregate.Complete = selected > 0 && aggregate.Missing == 0
	if aggregate.Samples > 0 && (aggregate.Complete || opts.AIPerf) {
		value := aggregate.ObservedTotal
		aggregate.Value = &value
		return
	}
	aggregate.Unavailable = "incomplete operand population"
	if selected == 0 {
		aggregate.Unavailable = "no eligible requests"
	} else if aggregate.Samples == 0 {
		aggregate.Unavailable = "no observed operand values"
	}
}
