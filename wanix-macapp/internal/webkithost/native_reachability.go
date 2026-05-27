package webkithost

import (
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/tmc/apple/dispatch"
	"github.com/tmc/apple/network"
	"github.com/tmc/apple/objectivec"
)

type reachabilitySnapshot struct {
	Status        network.NWPathStatus
	Unsatisfied   network.NWPathUnsatisfiedReason
	HasDNS        bool
	HasIPv4       bool
	HasIPv6       bool
	Expensive     bool
	Constrained   bool
	UsesWifi      bool
	UsesWired     bool
	UsesCellular  bool
	UsesLoopback  bool
	UsesOther     bool
	InterfaceText string
}

func (h *Host) applyReachabilityCtl(verb string) error {
	if verb != "refresh" {
		return nil
	}
	snap, err := captureReachability(2 * time.Second)
	if err != nil {
		_ = h.appleFS.WriteFile("reachability/status", []byte("api reachability\nstatus error\nerror "+err.Error()+"\n"))
		return err
	}
	if err := h.appleFS.WriteFile("reachability/status", []byte("api reachability\n"+formatReachabilityStatus(snap))); err != nil {
		return err
	}
	return h.appleFS.WriteFile("reachability/proxies", []byte(formatReachabilityProxies(snap)))
}

func (h *Host) applyReachabilitySession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	switch verb {
	case "check":
		snap, err := captureReachability(2 * time.Second)
		if err != nil {
			_ = h.appleFS.WriteFile("reachability/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
			return err
		}
		if err := h.appleFS.WriteFile("reachability/"+id+"/flags", []byte(formatReachabilityFlags(snap))); err != nil {
			return err
		}
		return h.appleFS.WriteFile("reachability/"+id+"/status", []byte(formatReachabilityStatus(snap)))
	case "destroy":
		return h.appleFS.WriteFile("reachability/"+id+"/status", []byte("status destroyed\n"))
	default:
		return nil
	}
}

func captureReachability(timeout time.Duration) (snap reachabilitySnapshot, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("network path monitor: %v", r)
		}
	}()
	monitor := network.NWPathMonitorCreate()
	queue := dispatch.QueueCreate("github.com.tmc.wanix.reachability")
	done := make(chan reachabilitySnapshot, 1)
	network.NWPathMonitorSetQueue(monitor, queue)
	network.NWPathMonitorSetUpdateHandler(monitor, func(path network.NWPath) {
		select {
		case done <- reachabilityFromPath(path):
		default:
		}
	})
	network.NWPathMonitorStart(monitor)
	defer network.NWPathMonitorCancel(monitor)

	select {
	case snap := <-done:
		return snap, nil
	case <-time.After(timeout):
		return reachabilitySnapshot{}, fmt.Errorf("network path monitor timed out")
	}
}

func reachabilityFromPath(path network.NWPath) reachabilitySnapshot {
	snap := reachabilitySnapshot{
		Status:       network.NWPathGetStatus(path),
		Unsatisfied:  network.NWPathGetUnsatisfiedReason(path),
		HasDNS:       network.NWPathHasDns(path),
		HasIPv4:      network.NWPathHasIpv4(path),
		HasIPv6:      network.NWPathHasIpv6(path),
		Expensive:    network.NWPathIsExpensive(path),
		Constrained:  network.NWPathIsConstrained(path),
		UsesWifi:     network.NWPathUsesInterfaceType(path, network.NWInterfaceTypeWifi),
		UsesWired:    network.NWPathUsesInterfaceType(path, network.NWInterfaceTypeWired),
		UsesCellular: network.NWPathUsesInterfaceType(path, network.NWInterfaceTypeCellular),
		UsesLoopback: network.NWPathUsesInterfaceType(path, network.NWInterfaceTypeLoopback),
		UsesOther:    network.NWPathUsesInterfaceType(path, network.NWInterfaceTypeOther),
	}
	var interfaces []string
	network.NWPathEnumerateInterfaces(path, func(iface objectivec.Object) bool {
		interfaces = append(interfaces, fmt.Sprintf("%s %s", cString(network.NWInterfaceGetName(iface)), reachabilityInterfaceType(network.NWInterfaceGetType(iface))))
		return true
	})
	snap.InterfaceText = strings.Join(interfaces, ",")
	return snap
}

func formatReachabilityStatus(s reachabilitySnapshot) string {
	return fmt.Sprintf("status %s\nreachable %t\n", reachabilityPathStatus(s.Status), s.Status == network.NWPathStatusSatisfied)
}

func formatReachabilityFlags(s reachabilitySnapshot) string {
	return fmt.Sprintf("reachable %t\nstatus %s\nreason %s\ndns %t\nipv4 %t\nipv6 %t\nexpensive %t\nconstrained %t\nwifi %t\nwired %t\ncellular %t\nloopback %t\nother %t\ninterfaces %s\n",
		s.Status == network.NWPathStatusSatisfied,
		reachabilityPathStatus(s.Status),
		reachabilityUnsatisfiedReason(s.Unsatisfied),
		s.HasDNS,
		s.HasIPv4,
		s.HasIPv6,
		s.Expensive,
		s.Constrained,
		s.UsesWifi,
		s.UsesWired,
		s.UsesCellular,
		s.UsesLoopback,
		s.UsesOther,
		s.InterfaceText,
	)
}

func formatReachabilityProxies(s reachabilitySnapshot) string {
	return fmt.Sprintf("http-enable false\nexpensive %t\nconstrained %t\n", s.Expensive, s.Constrained)
}

func reachabilityPathStatus(status network.NWPathStatus) string {
	switch status {
	case network.NWPathStatusSatisfied:
		return "satisfied"
	case network.NWPathStatusSatisfiable:
		return "satisfiable"
	case network.NWPathStatusUnsatisfied:
		return "unsatisfied"
	case network.NWPathStatusInvalid:
		return "invalid"
	default:
		return fmt.Sprintf("unknown-%d", status)
	}
}

func reachabilityUnsatisfiedReason(reason network.NWPathUnsatisfiedReason) string {
	switch reason {
	case network.NWPathUnsatisfiedReasonNotAvailable:
		return "not-available"
	case network.NWPathUnsatisfiedReasonCellularDenied:
		return "cellular-denied"
	case network.NWPathUnsatisfiedReasonWifiDenied:
		return "wifi-denied"
	case network.NWPathUnsatisfiedReasonLocalNetworkDenied:
		return "local-network-denied"
	case network.NWPathUnsatisfiedReasonVpnInactive:
		return "vpn-inactive"
	default:
		return fmt.Sprintf("unknown-%d", reason)
	}
}

func reachabilityInterfaceType(typ network.NWInterfaceType) string {
	switch typ {
	case network.NWInterfaceTypeWifi:
		return "wifi"
	case network.NWInterfaceTypeWired:
		return "wired"
	case network.NWInterfaceTypeCellular:
		return "cellular"
	case network.NWInterfaceTypeLoopback:
		return "loopback"
	case network.NWInterfaceTypeOther:
		return "other"
	default:
		return fmt.Sprintf("unknown-%d", typ)
	}
}

func cString(p *byte) string {
	if p == nil {
		return ""
	}
	var n int
	for q := uintptr(unsafe.Pointer(p)); *(*byte)(unsafe.Pointer(q)) != 0; q++ {
		n++
	}
	return unsafe.String(p, n)
}
