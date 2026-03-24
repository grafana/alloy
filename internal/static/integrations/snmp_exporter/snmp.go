package snmp_exporter

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/snmp_exporter/collector"
	snmp_config "github.com/prometheus/snmp_exporter/config"
)

const (
	namespace = "snmp"
)

type snmpHandler struct {
	cfg     *Config
	snmpCfg *snmp_config.Config
	log     log.Logger
}

func Handler(w http.ResponseWriter, r *http.Request, logger log.Logger, snmpCfg *snmp_config.Config,
	targets []SNMPTarget, wParams map[string]snmp_config.WalkParams, concurrency int) {

	query := r.URL.Query()

	snmpTargets := make(map[string]SNMPTarget)
	for _, target := range targets {
		snmpTargets[target.Name] = target
	}

	var target string
	targetName := query.Get("target")
	if len(query["target"]) != 1 || targetName == "" {
		http.Error(w, "'target' parameter must be specified once", http.StatusBadRequest)
		return
	}

	t, ok := snmpTargets[targetName]
	if ok {
		target = t.Target
	} else {
		target = targetName
	}

	moduleParam := query.Get("module")
	if len(query["module"]) > 1 {
		http.Error(w, "'module' parameter must only be specified once", http.StatusBadRequest)
		return
	}
	if moduleParam == "" {
		moduleParam = "if_mib"
	}

	authName := query.Get("auth")
	if len(query["auth"]) > 1 {
		http.Error(w, "'auth' parameter must only be specified once", http.StatusBadRequest)
		return
	}
	if authName == "" {
		authName = "public_v2"
	}

	snmpContext := query.Get("snmp_context")
	if len(query["snmp_context"]) > 1 {
		http.Error(w, "'snmp_context' parameter must only be specified once", http.StatusBadRequest)
		return
	}

	walkParams := query.Get("walk_params")
	if len(query["walk_params"]) > 1 {
		http.Error(w, "'walk_params' parameter must only be specified once", http.StatusBadRequest)
		return
	}

	var nmodules []*collector.NamedModule
	for _, moduleName := range strings.Split(moduleParam, ",") {
		module, ok := (*snmpCfg).Modules[moduleName]
		if !ok {
			http.Error(w, fmt.Sprintf("Unknown module '%s'", moduleName), http.StatusBadRequest)
			return
		}

		// override module connection details with custom walk params if provided
		if walkParams != "" {
			zeroRetries := 0
			if wp, ok := wParams[walkParams]; ok {
				if wp.MaxRepetitions != 0 {
					module.WalkParams.MaxRepetitions = wp.MaxRepetitions
				}
				if wp.Retries != nil && wp.Retries != &zeroRetries {
					module.WalkParams.Retries = wp.Retries
				}
				if wp.Timeout != 0 {
					module.WalkParams.Timeout = wp.Timeout
				}
			} else {
				http.Error(w, fmt.Sprintf("Unknown walk_params '%s'", walkParams), http.StatusBadRequest)
				return
			}
		}
		nmodules = append(nmodules, collector.NewNamedModule(moduleName, module))
	}
	if walkParams != "" {
		logger = log.With(logger, "module_param", moduleParam, "target", target, "walk_params", walkParams)
	} else {
		logger = log.With(logger, "module_param", moduleParam, "target", target)
	}

	auth, ok := (*snmpCfg).Auths[authName]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown auth '%s'", authName), http.StatusBadRequest)
		return
	}

	level.Debug(logger).Log("msg", "Starting scrape")

	start := time.Now()
	registry := prometheus.NewRegistry()
	c := collector.New(r.Context(), target, authName, snmpContext, auth, nmodules, slog.New(logging.NewSlogGoKitHandler(logger)), NewSNMPMetrics(registry), concurrency, false)
	registry.MustRegister(c)
	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
	duration := time.Since(start).Seconds()
	level.Debug(logger).Log("msg", "Finished scrape", "duration_seconds", duration)
}

func (sh snmpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	Handler(w, r, sh.log, sh.snmpCfg, sh.cfg.SnmpTargets, sh.cfg.WalkParams, sh.cfg.SnmpConcurrency)
}
