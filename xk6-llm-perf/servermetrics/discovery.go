package servermetrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// DiscoverTargets reads a configured JSON array of complete HTTP(S) scrape URLs.
// A caller may reread a changing file on each collection tick. Discovery does
// not scan networks or infer targets from benchmark requests.
func DiscoverTargets(r io.Reader) ([]string, error) {
	var targets []string
	d := json.NewDecoder(io.LimitReader(r, 1024*1024+1))
	if err := d.Decode(&targets); err != nil {
		return nil, fmt.Errorf("decode targets: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("trailing target data")
	}
	if len(targets) > 16 {
		return nil, fmt.Errorf("more than 16 targets")
	}
	seen := map[string]bool{}
	for _, t := range targets {
		if !validTarget(t) || seen[t] {
			return nil, fmt.Errorf("invalid or duplicate target")
		}
		seen[t] = true
	}
	sort.Strings(targets)
	return targets, nil
}

func validTarget(t string) bool {
	u, e := url.Parse(t)
	return len(t) <= 2048 && e == nil && u.Host != "" && oneOf(u.Scheme, "http", "https") && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}

// DiscoverPods reads a bounded Kubernetes PodList obtained by an authorized
// caller. Only Running pods in the explicit namespace matching all labels and
// prometheus.io/scrape=true become targets. No credentials or API calls are
// implicit. Ports must be explicitly annotated; paths default to /metrics.
func DiscoverPods(r io.Reader, namespace string, labels map[string]string) ([]string, error) {
	if namespace == "" || namespace == "*" {
		return nil, fmt.Errorf("explicit namespace required")
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Namespace   string
				Labels      map[string]string
				Annotations map[string]string
			}
			Status struct {
				Phase string
				PodIP string
			}
		}
	}
	data, err := io.ReadAll(io.LimitReader(r, 8*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 8*1024*1024 {
		return nil, fmt.Errorf("pod list exceeds 8 MiB")
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	if len(list.Items) > 10000 {
		return nil, fmt.Errorf("pod list exceeds item budget")
	}
	set := map[string]bool{}
	for _, p := range list.Items {
		if p.Metadata.Namespace != namespace || p.Status.Phase != "Running" || net.ParseIP(p.Status.PodIP) == nil || p.Metadata.Annotations["prometheus.io/scrape"] != "true" {
			continue
		}
		match := true
		for k, v := range labels {
			if p.Metadata.Labels[k] != v {
				match = false
			}
		}
		if !match {
			continue
		}
		a := p.Metadata.Annotations
		port, err := strconv.Atoi(a["prometheus.io/port"])
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid annotated scrape port")
		}
		scheme := a["prometheus.io/scheme"]
		if scheme == "" {
			scheme = "http"
		}
		path := a["prometheus.io/path"]
		if path == "" {
			path = "/metrics"
		}
		if !strings.HasPrefix(path, "/") {
			return nil, fmt.Errorf("invalid annotated scrape path")
		}
		u := url.URL{Scheme: scheme, Host: net.JoinHostPort(p.Status.PodIP, strconv.Itoa(port)), Path: path}
		target := u.String()
		if !validTarget(target) {
			return nil, fmt.Errorf("invalid annotated target")
		}
		set[target] = true
		if len(set) > 16 {
			return nil, fmt.Errorf("discovery exceeds 16 targets")
		}
	}
	targets := make([]string, 0, len(set))
	for t := range set {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	return targets, nil
}
