// Package servermetrics reads bounded Prometheus text scrapes for benchmark
// replay. Values retain label identity, type, timestamp and availability.
// Counter rates require an explicit gap bound; histogram buckets are cumulative
// counts and are never treated as independent observations.
//
// CollectContext supports sinks that honor context cancellation. Collect accepts
// a synchronous io.Writer; an uninterruptible Write can delay shutdown.
package servermetrics
