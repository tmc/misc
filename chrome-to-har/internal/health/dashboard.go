package health

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// Dashboard provides a web interface for health monitoring
type Dashboard struct {
	healthManager *HealthManager
	server        *http.Server
	config        *DashboardConfig
}

// DashboardConfig configures the dashboard
type DashboardConfig struct {
	Port        int    `json:"port"`
	Host        string `json:"host"`
	BasePath    string `json:"base_path"`
	Title       string `json:"title"`
	RefreshRate int    `json:"refresh_rate"` // seconds
	Theme       string `json:"theme"`        // light, dark, auto
	EnableAPI   bool   `json:"enable_api"`
}

// DashboardData represents data for the dashboard
type DashboardData struct {
	Title           string                      `json:"title"`
	BasePath        string                      `json:"base_path"`
	Timestamp       time.Time                   `json:"timestamp"`
	OverallStatus   HealthResult                `json:"overall_status"`
	Checks          map[string]HealthCheckStatus `json:"checks"`
	Alerts          []*Alert                    `json:"alerts"`
	Metrics         *HealthMetrics              `json:"metrics"`
	ResourceMetrics map[string]interface{}      `json:"resource_metrics"`
	RefreshRate     int                         `json:"refresh_rate"`
	Theme           string                      `json:"theme"`
}

// NewDashboard creates a new dashboard
func NewDashboard(hm *HealthManager) *Dashboard {
	config := &DashboardConfig{
		Port:        8080,
		Host:        "localhost",
		BasePath:    "/health",
		Title:       "Health Dashboard",
		RefreshRate: 30,
		Theme:       "auto",
		EnableAPI:   true,
	}

	return &Dashboard{
		healthManager: hm,
		config:        config,
	}
}

// Start starts the dashboard server
func (d *Dashboard) Start() error {
	mux := http.NewServeMux()

	// Dashboard routes
	mux.HandleFunc(d.config.BasePath+"/", d.handleDashboard)
	mux.HandleFunc(d.config.BasePath+"/api/status", d.handleAPIStatus)
	mux.HandleFunc(d.config.BasePath+"/api/checks", d.handleAPIChecks)
	mux.HandleFunc(d.config.BasePath+"/api/alerts", d.handleAPIAlerts)
	mux.HandleFunc(d.config.BasePath+"/api/metrics", d.handleAPIMetrics)
	mux.HandleFunc(d.config.BasePath+"/api/check/", d.handleAPICheck)
	mux.HandleFunc(d.config.BasePath+"/api/alert/", d.handleAPIAlert)

	// Static assets
	mux.HandleFunc(d.config.BasePath+"/static/", d.handleStatic)

	addr := fmt.Sprintf("%s:%d", d.config.Host, d.config.Port)
	d.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Starting health dashboard on http://%s%s", addr, d.config.BasePath)
	return d.server.ListenAndServe()
}

// Stop stops the dashboard server
func (d *Dashboard) Stop() error {
	if d.server == nil {
		return nil
	}
	return d.server.Close()
}

// handleDashboard serves the main dashboard page
func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := d.getDashboardData()

	// If requesting JSON, return JSON
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
		return
	}

	// Otherwise, return HTML
	tmpl := d.getDashboardTemplate()
	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleAPIStatus handles the status API endpoint
func (d *Dashboard) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := d.healthManager.GetOverallStatus()
	json.NewEncoder(w).Encode(status)
}

// handleAPIChecks handles the checks API endpoint
func (d *Dashboard) handleAPIChecks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	checks := d.healthManager.GetAllStatuses()
	json.NewEncoder(w).Encode(checks)
}

// handleAPIAlerts handles the alerts API endpoint
func (d *Dashboard) handleAPIAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	var alerts []*Alert
	if d.healthManager.alertManager != nil {
		alerts = d.healthManager.alertManager.GetAlerts()
	}
	
	json.NewEncoder(w).Encode(alerts)
}

// handleAPIMetrics handles the metrics API endpoint
func (d *Dashboard) handleAPIMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	metrics := d.healthManager.metrics
	resourceMetrics := make(map[string]interface{})
	
	if d.healthManager.resourceMonitor != nil {
		resourceMetrics = d.healthManager.resourceMonitor.GetResourceMetrics()
	}
	
	response := map[string]interface{}{
		"health_metrics":   metrics,
		"resource_metrics": resourceMetrics,
	}
	
	json.NewEncoder(w).Encode(response)
}

// handleAPICheck handles individual check operations
func (d *Dashboard) handleAPICheck(w http.ResponseWriter, r *http.Request) {
	checkName := r.URL.Path[len(d.config.BasePath+"/api/check/"):]
	
	switch r.Method {
	case "GET":
		status, err := d.healthManager.GetStatus(checkName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
		
	case "POST":
		// Enable/disable check
		action := r.URL.Query().Get("action")
		switch action {
		case "enable":
			if err := d.healthManager.EnableCheck(checkName); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "disable":
			if err := d.healthManager.DisableCheck(checkName); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
			return
		}
		
		w.WriteHeader(http.StatusOK)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAPIAlert handles individual alert operations
func (d *Dashboard) handleAPIAlert(w http.ResponseWriter, r *http.Request) {
	alertID := r.URL.Path[len(d.config.BasePath+"/api/alert/"):]
	
	if d.healthManager.alertManager == nil {
		http.Error(w, "Alert manager not available", http.StatusServiceUnavailable)
		return
	}
	
	switch r.Method {
	case "POST":
		action := r.URL.Query().Get("action")
		switch action {
		case "acknowledge":
			acknowledgedBy := r.URL.Query().Get("acknowledged_by")
			if acknowledgedBy == "" {
				acknowledgedBy = "dashboard"
			}
			
			if err := d.healthManager.alertManager.AcknowledgeAlert(alertID, acknowledgedBy); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			
		case "resolve":
			if err := d.healthManager.alertManager.ResolveAlert(alertID); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
			return
		}
		
		w.WriteHeader(http.StatusOK)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStatic serves static assets
func (d *Dashboard) handleStatic(w http.ResponseWriter, r *http.Request) {
	// This would serve CSS, JS, and other static files
	// For now, just return 404
	http.NotFound(w, r)
}

// getDashboardData prepares data for the dashboard
func (d *Dashboard) getDashboardData() *DashboardData {
	overallStatus := d.healthManager.GetOverallStatus()
	checks := d.healthManager.GetAllStatuses()
	
	var alerts []*Alert
	if d.healthManager.alertManager != nil {
		alerts = d.healthManager.alertManager.GetAlerts()
	}
	
	var resourceMetrics map[string]interface{}
	if d.healthManager.resourceMonitor != nil {
		resourceMetrics = d.healthManager.resourceMonitor.GetResourceMetrics()
	}
	
	return &DashboardData{
		Title:           d.config.Title,
		BasePath:        d.config.BasePath,
		Timestamp:       time.Now(),
		OverallStatus:   overallStatus,
		Checks:          checks,
		Alerts:          alerts,
		Metrics:         d.healthManager.metrics,
		ResourceMetrics: resourceMetrics,
		RefreshRate:     d.config.RefreshRate,
		Theme:           d.config.Theme,
	}
}

// getDashboardTemplate returns the HTML template for the dashboard
func (d *Dashboard) getDashboardTemplate() *template.Template {
	return template.Must(template.New("dashboard").Parse(dashboardHTML))
}

// dashboardHTML contains the HTML template for the dashboard
const dashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0; 
            padding: 20px; 
            background-color: #f5f5f5;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { 
            background: white; 
            padding: 20px; 
            border-radius: 8px; 
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .status-card { 
            background: white; 
            padding: 20px; 
            border-radius: 8px; 
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .status-healthy { border-left: 4px solid #28a745; }
        .status-degraded { border-left: 4px solid #ffc107; }
        .status-unhealthy { border-left: 4px solid #dc3545; }
        .status-unknown { border-left: 4px solid #6c757d; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .metric { display: flex; justify-content: space-between; padding: 10px 0; border-bottom: 1px solid #eee; }
        .metric:last-child { border-bottom: none; }
        .alert { 
            padding: 15px; 
            margin-bottom: 10px; 
            border-radius: 4px; 
            border-left: 4px solid #dc3545;
            background: #f8d7da;
        }
        .alert-warning { border-left-color: #ffc107; background: #fff3cd; }
        .alert-info { border-left-color: #17a2b8; background: #d1ecf1; }
        .timestamp { color: #6c757d; font-size: 0.9em; }
        .refresh-indicator { 
            position: fixed; 
            top: 20px; 
            right: 20px; 
            background: #007bff; 
            color: white; 
            padding: 10px 15px; 
            border-radius: 4px;
            display: none;
        }
        .actions { margin-top: 10px; }
        .btn { 
            padding: 5px 10px; 
            margin-right: 5px; 
            border: none; 
            border-radius: 4px; 
            cursor: pointer;
            font-size: 0.9em;
        }
        .btn-primary { background: #007bff; color: white; }
        .btn-success { background: #28a745; color: white; }
        .btn-warning { background: #ffc107; color: black; }
        .btn-danger { background: #dc3545; color: white; }
        .performance-metrics { font-size: 0.85em; color: #6c757d; margin-top: 10px; }
        .tabs { margin-bottom: 20px; }
        .tab { 
            display: inline-block; 
            padding: 10px 20px; 
            margin-right: 5px; 
            background: #e9ecef; 
            border-radius: 4px 4px 0 0;
            cursor: pointer;
        }
        .tab.active { background: white; }
        .tab-content { display: none; }
        .tab-content.active { display: block; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.Title}}</h1>
            <div class="timestamp">Last updated: {{.Timestamp.Format "2006-01-02 15:04:05"}}</div>
        </div>

        <div class="status-card status-{{.OverallStatus.Status}}">
            <h2>Overall Status: {{.OverallStatus.Status | title}}</h2>
            <p>{{.OverallStatus.Message}}</p>
            <div class="performance-metrics">
                {{range $key, $value := .OverallStatus.Metrics}}
                <div class="metric">
                    <span>{{$key}}</span>
                    <span>{{$value}}</span>
                </div>
                {{end}}
            </div>
        </div>

        <div class="tabs">
            <div class="tab active" onclick="showTab('checks')">Health Checks</div>
            <div class="tab" onclick="showTab('alerts')">Alerts</div>
            <div class="tab" onclick="showTab('metrics')">Metrics</div>
        </div>

        <div id="checks" class="tab-content active">
            <div class="grid">
                {{range $name, $check := .Checks}}
                <div class="status-card status-{{$check.LastResult.Status}}">
                    <h3>{{$name}}</h3>
                    <p><strong>Status:</strong> {{$check.LastResult.Status | title}}</p>
                    <p><strong>Message:</strong> {{$check.LastResult.Message}}</p>
                    <p><strong>Category:</strong> {{$check.Category}}</p>
                    <p><strong>Last Check:</strong> {{$check.LastCheck.Format "15:04:05"}}</p>
                    
                    {{if $check.PerformanceMetrics}}
                    <div class="performance-metrics">
                        <div>Avg: {{$check.PerformanceMetrics.AvgDuration}}</div>
                        <div>Min: {{$check.PerformanceMetrics.MinDuration}}</div>
                        <div>Max: {{$check.PerformanceMetrics.MaxDuration}}</div>
                    </div>
                    {{end}}
                    
                    <div class="metric">
                        <span>Success Rate</span>
                        <span>{{printf "%.1f%%" (mul $check.SuccessRate 100)}}</span>
                    </div>
                    <div class="metric">
                        <span>Total Checks</span>
                        <span>{{$check.TotalChecks}}</span>
                    </div>
                    <div class="metric">
                        <span>Consecutive Failures</span>
                        <span>{{$check.ConsecutiveFailures}}</span>
                    </div>
                    
                    <div class="actions">
                        {{if $check.Enabled}}
                        <button class="btn btn-warning" onclick="toggleCheck('{{$name}}', 'disable')">Disable</button>
                        {{else}}
                        <button class="btn btn-success" onclick="toggleCheck('{{$name}}', 'enable')">Enable</button>
                        {{end}}
                    </div>
                </div>
                {{end}}
            </div>
        </div>

        <div id="alerts" class="tab-content">
            {{if .Alerts}}
            {{range .Alerts}}
            <div class="alert {{if eq .Severity "warning"}}alert-warning{{else if eq .Severity "info"}}alert-info{{end}}">
                <h4>{{.CheckName}} - {{.Severity | title}}</h4>
                <p>{{.Message}}</p>
                <div class="timestamp">{{.Timestamp.Format "2006-01-02 15:04:05"}}</div>
                <div class="actions">
                    {{if not .Acknowledged}}
                    <button class="btn btn-primary" onclick="acknowledgeAlert('{{.ID}}')">Acknowledge</button>
                    {{end}}
                    {{if not .Resolved}}
                    <button class="btn btn-success" onclick="resolveAlert('{{.ID}}')">Resolve</button>
                    {{end}}
                </div>
            </div>
            {{end}}
            {{else}}
            <p>No active alerts</p>
            {{end}}
        </div>

        <div id="metrics" class="tab-content">
            <div class="grid">
                <div class="status-card">
                    <h3>Health Metrics</h3>
                    <div class="metric">
                        <span>Total Checks</span>
                        <span>{{.Metrics.TotalChecks}}</span>
                    </div>
                    <div class="metric">
                        <span>Healthy Checks</span>
                        <span>{{.Metrics.HealthyChecks}}</span>
                    </div>
                    <div class="metric">
                        <span>Degraded Checks</span>
                        <span>{{.Metrics.DegradedChecks}}</span>
                    </div>
                    <div class="metric">
                        <span>Unhealthy Checks</span>
                        <span>{{.Metrics.UnhealthyChecks}}</span>
                    </div>
                    <div class="metric">
                        <span>Total Executions</span>
                        <span>{{.Metrics.TotalExecutions}}</span>
                    </div>
                    <div class="metric">
                        <span>Total Failures</span>
                        <span>{{.Metrics.TotalFailures}}</span>
                    </div>
                    <div class="metric">
                        <span>Average Latency</span>
                        <span>{{.Metrics.AverageLatency}}</span>
                    </div>
                    <div class="metric">
                        <span>Uptime</span>
                        <span>{{.Metrics.Uptime}}</span>
                    </div>
                </div>
                
                {{if .ResourceMetrics}}
                <div class="status-card">
                    <h3>Resource Metrics</h3>
                    {{range $key, $value := .ResourceMetrics}}
                    <div class="metric">
                        <span>{{$key}}</span>
                        <span>{{$value}}</span>
                    </div>
                    {{end}}
                </div>
                {{end}}
            </div>
        </div>
    </div>

    <div class="refresh-indicator" id="refreshIndicator">Refreshing...</div>

    <script>
        function showTab(tabName) {
            // Hide all tab content
            const contents = document.querySelectorAll('.tab-content');
            contents.forEach(content => content.classList.remove('active'));
            
            // Remove active class from all tabs
            const tabs = document.querySelectorAll('.tab');
            tabs.forEach(tab => tab.classList.remove('active'));
            
            // Show selected tab content
            document.getElementById(tabName).classList.add('active');
            
            // Add active class to clicked tab
            event.target.classList.add('active');
        }

        function toggleCheck(checkName, action) {
            fetch('{{.BasePath}}/api/check/' + checkName + '?action=' + action, {
                method: 'POST'
            })
            .then(response => {
                if (response.ok) {
                    setTimeout(() => location.reload(), 1000);
                } else {
                    alert('Failed to ' + action + ' check');
                }
            });
        }

        function acknowledgeAlert(alertId) {
            fetch('{{.BasePath}}/api/alert/' + alertId + '?action=acknowledge', {
                method: 'POST'
            })
            .then(response => {
                if (response.ok) {
                    setTimeout(() => location.reload(), 1000);
                } else {
                    alert('Failed to acknowledge alert');
                }
            });
        }

        function resolveAlert(alertId) {
            fetch('{{.BasePath}}/api/alert/' + alertId + '?action=resolve', {
                method: 'POST'
            })
            .then(response => {
                if (response.ok) {
                    setTimeout(() => location.reload(), 1000);
                } else {
                    alert('Failed to resolve alert');
                }
            });
        }

        // Auto-refresh
        setInterval(() => {
            const indicator = document.getElementById('refreshIndicator');
            indicator.style.display = 'block';
            
            fetch(window.location.href, {
                headers: { 'Accept': 'application/json' }
            })
            .then(response => response.json())
            .then(data => {
                // Update the page with new data
                location.reload();
            })
            .catch(error => {
                console.error('Refresh failed:', error);
            })
            .finally(() => {
                indicator.style.display = 'none';
            });
        }, {{.RefreshRate}} * 1000);
    </script>
</body>
</html>
`