package webkithost

import (
	"strings"
	"testing"

	"github.com/tmc/apple/network"
)

func TestReachabilityText(t *testing.T) {
	snap := reachabilitySnapshot{
		Status:        network.NWPathStatusSatisfied,
		Unsatisfied:   network.NWPathUnsatisfiedReasonNotAvailable,
		HasDNS:        true,
		HasIPv4:       true,
		UsesWifi:      true,
		InterfaceText: "en0 wifi",
	}
	if got := formatReachabilityStatus(snap); got != "status satisfied\nreachable true\n" {
		t.Fatalf("status = %q", got)
	}
	flags := formatReachabilityFlags(snap)
	for _, want := range []string{"reachable true\n", "dns true\n", "wifi true\n", "interfaces en0 wifi\n"} {
		if !strings.Contains(flags, want) {
			t.Fatalf("flags = %q, missing %q", flags, want)
		}
	}
}

func TestReachabilityNames(t *testing.T) {
	if got := reachabilityPathStatus(network.NWPathStatusUnsatisfied); got != "unsatisfied" {
		t.Fatalf("path status = %q", got)
	}
	if got := reachabilityUnsatisfiedReason(network.NWPathUnsatisfiedReasonWifiDenied); got != "wifi-denied" {
		t.Fatalf("reason = %q", got)
	}
	if got := reachabilityInterfaceType(network.NWInterfaceTypeWired); got != "wired" {
		t.Fatalf("interface type = %q", got)
	}
}
