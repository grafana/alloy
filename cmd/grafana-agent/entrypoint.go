package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/agent/internal/agentseed"
	"github.com/grafana/agent/internal/static/config"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/agent/internal/static/metrics"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/grafana/agent/internal/static/server"
	"github.com/grafana/agent/internal/static/supportbundle"
	"github.com/grafana/agent/internal/static/traces"
	"github.com/grafana/agent/internal/usagestats"
	"github.com/grafana/dskit/signals"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

// Entrypoint is the entrypoint of the application that starts all subsystems.
type Entrypoint struct {
	mut sync.Mutex

	reloader Reloader

	log *server.Logger
	cfg config.Config

	srv          *server.Server
	promMetrics  *metrics.Agent
	lokiLogs     *logs.Logs
	tempoTraces  *traces.Traces
	integrations config.Integrations
	reporter     *usagestats.Reporter

	reloadListener net.Listener
	reloadServer   *http.Server
}

// Reloader is any function that returns a new config.
type Reloader = func(log *server.Logger) (*config.Config, error)

// NewEntrypoint creates a new Entrypoint.
func NewEntrypoint(logger *server.Logger, cfg *config.Config, reloader Reloader) (*Entrypoint, error) {
	var (
		reg      = prometheus.DefaultRegisterer
		gatherer = prometheus.DefaultGatherer

		ep = &Entrypoint{
			log:      logger,
			reloader: reloader,
		}
		err error
	)

	ep.srv, err = server.New(logger, reg, gatherer, *cfg.Server, cfg.ServerFlags)
	if err != nil {
		return nil, err
	}

	ep.promMetrics, err = metrics.New(reg, cfg.Metrics, logger)
	if err != nil {
		return nil, err
	}

	ep.lokiLogs, err = logs.New(reg, cfg.Logs, logger, false)
	if err != nil {
		return nil, err
	}

	ep.tempoTraces, err = traces.New(ep.lokiLogs, ep.promMetrics.InstanceManager(), reg, cfg.Traces, logger)
	if err != nil {
		return nil, err
	}

	integrationGlobals, err := ep.createIntegrationsGlobals(cfg)
	if err != nil {
		return nil, err
	}

	ep.integrations, err = config.NewIntegrations(logger, &cfg.Integrations, integrationGlobals)
	if err != nil {
		return nil, err
	}

	agentseed.Init("", logger)
	ep.reporter, err = usagestats.NewReporter(logger)
	if err != nil {
		return nil, err
	}

	ep.wire(ep.srv.HTTP, ep.srv.GRPC)

	// Mostly everything should be up-to-date except for the server, which hasn't
	// been created yet.
	if err := ep.ApplyConfig(*cfg); err != nil {
		return nil, err
	}
	return ep, nil
}

func (ep *Entrypoint) createIntegrationsGlobals(cfg *config.Config) (config.IntegrationsGlobals, error) {
	hostname, err := instance.Hostname()
	if err != nil {
		return config.IntegrationsGlobals{}, fmt.Errorf("getting hostname: %w", err)
	}

	var listenPort int
	if ta, ok := ep.srv.HTTPAddress().(*net.TCPAddr); ok {
		listenPort = ta.Port
	}

	return config.IntegrationsGlobals{
		AgentIdentifier: fmt.Sprintf("%s:%d", hostname, listenPort),
		Metrics:         ep.promMetrics,
		Logs:            ep.lokiLogs,
		Tracing:         ep.tempoTraces,
		// TODO(rfratto): set SubsystemOptions here when v1 is removed.

		// Configure integrations to connect to the agent's in-memory server and avoid the network.
		DialContextFunc: ep.srv.DialContext,
		AgentBaseURL: &url.URL{
			Scheme: "http",
			Host:   cfg.ServerFlags.HTTP.InMemoryAddr,
		},
	}, nil
}

// ApplyConfig applies changes to the subsystems of the Agent.
func (ep *Entrypoint) ApplyConfig(cfg config.Config) error {
	ep.mut.Lock()
	defer ep.mut.Unlock()

	var failed bool

	if err := ep.log.ApplyConfig(cfg.Server); err != nil {
		level.Error(ep.log).Log("msg", "failed to update logger", "err", err)
		failed = true
	}

	if err := ep.srv.ApplyConfig(*cfg.Server); err != nil {
		level.Error(ep.log).Log("msg", "failed to update server", "err", err)
		failed = true
	}

	// Go through each component and update it.
	if err := ep.promMetrics.ApplyConfig(cfg.Metrics); err != nil {
		level.Error(ep.log).Log("msg", "failed to update prometheus", "err", err)
		failed = true
	}

	if err := ep.lokiLogs.ApplyConfig(cfg.Logs, false); err != nil {
		level.Error(ep.log).Log("msg", "failed to update loki", "err", err)
		failed = true
	}

	if err := ep.tempoTraces.ApplyConfig(ep.lokiLogs, ep.promMetrics.InstanceManager(), cfg.Traces); err != nil {
		level.Error(ep.log).Log("msg", "failed to update traces", "err", err)
		failed = true
	}

	integrationGlobals, err := ep.createIntegrationsGlobals(&cfg)
	if err != nil {
		level.Error(ep.log).Log("msg", "failed to update integrations", "err", err)
		failed = true
	} else if err := ep.integrations.ApplyConfig(&cfg.Integrations, integrationGlobals); err != nil {
		level.Error(ep.log).Log("msg", "failed to update integrations", "err", err)
		failed = true
	}

	ep.cfg = cfg
	if failed {
		return fmt.Errorf("changes did not apply successfully")
	}

	return nil
}

// wire is used to hook up API endpoints to components. It is called once after
// all subsystems are created.
func (ep *Entrypoint) wire(mux *mux.Router, grpc *grpc.Server) {
	ep.promMetrics.WireAPI(mux)
	ep.promMetrics.WireGRPC(grpc)

	ep.integrations.WireAPI(mux)
	ep.lokiLogs.WireAPI(mux)

	mux.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Agent is Healthy.\n")
	})

	mux.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		if !ep.promMetrics.Ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "Metrics are not ready yet.\n")

			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Agent is Ready.\n")
	})

	mux.HandleFunc("/-/config", func(rw http.ResponseWriter, r *http.Request) {
		ep.mut.Lock()
		cfg := ep.cfg
		ep.mut.Unlock()

		if cfg.EnableConfigEndpoints {
			bb, err := yaml.Marshal(cfg)
			if err != nil {
				http.Error(rw, fmt.Sprintf("failed to marshal config: %s", err), http.StatusInternalServerError)
			} else {
				_, _ = rw.Write(bb)
			}
		} else {
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write([]byte("404 - config endpoint is disabled"))
		}
	})

	mux.HandleFunc("/-/reload", ep.reloadHandler).Methods("GET", "POST")

	mux.HandleFunc("/-/support", ep.supportHandler).Methods("GET")
}

func (ep *Entrypoint) reloadHandler(rw http.ResponseWriter, r *http.Request) {
	success := ep.TriggerReload()
	if success {
		rw.WriteHeader(http.StatusOK)
	} else {
		rw.WriteHeader(http.StatusBadRequest)
	}
}

// getReporterMetrics creates the metrics map to send to usage reporter
func (ep *Entrypoint) getReporterMetrics() map[string]interface{} {
	ep.mut.Lock()
	defer ep.mut.Unlock()
	return map[string]interface{}{
		"enabled-features":     ep.cfg.EnabledFeatures,
		"enabled-integrations": ep.cfg.Integrations.EnabledIntegrations(),
	}
}

func getServerWriteTimeout(r *http.Request) time.Duration {
	srv, ok := r.Context().Value(http.ServerContextKey).(*http.Server)
	if ok && srv.WriteTimeout != 0 {
		return srv.WriteTimeout
	}
	return 30 * time.Second
}

func (ep *Entrypoint) supportHandler(rw http.ResponseWriter, r *http.Request) {
	ep.mut.Lock()
	cfg := ep.cfg
	ep.mut.Unlock()

	if cfg.DisableSupportBundle {
		rw.WriteHeader(http.StatusForbidden)
		_, _ = rw.Write([]byte("403 - support bundle generation is disabled; it can be re-enabled by removing the -disable-support-bundle flag"))
		return
	}

	duration := getServerWriteTimeout(r)
	if r.URL.Query().Has("duration") {
		d, err := strconv.Atoi(r.URL.Query().Get("duration"))
		if err != nil {
			http.Error(rw, fmt.Sprintf("duration value (in seconds) should be a positive integer: %s", err), http.StatusBadRequest)
			return
		}
		if d < 1 {
			http.Error(rw, "duration value (in seconds) should be larger than 1", http.StatusBadRequest)
			return
		}
		if float64(d) > duration.Seconds() {
			http.Error(rw, "duration value exceeds the server's write timeout", http.StatusBadRequest)
			return
		}
		duration = time.Duration(d) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	ep.mut.Lock()
	var (
		enabledFeatures = ep.cfg.EnabledFeatures
		httpSrvAddress  = ep.cfg.ServerFlags.HTTP.ListenAddress
	)
	ep.mut.Unlock()

	var logsBuffer bytes.Buffer
	logger := log.NewSyncLogger(log.NewLogfmtLogger(&logsBuffer))
	defer func() {
		ep.log.HookLogger.Set(nil)
	}()
	ep.log.HookLogger.Set(logger)

	var configBytes []byte
	var err error
	if cfg.EnableConfigEndpoints {
		configBytes, err = yaml.Marshal(cfg)
		if err != nil {
			http.Error(rw, fmt.Sprintf("failed to marshal config: %s", err), http.StatusInternalServerError)
		}
	}

	bundle, err := supportbundle.Export(ctx, enabledFeatures, configBytes, httpSrvAddress, ep.srv.DialContext)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := supportbundle.Serve(rw, bundle, &logsBuffer); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
}

// TriggerReload will cause the Entrypoint to re-request the config file and
// apply the latest config. TriggerReload returns true if the reload was
// successful.
func (ep *Entrypoint) TriggerReload() bool {
	level.Info(ep.log).Log("msg", "reload of config file requested")

	cfg, err := ep.reloader(ep.log)
	if err != nil {
		level.Error(ep.log).Log("msg", "failed to reload config file", "err", err)
		return false
	}
	cfg.LogDeprecations(ep.log)

	err = ep.ApplyConfig(*cfg)
	if err != nil {
		level.Error(ep.log).Log("msg", "failed to reload config file", "err", err)
		return false
	}

	return true
}

// pollConfig triggers a reload of the config on each tick of the ticker until the context
// completes.
func (ep *Entrypoint) pollConfig(ctx context.Context, sleepTime time.Duration) error {
	// Add an initial jitter to requests
	time.Sleep(ep.cfg.AgentManagement.JitterTime())

	t := time.NewTicker(sleepTime)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			ok := ep.TriggerReload()
			if !ok {
				level.Error(ep.log).Log("msg", "config reload did not succeed")
			}
		}
	}
}

// Stop stops the Entrypoint and all subsystems.
func (ep *Entrypoint) Stop() {
	ep.mut.Lock()
	defer ep.mut.Unlock()

	ep.integrations.Stop()
	ep.lokiLogs.Stop()
	ep.promMetrics.Stop()
	ep.tempoTraces.Stop()
	ep.srv.Close()

	if ep.reloadServer != nil {
		ep.reloadServer.Close()
	}
}

// Start starts the server used by the Entrypoint, and will block until a
// termination signal is sent to the process.
func (ep *Entrypoint) Start() error {
	var g run.Group

	// Create a signal handler that will stop the Entrypoint once a termination
	// signal is received.
	signalHandler := signals.NewHandler(ep.log)

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGHUP)

	defer func() {
		signal.Stop(notifier)
		close(notifier)
	}()

	g.Add(func() error {
		signalHandler.Loop()
		return nil
	}, func(e error) {
		signalHandler.Stop()
	})

	if ep.reloadServer != nil && ep.reloadListener != nil {
		g.Add(func() error {
			return ep.reloadServer.Serve(ep.reloadListener)
		}, func(e error) {
			ep.reloadServer.Close()
		})
	}

	if ep.cfg.AgentManagement.Enabled {
		managementContext, managementCancel := context.WithCancel(context.Background())
		defer managementCancel()

		sleepTime := ep.cfg.AgentManagement.SleepTime()
		g.Add(func() error {
			return ep.pollConfig(managementContext, sleepTime)
		}, func(e error) {
			managementCancel()
		})
	}

	srvContext, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()
	defer ep.srv.Close()

	g.Add(func() error {
		return ep.srv.Run(srvContext)
	}, func(e error) {
		srvCancel()
	})

	ep.mut.Lock()
	cfg := ep.cfg
	ep.mut.Unlock()
	if cfg.EnableUsageReport {
		g.Add(func() error {
			return ep.reporter.Start(srvContext, ep.getReporterMetrics)
		}, func(e error) {
			srvCancel()
		})
	}

	go func() {
		for range notifier {
			ep.TriggerReload()
		}
	}()

	return g.Run()
}
