package health

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Alert represents a health alert
type Alert struct {
	ID           string         `json:"id"`
	CheckName    string         `json:"check_name"`
	Severity     HealthSeverity `json:"severity"`
	Message      string         `json:"message"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
	Category     HealthCategory `json:"category"`
	Predictive   bool           `json:"predictive"`
	Confidence   float64        `json:"confidence,omitempty"`
	Trend        string         `json:"trend,omitempty"`
	Resolved     bool           `json:"resolved"`
	ResolvedAt   time.Time      `json:"resolved_at,omitempty"`
	Acknowledged bool           `json:"acknowledged"`
	AcknowledgedAt time.Time    `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string       `json:"acknowledged_by,omitempty"`
	Escalated    bool           `json:"escalated"`
	EscalatedAt  time.Time      `json:"escalated_at,omitempty"`
	Actions      []string       `json:"actions,omitempty"`
}

// AlertManager manages health alerts
type AlertManager struct {
	healthManager *HealthManager
	alerts        map[string]*Alert
	suppressions  map[string]time.Time
	handlers      []AlertHandler
	mu            sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// AlertHandler defines the interface for alert handlers
type AlertHandler interface {
	HandleAlert(alert *Alert) error
	Name() string
}

// NewAlertManager creates a new alert manager
func NewAlertManager(hm *HealthManager) *AlertManager {
	am := &AlertManager{
		healthManager: hm,
		alerts:        make(map[string]*Alert),
		suppressions:  make(map[string]time.Time),
		handlers:      make([]AlertHandler, 0),
		stopChan:      make(chan struct{}),
	}

	// Register default handlers
	am.RegisterHandler(&LogAlertHandler{})

	return am
}

// RegisterHandler registers an alert handler
func (am *AlertManager) RegisterHandler(handler AlertHandler) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.handlers = append(am.handlers, handler)
}

// Start starts the alert manager
func (am *AlertManager) Start() {
	defer am.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			am.processAlerts()
		case <-am.stopChan:
			return
		}
	}
}

// Stop stops the alert manager
func (am *AlertManager) Stop() {
	close(am.stopChan)
	am.wg.Wait()
}

// ProcessResult processes a health check result for alerting
func (am *AlertManager) ProcessResult(checkName string, result HealthResult) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if this check is suppressed
	if suppressedUntil, exists := am.suppressions[checkName]; exists {
		if time.Now().Before(suppressedUntil) {
			return // Still suppressed
		}
		delete(am.suppressions, checkName)
	}

	// Get the check configuration
	check := am.healthManager.checks[checkName]
	if check == nil || !check.AlertConfig.Enabled {
		return
	}

	// Check if we should create an alert
	if result.Status == StatusUnhealthy || result.Status == StatusDegraded {
		// Check if we've reached the failure threshold
		if check.consecutiveFailures >= int64(check.AlertConfig.FailureThreshold) {
			alertID := fmt.Sprintf("%s-%d", checkName, time.Now().Unix())
			
			alert := &Alert{
				ID:        alertID,
				CheckName: checkName,
				Severity:  result.Severity,
				Message:   result.Message,
				Details:   result.Details,
				Timestamp: time.Now(),
				Category:  result.Category,
			}

			am.alerts[alertID] = alert
			am.sendAlert(alert)

			// Apply suppression
			if check.AlertConfig.SuppressFor > 0 {
				am.suppressions[checkName] = time.Now().Add(check.AlertConfig.SuppressFor)
			}
		}
	} else if result.Status == StatusHealthy {
		// Check for recovery notifications
		if check.AlertConfig.RecoveryNotification && check.consecutiveFailures > 0 {
			alertID := fmt.Sprintf("%s-recovery-%d", checkName, time.Now().Unix())
			
			alert := &Alert{
				ID:        alertID,
				CheckName: checkName,
				Severity:  SeverityInfo,
				Message:   fmt.Sprintf("Health check %s has recovered", checkName),
				Details:   result.Details,
				Timestamp: time.Now(),
				Category:  result.Category,
			}

			am.sendAlert(alert)
		}
	}
}

// SendAlert sends an alert immediately
func (am *AlertManager) SendAlert(alert *Alert) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert.ID == "" {
		alert.ID = fmt.Sprintf("%s-%d", alert.CheckName, time.Now().Unix())
	}

	am.alerts[alert.ID] = alert
	am.sendAlert(alert)
}

// sendAlert sends an alert to all registered handlers
func (am *AlertManager) sendAlert(alert *Alert) {
	for _, handler := range am.handlers {
		go func(h AlertHandler) {
			if err := h.HandleAlert(alert); err != nil {
				log.Printf("Error handling alert with %s: %v", h.Name(), err)
			}
		}(handler)
	}
}

// processAlerts processes existing alerts for escalation
func (am *AlertManager) processAlerts() {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, alert := range am.alerts {
		if alert.Resolved || alert.Escalated {
			continue
		}

		// Check if alert should be escalated
		check := am.healthManager.checks[alert.CheckName]
		if check == nil {
			continue
		}

		for _, rule := range check.AlertConfig.EscalationRules {
			if check.consecutiveFailures >= int64(rule.AfterFailures) {
				// Escalate alert
				alert.Escalated = true
				alert.EscalatedAt = time.Now()
				alert.Severity = rule.Severity
				alert.Actions = rule.Actions

				// Send escalated alert
				escalatedAlert := &Alert{
					ID:        fmt.Sprintf("%s-escalated-%d", alert.CheckName, time.Now().Unix()),
					CheckName: alert.CheckName,
					Severity:  rule.Severity,
					Message:   fmt.Sprintf("ESCALATED: %s", alert.Message),
					Details:   alert.Details,
					Timestamp: time.Now(),
					Category:  alert.Category,
					Escalated: true,
				}

				am.sendAlert(escalatedAlert)
				break
			}
		}
	}
}

// GetAlerts returns all active alerts
func (am *AlertManager) GetAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*Alert, 0, len(am.alerts))
	for _, alert := range am.alerts {
		if !alert.Resolved {
			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// AcknowledgeAlert acknowledges an alert
func (am *AlertManager) AcknowledgeAlert(alertID, acknowledgedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	alert.Acknowledged = true
	alert.AcknowledgedAt = time.Now()
	alert.AcknowledgedBy = acknowledgedBy

	return nil
}

// ResolveAlert resolves an alert
func (am *AlertManager) ResolveAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	alert.Resolved = true
	alert.ResolvedAt = time.Now()

	return nil
}

// SuppressCheck suppresses alerts for a check for a specified duration
func (am *AlertManager) SuppressCheck(checkName string, duration time.Duration) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.suppressions[checkName] = time.Now().Add(duration)
}

// LogAlertHandler logs alerts to the standard logger
type LogAlertHandler struct{}

func (h *LogAlertHandler) HandleAlert(alert *Alert) error {
	log.Printf("[ALERT] %s: %s (%s) - %s", 
		alert.Severity, alert.CheckName, alert.Category, alert.Message)
	return nil
}

func (h *LogAlertHandler) Name() string {
	return "log"
}

// WebhookAlertHandler sends alerts to a webhook
type WebhookAlertHandler struct {
	URL     string
	Headers map[string]string
}

func (h *WebhookAlertHandler) HandleAlert(alert *Alert) error {
	// Implementation would send HTTP POST to webhook URL
	// This is a placeholder for the actual implementation
	log.Printf("[WEBHOOK] Sending alert to %s: %s", h.URL, alert.Message)
	return nil
}

func (h *WebhookAlertHandler) Name() string {
	return "webhook"
}

// EmailAlertHandler sends alerts via email
type EmailAlertHandler struct {
	SMTPServer string
	From       string
	To         []string
	Username   string
	Password   string
}

func (h *EmailAlertHandler) HandleAlert(alert *Alert) error {
	// Implementation would send email
	// This is a placeholder for the actual implementation
	log.Printf("[EMAIL] Sending alert to %v: %s", h.To, alert.Message)
	return nil
}

func (h *EmailAlertHandler) Name() string {
	return "email"
}

// SlackAlertHandler sends alerts to Slack
type SlackAlertHandler struct {
	WebhookURL string
	Channel    string
	Username   string
}

func (h *SlackAlertHandler) HandleAlert(alert *Alert) error {
	// Implementation would send to Slack webhook
	// This is a placeholder for the actual implementation
	log.Printf("[SLACK] Sending alert to %s: %s", h.Channel, alert.Message)
	return nil
}

func (h *SlackAlertHandler) Name() string {
	return "slack"
}

// Notifier handles notifications
type Notifier struct {
	channels map[string]chan *Alert
	mu       sync.RWMutex
}

// NewNotifier creates a new notifier
func NewNotifier() *Notifier {
	return &Notifier{
		channels: make(map[string]chan *Alert),
	}
}

// Subscribe creates a notification channel
func (n *Notifier) Subscribe(id string) <-chan *Alert {
	n.mu.Lock()
	defer n.mu.Unlock()

	ch := make(chan *Alert, 100)
	n.channels[id] = ch
	return ch
}

// Unsubscribe removes a notification channel
func (n *Notifier) Unsubscribe(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if ch, exists := n.channels[id]; exists {
		close(ch)
		delete(n.channels, id)
	}
}

// Notify sends an alert to all subscribers
func (n *Notifier) Notify(alert *Alert) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, ch := range n.channels {
		select {
		case ch <- alert:
		default:
			// Channel is full, skip
		}
	}
}