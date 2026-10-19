// Package traceworkload converts bounded source traces into workload artifacts.
// ReadBurstGPT produces flat targets. ReadMooncake, ReadBailian and ReadTraceLab
// preserve grouped sessions, source counts, timing policy and cache identities.
// Counts are declarations, not independently tokenized prompts or observed
// generated output. The readers do not synthesize text or an empirical prefix
// graph. The llm-import-trace command can export explicitly serialized replay
// sessions only when authored request content is present.
//
// Session readers cap the entire import at 100000 rows, 10000 sessions,
// 1000 turns per session, 262144 stored cache/virtual-block entries and a
// conservative 64 MiB normalized/export output budget. Byte/row/token limits
// alone do not admit an amplified derived block allocation.
package traceworkload
