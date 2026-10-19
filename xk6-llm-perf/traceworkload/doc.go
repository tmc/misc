// Package traceworkload converts bounded BurstGPT CSV records into workload
// targets. It preserves source bytes by hash, record order and timestamp origin.
// Input counts are targets for independently tokenized candidate prompts; output
// budgets are generation limits, not observed generated counts. No prompt text
// or prefix-sharing graph is synthesized from token counts.
package traceworkload
