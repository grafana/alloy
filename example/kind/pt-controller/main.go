package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed ui.html
var uiFS embed.FS

// --- Configuration ---

type Config struct {
	GrafanaURL     string
	GrafanaAPIKey  string
	FMURL          string
	FMUsername      string
	FMPassword     string
	FMPipelineID   string
	AlloyConfigPath string
	ListenAddr     string
}

func loadConfig() Config {
	return Config{
		GrafanaURL:      envOrDefault("GRAFANA_URL", "https://thampiotr.grafana.net"),
		GrafanaAPIKey:   mustEnv("GRAFANA_SERVICE_ACCOUNT_TOKEN"),
		FMURL:           envOrDefault("REMOTE_CFG_URL", "https://fleet-management-prod-006.grafana.net"),
		FMUsername:       mustEnv("REMOTE_CFG_USERNAME"),
		FMPassword:      mustEnv("GRAFANA_CLOUD_API_KEY"),
		FMPipelineID:    mustEnv("FM_PIPELINE_ID"),
		AlloyConfigPath: envOrDefault("ALLOY_CONFIG_PATH", "../config/shop/config.alloy"),
		ListenAddr:      envOrDefault("LISTEN_ADDR", ":8090"),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Warn("environment variable not set", "key", key)
	}
	return v
}

// --- PT Rule ---

type PTRule struct {
	AlertUID   string   `json:"alert_uid"`
	AlertName  string   `json:"alert_name"`
	Namespaces []string `json:"namespaces"`
	Condition  string   `json:"condition"` // "pending" or "firing"
	Level      string   `json:"level"`     // "debug", "normal", "minimal", "off"
	TTL        string   `json:"ttl"`
}

func (r PTRule) NamespaceRegex() string {
	return strings.Join(r.Namespaces, "|")
}

type ControllerState struct {
	Phase          string    `json:"phase"` // "watching", "activated", "cooldown"
	ActivatedAt    time.Time `json:"activated_at,omitempty"`
	DeactivatesAt  time.Time `json:"deactivates_at,omitempty"`
	CooldownUntil  time.Time `json:"cooldown_until,omitempty"`
	ActiveRule     *PTRule   `json:"active_rule,omitempty"`
	LastAlertState string    `json:"last_alert_state"`
	LastPollTime   time.Time `json:"last_poll_time"`
	LastError      string    `json:"last_error,omitempty"`
}

// --- Controller ---

type Controller struct {
	mu     sync.RWMutex
	cfg    Config
	logger *slog.Logger
	rules  []PTRule
	state  ControllerState
	client *http.Client
}

func NewController(cfg Config, logger *slog.Logger) *Controller {
	return &Controller{
		cfg:    cfg,
		logger: logger,
		state:  ControllerState{Phase: "no_rule"},
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Controller) Run() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.tick()
	}
}

func (c *Controller) tick() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.rules) == 0 {
		return
	}

	// Check TTL expiry first
	if c.state.Phase == "activated" && time.Now().After(c.state.DeactivatesAt) {
		c.logger.Info("TTL expired, reverting to normal telemetry")
		if err := c.updatePipeline("^$"); err != nil {
			c.state.LastError = fmt.Sprintf("revert failed: %v", err)
			c.logger.Error("failed to revert pipeline", "error", err)
		} else {
			c.state.Phase = "cooldown"
			c.state.CooldownUntil = time.Now().Add(1 * time.Hour)
			c.state.LastError = ""
			c.logger.Info("reverted to normal telemetry, entering cooldown")
		}
		return
	}

	if c.state.Phase == "cooldown" && time.Now().After(c.state.CooldownUntil) {
		c.state.Phase = "watching"
		c.logger.Info("cooldown expired, watching again")
	}

	if c.state.Phase != "watching" {
		return
	}

	// Poll alert state for each configured rule
	for i := range c.rules {
		rule := &c.rules[i]
		alertState, err := c.pollAlertState(rule.AlertUID)
		c.state.LastPollTime = time.Now()
		if err != nil {
			c.state.LastError = fmt.Sprintf("poll failed: %v", err)
			c.logger.Error("failed to poll alert", "uid", rule.AlertUID, "error", err)
			continue
		}
		c.state.LastAlertState = alertState
		c.state.LastError = ""

		if alertState == rule.Condition {
			ttl, _ := time.ParseDuration(rule.TTL)
			if ttl == 0 {
				ttl = 15 * time.Minute
			}

			c.logger.Info("alert matches condition, activating debug telemetry",
				"alert", rule.AlertName, "state", alertState, "namespaces", rule.Namespaces, "ttl", ttl)

			if err := c.updatePipeline(rule.NamespaceRegex()); err != nil {
				c.state.LastError = fmt.Sprintf("activation failed: %v", err)
				c.logger.Error("failed to activate pipeline", "error", err)
				continue
			}

			c.state.Phase = "activated"
			c.state.ActivatedAt = time.Now()
			c.state.DeactivatesAt = time.Now().Add(ttl)
			c.state.ActiveRule = rule
			c.state.LastError = ""
			return
		}
	}
}

// --- Grafana Alerting API ---

func (c *Controller) pollAlertState(alertUID string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/provisioning/alert-rules/%s", c.cfg.GrafanaURL, alertUID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.cfg.GrafanaAPIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}

	// The provisioning API doesn't return runtime state directly.
	// Use the ruler API instead.
	return c.pollAlertStateRuler(alertUID)
}

func (c *Controller) pollAlertStateRuler(alertUID string) (string, error) {
	url := fmt.Sprintf("%s/api/alerting/grafana/api/v1/rules", c.cfg.GrafanaURL)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.cfg.GrafanaAPIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string][]struct {
		Rules []struct {
			Labels map[string]string `json:"labels"`
			State  string            `json:"state"`
			Alerts []struct {
				State  string            `json:"state"`
				Labels map[string]string `json:"labels"`
			} `json:"alerts"`
		} `json:"rules"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}

	for _, groups := range result {
		for _, group := range groups {
			for _, rule := range group.Rules {
				alertName := rule.Labels["alertname"]
				if alertName == "" {
					continue
				}
				// Match by looking through all rules for our UID
				// The ruler API uses alertname, not UID directly
				if rule.State != "" {
					for _, alert := range rule.Alerts {
						_ = alert // we just need the rule-level state
					}
					// Check if this might be our rule by checking alerts
					if matchesAlertUID(rule, alertUID) {
						return rule.State, nil
					}
				}
			}
		}
	}

	return "normal", nil
}

func matchesAlertUID(_ any, _ string) bool {
	// For the hackathon, we match by position -- the first rule that has
	// a non-normal state. In production, we'd match by UID annotation.
	// This is handled by the simpler approach below.
	return false
}

func (c *Controller) listAlertRules() ([]map[string]any, error) {
	url := fmt.Sprintf("%s/api/v1/provisioning/alert-rules", c.cfg.GrafanaURL)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.cfg.GrafanaAPIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var rules []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// --- Fleet Management Pipeline API ---

var debugRegex = regexp.MustCompile(`(regex\s*=\s*)"(\^\$|[^"]*)"(\s*//\s*<-- PT)`)

func (c *Controller) updatePipeline(namespace string) error {
	configBytes, err := os.ReadFile(c.cfg.AlloyConfigPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	config := string(configBytes)
	newConfig := debugRegex.ReplaceAllString(config, fmt.Sprintf(`${1}"%s"${3}`, namespace))

	if newConfig == config && namespace != "^$" {
		return fmt.Errorf("no PT control markers found in config (expected regex with // <-- PT comment)")
	}

	if c.cfg.FMPipelineID == "" {
		return fmt.Errorf("FM_PIPELINE_ID is not set")
	}

	c.logger.Info("updating FM pipeline", "namespace", namespace, "pipeline_id", c.cfg.FMPipelineID)

	payload := map[string]any{
		"pipeline": map[string]any{
			"id":       c.cfg.FMPipelineID,
			"name":     "pt_pipeline",
			"contents": newConfig,
			"matchers": []string{`workloadName="alloy-daemon"`},
			"enabled":  true,
		},
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/pipeline.v1.PipelineService/UpdatePipeline", c.cfg.FMURL)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.cfg.FMUsername, c.cfg.FMPassword)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("FM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("FM status %d: %s", resp.StatusCode, string(respBody))
	}

	c.logger.Info("FM pipeline updated successfully", "namespace", namespace)
	return nil
}

// --- HTTP handlers ---

func (c *Controller) handleUI(w http.ResponseWriter, _ *http.Request) {
	tmpl, err := template.ParseFS(uiFS, "ui.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	tmpl.Execute(w, nil)
}

func (c *Controller) handleListAlerts(w http.ResponseWriter, _ *http.Request) {
	rules, err := c.listAlertRules()
	if err != nil {
		writeJSONError(w, err.Error(), 502)
		return
	}

	type alertInfo struct {
		UID   string `json:"uid"`
		Title string `json:"title"`
	}
	var alerts []alertInfo
	for _, r := range rules {
		uid, _ := r["uid"].(string)
		title, _ := r["title"].(string)
		if uid != "" && title != "" {
			alerts = append(alerts, alertInfo{UID: uid, Title: title})
		}
	}
	writeJSON(w, alerts)
}

func (c *Controller) handleSaveRule(w http.ResponseWriter, r *http.Request) {
	var rule PTRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSONError(w, "invalid request body", 400)
		return
	}

	c.mu.Lock()
	c.rules = []PTRule{rule}
	if c.state.Phase == "no_rule" {
		c.state.Phase = "watching"
	}
	c.mu.Unlock()

	c.logger.Info("PT rule saved", "alert", rule.AlertName, "namespaces", rule.Namespaces, "condition", rule.Condition, "level", rule.Level, "ttl", rule.TTL)
	writeJSON(w, map[string]string{"status": "saved"})
}

func (c *Controller) handleGetRules(w http.ResponseWriter, _ *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	writeJSON(w, c.rules)
}

func (c *Controller) handleStatus(w http.ResponseWriter, _ *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	writeJSON(w, c.state)
}

func (c *Controller) handleActivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Namespace  string   `json:"namespace"`
		Namespaces []string `json:"namespaces"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", 400)
		return
	}
	ns := req.Namespaces
	if len(ns) == 0 && req.Namespace != "" {
		ns = []string{req.Namespace}
	}
	if len(ns) == 0 {
		writeJSONError(w, "namespace(s) required", 400)
		return
	}
	regex := strings.Join(ns, "|")

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.updatePipeline(regex); err != nil {
		c.logger.Error("manual activation failed", "namespaces", ns, "error", err)
		writeJSONError(w, err.Error(), 500)
		return
	}

	c.state.Phase = "activated"
	c.state.ActivatedAt = time.Now()
	c.state.DeactivatesAt = time.Now().Add(15 * time.Minute)
	writeJSON(w, map[string]any{"status": "activated", "namespaces": ns})
}

func (c *Controller) handleDeactivate(w http.ResponseWriter, _ *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.updatePipeline("^$"); err != nil {
		c.logger.Error("deactivation failed", "error", err)
		writeJSONError(w, err.Error(), 500)
		return
	}

	c.state.Phase = "watching"
	writeJSON(w, map[string]string{"status": "deactivated"})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Poll alert state via the Prometheus-compatible ruler API.
// Endpoint: /api/prometheus/grafana/api/v1/rules
func (c *Controller) pollAlertSimple() (string, error) {
	if len(c.rules) == 0 {
		return "normal", nil
	}
	rule := c.rules[0]

	url := fmt.Sprintf("%s/api/prometheus/grafana/api/v1/rules", c.cfg.GrafanaURL)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.cfg.GrafanaAPIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body)[:200])
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			Groups []struct {
				Rules []struct {
					Name   string `json:"name"`
					State  string `json:"state"`
					Alerts []struct {
						State  string            `json:"state"`
						Labels map[string]string `json:"labels"`
					} `json:"alerts"`
				} `json:"rules"`
			} `json:"groups"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	for _, group := range result.Data.Groups {
		for _, r := range group.Rules {
			if r.Name != rule.AlertName {
				continue
			}
			// Check individual alert instances for the worst state
			for _, alert := range r.Alerts {
				state := strings.ToLower(alert.State)
				if state == "firing" || state == "alerting" {
					return "firing", nil
				}
				if state == "pending" {
					return "pending", nil
				}
			}
			// Rule-level state: "inactive", "pending", "firing"
			state := strings.ToLower(r.State)
			if state == "firing" {
				return "firing", nil
			}
			if state == "pending" {
				return "pending", nil
			}
			return "normal", nil
		}
	}

	return "normal", nil
}

// --- Main ---

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := loadConfig()

	ctrl := NewController(cfg, logger)

	// Override the tick to use simpler polling
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			func() {
				ctrl.mu.Lock()
				defer ctrl.mu.Unlock()

				if len(ctrl.rules) == 0 {
					return
				}

				// Check TTL expiry
				if ctrl.state.Phase == "activated" && time.Now().After(ctrl.state.DeactivatesAt) {
					logger.Info("TTL expired, reverting to normal telemetry")
					if err := ctrl.updatePipeline("^$"); err != nil {
						ctrl.state.LastError = fmt.Sprintf("revert failed: %v", err)
						logger.Error("failed to revert pipeline", "error", err)
					} else {
						ctrl.state.Phase = "cooldown"
						ctrl.state.CooldownUntil = time.Now().Add(1 * time.Hour)
						ctrl.state.LastError = ""
					}
					return
				}

				if ctrl.state.Phase == "cooldown" && time.Now().After(ctrl.state.CooldownUntil) {
					ctrl.state.Phase = "watching"
				}

				if ctrl.state.Phase != "watching" {
					return
				}

				state, err := ctrl.pollAlertSimple()
				ctrl.state.LastPollTime = time.Now()
				if err != nil {
					ctrl.state.LastError = fmt.Sprintf("poll: %v", err)
					return
				}
				ctrl.state.LastAlertState = state
				ctrl.state.LastError = ""

				rule := ctrl.rules[0]
				if state == rule.Condition {
					ttl, _ := time.ParseDuration(rule.TTL)
					if ttl == 0 {
						ttl = 15 * time.Minute
					}
				logger.Info("alert matches condition, activating",
					"alert", rule.AlertName, "state", state, "namespaces", rule.Namespaces, "ttl", ttl)

				if err := ctrl.updatePipeline(rule.NamespaceRegex()); err != nil {
						ctrl.state.LastError = fmt.Sprintf("activate: %v", err)
						return
					}
					ctrl.state.Phase = "activated"
					ctrl.state.ActivatedAt = time.Now()
					ctrl.state.DeactivatesAt = time.Now().Add(ttl)
					ctrl.state.ActiveRule = &rule
					ctrl.state.LastError = ""
				}
			}()
		}
	}()

	// Reset pipeline to normal state on startup
	logger.Info("resetting FM pipeline to normal state on startup")
	if err := ctrl.updatePipeline("^$"); err != nil {
		logger.Error("failed to reset pipeline on startup", "error", err)
	} else {
		logger.Info("FM pipeline reset to normal state")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", ctrl.handleUI)
	mux.HandleFunc("GET /api/alerts", ctrl.handleListAlerts)
	mux.HandleFunc("POST /api/rules", ctrl.handleSaveRule)
	mux.HandleFunc("GET /api/rules", ctrl.handleGetRules)
	mux.HandleFunc("GET /api/status", ctrl.handleStatus)
	mux.HandleFunc("POST /api/activate", ctrl.handleActivate)
	mux.HandleFunc("POST /api/deactivate", ctrl.handleDeactivate)

	logger.Info("PT controller starting", "addr", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
