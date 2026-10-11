package serving

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ProcessResources is a Linux /proc observation. SocketDescriptors includes
// listening, connected and non-TCP sockets; it is a conservative connection
// occupancy bound, not a count of established HTTP connections.
type ProcessResources struct {
	PID               int       `json:"pid"`
	ObservedAt        time.Time `json:"observed_at"`
	UserSystemTicks   uint64    `json:"user_system_ticks"`
	RSSBytes          uint64    `json:"rss_bytes"`
	SocketDescriptors uint64    `json:"socket_descriptors"`
	StartTicks        uint64    `json:"start_ticks"`
}

// ReadProcessResources reads a live process. Linux procfs is required. CPU time
// is reported in raw ticks: callers must obtain the host's CLK_TCK rather than
// assume a tick rate. No CPU percent is inferred from one observation.
func ReadProcessResources(pid int) (ProcessResources, error) {
	var r ProcessResources
	if pid <= 0 {
		return r, fmt.Errorf("process resources: invalid pid")
	}
	root := filepath.Join("/proc", strconv.Itoa(pid))
	data, err := os.ReadFile(filepath.Join(root, "stat"))
	if err != nil {
		return r, fmt.Errorf("process resources: stat: %w", err)
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return r, fmt.Errorf("process resources: malformed stat")
	}
	fields := strings.Fields(string(data[end+1:]))
	if len(fields) < 22 {
		return r, fmt.Errorf("process resources: short stat")
	}
	number := func(i int) (uint64, error) { return strconv.ParseUint(fields[i], 10, 64) }
	user, err := number(11)
	if err != nil {
		return r, err
	}
	system, err := number(12)
	if err != nil {
		return r, err
	}
	start, err := number(19)
	if err != nil {
		return r, err
	}
	rss, err := number(21)
	if err != nil {
		return r, err
	}
	if user > ^uint64(0)-system || rss > ^uint64(0)/uint64(os.Getpagesize()) {
		return r, fmt.Errorf("process resources: counter overflow")
	}
	entries, err := os.ReadDir(filepath.Join(root, "fd"))
	if err != nil {
		return r, fmt.Errorf("process resources: fd: %w", err)
	}
	sockets := uint64(0)
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(root, "fd", entry.Name()))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return r, fmt.Errorf("process resources: fd link: %w", err)
		}
		if strings.HasPrefix(target, "socket:[") {
			sockets++
		}
	}
	r = ProcessResources{PID: pid, ObservedAt: time.Now(), UserSystemTicks: user + system, RSSBytes: rss * uint64(os.Getpagesize()), SocketDescriptors: sockets, StartTicks: start}
	return r, nil
}

// ProcessCPUPercent computes CPU use between two observations of the same
// process identity. ticksPerSecond must be measured using getconf CLK_TCK.
func ProcessCPUPercent(first, last ProcessResources, ticksPerSecond float64) (float64, error) {
	seconds := last.ObservedAt.Sub(first.ObservedAt).Seconds()
	if first.PID != last.PID || first.StartTicks != last.StartTicks || last.UserSystemTicks < first.UserSystemTicks || seconds <= 0 || ticksPerSecond <= 0 || math.IsNaN(ticksPerSecond) || math.IsInf(ticksPerSecond, 0) {
		return 0, fmt.Errorf("process CPU: invalid identity, interval or clock frequency")
	}
	percent := float64(last.UserSystemTicks-first.UserSystemTicks) / ticksPerSecond / seconds * 100
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		return 0, fmt.Errorf("process CPU: nonfinite result")
	}
	return percent, nil
}

// SampleProcessResources reads a bounded resource series. Measurements bracket
// intervals; this does not assert that a load window fits inside the series.
func SampleProcessResources(ctx context.Context, pid int, interval time.Duration, samples int) ([]ProcessResources, error) {
	if interval < time.Millisecond || interval > time.Second || samples < 2 || samples > 10000 || interval*time.Duration(samples) > time.Minute {
		return nil, fmt.Errorf("process resources: invalid sampling bounds")
	}
	observations := make([]ProcessResources, 0, samples)
	for i := 0; i < samples; i++ {
		if err := ctx.Err(); err != nil {
			return observations, err
		}
		r, err := ReadProcessResources(pid)
		if err != nil {
			return observations, err
		}
		if len(observations) > 0 && r.StartTicks != observations[0].StartTicks {
			return observations, fmt.Errorf("process resources: process identity changed")
		}
		observations = append(observations, r)
		if i+1 < samples {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return observations, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return observations, nil
}
