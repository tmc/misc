package output_test

import (
	"fmt"
	llm "github.com/tmc/misc/xk6-llm-perf"
	"github.com/tmc/misc/xk6-llm-perf/output"
	"strings"
)

func ExampleReadPrices() {
	p, err := output.ReadPrices(strings.NewReader(`{"currency":"USD","source_url":"https://example.com/prices","effective_date":"2026-10-11","models":{"model":{"input":2,"output":4}}}`))
	fmt.Println(p.Currency, p.EffectiveDate, err)
	// Output: USD 2026-10-11 <nil>
}

func ExampleDeriveCost() {
	p, err := output.ReadPrices(strings.NewReader(`{"currency":"USD","source_url":"https://example.com/prices","effective_date":"2026-10-11","models":{"model":{"input":2,"output":4}}}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	run := llm.RunSummary{SchemaVersion: 1, RunID: "example", StartedAt: 1000, FinishedAt: 2000, Duration: 1, Records: []llm.Measurement{}}
	ledger, err := output.DeriveCost(run, p)
	fmt.Println(ledger.Currency, *ledger.Total, err)
	// Output: USD 0 <nil>
}
