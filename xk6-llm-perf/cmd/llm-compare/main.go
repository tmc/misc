// Command llm-compare renders saved runs or serves a loopback comparison dashboard.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	llm "github.com/tmc/misc/xk6-llm-perf"
	"github.com/tmc/misc/xk6-llm-perf/output"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "llm-compare:", err)
		os.Exit(1)
	}
}
func load(paths []string) (output.Comparison, error) {
	if len(paths) < 2 || len(paths) > 64 {
		return output.Comparison{}, fmt.Errorf("comparison requires 2..64 input files")
	}
	var runs []llm.RunSummary
	for _, path := range paths {
		f, err := openSnapshot(path)
		if err != nil {
			return output.Comparison{}, err
		}
		r, err := llm.ReadRun(f)
		f.Close()
		if err != nil {
			return output.Comparison{}, fmt.Errorf("read %s: %w", path, err)
		}
		runs = append(runs, r)
	}
	return output.Compare(runs)
}
func run(args []string, w io.Writer) error {
	flags := flag.NewFlagSet("llm-compare", flag.ContinueOnError)
	format := flags.String("format", "html", "json, html, svg or png")
	metric := flags.String("metric", "ttft", "metric mean to plot")
	serve := flags.String("serve", "", "loopback address to serve refreshing saved-run comparison")
	renderer := flags.String("png-renderer", "", "explicit rsvg-convert executable for PNG")
	pin := flags.String("png-renderer-sha256", "", "required PNG renderer binary SHA256")
	if err := flags.Parse(args); err != nil {
		return err
	}
	paths := flags.Args()
	c, err := load(paths)
	if err != nil {
		return err
	}
	if *serve != "" {
		host, _, err := net.SplitHostPort(*serve)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("dashboard requires literal loopback address")
		}
		listener, err := net.Listen("tcp", *serve)
		if err != nil {
			return err
		}
		defer listener.Close()
		mux := http.NewServeMux()
		mux.HandleFunc("/comparison.json", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			current, err := load(paths)
			if err != nil {
				http.Error(w, "current run snapshot unavailable", http.StatusServiceUnavailable)
				return
			}
			json.NewEncoder(w).Encode(current)
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, dashboard)
		})
		fmt.Fprintln(os.Stderr, "comparison dashboard: http://"+listener.Addr().String())
		return (&http.Server{Handler: guard(mux, listener.Addr().String()), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}).Serve(listener)
	}
	switch *format {
	case "json":
		return json.NewEncoder(w).Encode(c)
	case "svg":
		return output.ComparisonSVG(w, c, *metric)
	case "html":
		var b bytes.Buffer
		if err := output.ComparisonSVG(&b, c, *metric); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "<!doctype html><meta charset=utf-8><title>Run comparison</title><h1>Run comparison</h1>"); err != nil {
			return err
		}
		if _, err := w.Write(b.Bytes()); err != nil {
			return err
		}
		for _, d := range c.Runs {
			if _, err := fmt.Fprintf(w, "<h2>%s</h2><p>%d attempts; %d successes; %d failures; %d dropped; %d boundary exclusions; %d inflight.</p><p>%s; %s; %s</p><table><tr><th>Metric</th><th>Unit</th><th>n</th><th>Mean</th></tr>", html.EscapeString(d.RunID), d.Attempts, d.Successes, d.Failures, d.DroppedRecords, d.ExcludedBoundaryRequests, d.InflightAtEnd, html.EscapeString(d.MeasurementPolicy), html.EscapeString(d.WindowPolicy), html.EscapeString(d.WarmupPolicy)); err != nil {
				return err
			}
			p, _ := json.Marshal(c.Policies[d.RunID])
			if _, err := fmt.Fprintf(w, "<p>Observed policy: %s; complete=%t. Zero SLO limits are disabled.</p>", html.EscapeString(string(p)), c.PolicyComplete[d.RunID]); err != nil {
				return err
			}
			for _, m := range d.Metrics {
				value := "N/A"
				if m.Mean != nil {
					value = fmt.Sprintf("%.6g", *m.Mean)
				}
				if _, err := fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>", html.EscapeString(m.Name), html.EscapeString(m.Unit), m.Count, value); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, "</table>"); err != nil {
				return err
			}
		}
		return nil
	case "png":
		if *renderer == "" || len(*pin) != 64 {
			return fmt.Errorf("PNG requires explicit renderer and SHA256")
		}
		data, err := os.ReadFile(*renderer)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != *pin {
			return fmt.Errorf("PNG renderer hash mismatch")
		}
		var svg bytes.Buffer
		if err := output.ComparisonSVG(&svg, c, *metric); err != nil {
			return err
		}
		png, err := renderPNG(*renderer, svg.Bytes())
		if err != nil {
			return err
		}
		_, err = w.Write(png)
		return err
	default:
		return fmt.Errorf("unknown comparison format")
	}
}

const dashboard = `<!doctype html><meta charset="utf-8"><title>Live saved-run comparison</title><style>body{font:16px system-ui;margin:2em}td,th{padding:.5em;text-align:left}th{background:#eee}.bar{height:12px;background:#3569bd}</style><h1>Live saved-run comparison</h1><p>NOT statistically gated. Individual populations; no pooling or confidence claim. Observed numeric SLO thresholds and policy completeness are shown per run. Zero disables that limit; missing remains unavailable. Updates every two seconds from validated saved-run snapshots.</p><p id="status">Loading snapshots</p><label>Metric <select id="metric"></select></label><table id="runs"></table><script>
const select=document.querySelector('#metric'),table=document.querySelector('#runs'),status=document.querySelector('#status');let current;function cell(tr,value){let td=document.createElement('td');td.textContent=value;tr.append(td)}function draw(){if(!current)return;table.replaceChildren();let names=['Run','Unit','Mean','n','Attempts / successes / failures','Dropped / boundary / inflight','Policies'];let head=document.createElement('tr');for(let n of names)cell(head,n);table.append(head);for(let r of current.runs){let m=r.metrics.find(m=>m.name===select.value),tr=document.createElement('tr');cell(tr,r.run_id);cell(tr,m?.unit??'N/A');cell(tr,m?.mean==null?'N/A':m.mean.toPrecision(6));cell(tr,m?.count??0);cell(tr,[r.attempts,r.successes,r.failures].join(' / '));cell(tr,[r.dropped_records,r.excluded_boundary_requests,r.inflight_at_end].join(' / '));cell(tr,[r.measurement_policy,r.window_policy,r.warmup_policy,r.percentile_method,JSON.stringify(current.policies[r.run_id]??null),'complete='+current.policy_complete[r.run_id]].join('; '));table.append(tr)}}async function update(){try{let response=await fetch('/comparison.json',{cache:'no-store'});if(!response.ok)throw Error('Snapshot unavailable; current values withheld');current=await response.json();let old=select.value,names=[...new Set(current.runs.flatMap(r=>r.metrics.map(m=>m.name)))];select.replaceChildren(...names.map(name=>{let option=document.createElement('option');option.value=name;option.textContent=name;return option}));if(names.includes(old))select.value=old;status.textContent='Validated snapshots at '+new Date().toISOString();draw()}catch(error){current=null;table.replaceChildren();status.textContent=error.message}}select.addEventListener('change',draw);update();setInterval(update,2000);
</script>`

func guard(next http.Handler, authority string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != authority || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+authority) {
			http.Error(w, "local origin required", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}
