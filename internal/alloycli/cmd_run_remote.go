package alloycli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/fatih/color"
	"github.com/go-kit/log"
	"github.com/grafana/ckit/advertise"
	"github.com/grafana/ckit/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/boringcrypto"
	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/internal/service"
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	otel_service "github.com/grafana/alloy/internal/service/otel"
	remotecfgservice "github.com/grafana/alloy/internal/service/remotecfg"
	uiservice "github.com/grafana/alloy/internal/service/ui"
	"github.com/grafana/alloy/internal/static/config/instrumentation"
	"github.com/grafana/alloy/internal/usagestats"
	"github.com/grafana/alloy/syntax/diag"

	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
	commonconfig "github.com/prometheus/common/config"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

func runRemoteCommand() *cobra.Command {
	r := &alloyRunRemote{
		inMemoryAddr:          "alloy.internal:12345",
		httpListenAddr:        "127.0.0.1:12345",
		storagePath:           "data-alloy/",
		minStability:          featuregate.StabilityGenerallyAvailable,
		uiPrefix:              "/",
		disableReporting:      false,
		enablePprof:           true,
		clusterAdvInterfaces:  advertise.DefaultInterfaces,
		clusterMaxJoinPeers:   5,
		clusterRejoinInterval: 60 * time.Second,
		disableSupportBundle:  false,
	}

	cmd := &cobra.Command{
		Use:   "run-remote [flags]",
		Short: "Run Grafana Alloy",
		Long: `The run-remote subcommand runs Grafana Alloy in the foreground until an interrupt
is received.

run-remote must be provided flags to configure the remote config service.

run-remote starts an HTTP server which can be used to debug Grafana Alloy or
force it to reload (by sending a GET or POST request to /-/reload). The listen
address can be changed through the --server.http.listen-addr flag.

By default, the HTTP server exposes a debugging UI at /. The path of the
debugging UI can be changed by providing a different value to
--server.http.ui-path-prefix.

Additionally, the HTTP server exposes the following debug endpoints:

  /debug/pprof   Go performance profiling tools

If reloading the config fails, Grafana Alloy will continue running in
its last valid state. Components which failed may be be listed as unhealthy,
depending on the nature of the reload error.
`,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			return r.RunRemote(cmd)
		},
	}

	// Server flags
	cmd.Flags().
		StringVar(&r.httpListenAddr, "server.http.listen-addr", r.httpListenAddr, "Address to listen for HTTP traffic on")
	cmd.Flags().StringVar(&r.inMemoryAddr, "server.http.memory-addr", r.inMemoryAddr, "Address to listen for in-memory HTTP traffic on. Change if it collides with a real address")
	cmd.Flags().StringVar(&r.uiPrefix, "server.http.ui-path-prefix", r.uiPrefix, "Prefix to serve the HTTP UI at")
	cmd.Flags().
		BoolVar(&r.enablePprof, "server.http.enable-pprof", r.enablePprof, "Enable /debug/pprof profiling endpoints.")
	cmd.Flags().
		BoolVar(&r.disableSupportBundle, "server.http.disable-support-bundle", r.disableSupportBundle, "Disable /-/support support bundle retrieval.")

	// Cluster flags
	cmd.Flags().
		BoolVar(&r.clusterEnabled, "cluster.enabled", r.clusterEnabled, "Start in clustered mode")
	cmd.Flags().
		StringVar(&r.clusterNodeName, "cluster.node-name", r.clusterNodeName, "The name to use for this node")
	cmd.Flags().
		StringVar(&r.clusterAdvAddr, "cluster.advertise-address", r.clusterAdvAddr, "Address to advertise to the cluster")
	cmd.Flags().
		StringVar(&r.clusterJoinAddr, "cluster.join-addresses", r.clusterJoinAddr, "Comma-separated list of addresses to join the cluster at")
	cmd.Flags().
		StringVar(&r.clusterDiscoverPeers, "cluster.discover-peers", r.clusterDiscoverPeers, "List of key-value tuples for discovering peers")
	cmd.Flags().
		StringSliceVar(&r.clusterAdvInterfaces, "cluster.advertise-interfaces", r.clusterAdvInterfaces, "List of interfaces used to infer an address to advertise")
	cmd.Flags().
		DurationVar(&r.clusterRejoinInterval, "cluster.rejoin-interval", r.clusterRejoinInterval, "How often to rejoin the list of peers")
	cmd.Flags().
		IntVar(&r.clusterMaxJoinPeers, "cluster.max-join-peers", r.clusterMaxJoinPeers, "Number of peers to join from the discovered set")
	cmd.Flags().
		StringVar(&r.clusterName, "cluster.name", r.clusterName, "The name of the cluster to join")
	cmd.Flags().
		BoolVar(&r.clusterEnableTLS, "cluster.enable-tls", r.clusterEnableTLS, "Specifies whether TLS should be used for communication between peers")
	cmd.Flags().
		StringVar(&r.clusterTLSCAPath, "cluster.tls-ca-path", r.clusterTLSCAPath, "Path to the CA certificate file")
	cmd.Flags().
		StringVar(&r.clusterTLSCertPath, "cluster.tls-cert-path", r.clusterTLSCertPath, "Path to the certificate file")
	cmd.Flags().
		StringVar(&r.clusterTLSKeyPath, "cluster.tls-key-path", r.clusterTLSKeyPath, "Path to the key file")
	cmd.Flags().
		StringVar(&r.clusterTLSServerName, "cluster.tls-server-name", r.clusterTLSServerName, "Server name to use for TLS communication")

	// Misc flags
	cmd.Flags().
		BoolVar(&r.disableReporting, "disable-reporting", r.disableReporting, "Disable reporting of enabled components to Grafana.")
	cmd.Flags().StringVar(&r.storagePath, "storage.path", r.storagePath, "Base directory where components can store data")
	cmd.Flags().Var(&r.minStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))
	cmd.Flags().BoolVar(&r.enableCommunityComps, "feature.community-components.enabled", r.enableCommunityComps, "Enable community components.")

	return cmd
}

type alloyRunRemote struct {
	inMemoryAddr          string
	httpListenAddr        string
	storagePath           string
	minStability          featuregate.Stability
	uiPrefix              string
	enablePprof           bool
	disableReporting      bool
	clusterEnabled        bool
	clusterNodeName       string
	clusterAdvAddr        string
	clusterJoinAddr       string
	clusterDiscoverPeers  string
	clusterAdvInterfaces  []string
	clusterRejoinInterval time.Duration
	clusterMaxJoinPeers   int
	clusterName           string
	clusterEnableTLS      bool
	clusterTLSCAPath      string
	clusterTLSCertPath    string
	clusterTLSKeyPath     string
	clusterTLSServerName  string
	enableCommunityComps  bool
	disableSupportBundle  bool
}

func (fr *alloyRunRemote) RunRemote(cmd *cobra.Command) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := interruptContext()
	defer cancel()

	// Buffer logs until log format has been determined
	l, err := logging.NewDeferred(os.Stderr)
	if err != nil {
		return fmt.Errorf("building logger: %w", err)
	}

	t, err := tracing.New(tracing.DefaultOptions)
	if err != nil {
		return fmt.Errorf("building tracer: %w", err)
	}

	// Set the global tracer provider to catch global traces, but ideally things
	// use the tracer provider given to them so the appropriate attributes get
	// injected.
	otel.SetTracerProvider(t)

	level.Info(l).Log("boringcrypto enabled", boringcrypto.Enabled)

	// Set the memory limit, this will honor GOMEMLIMIT if set
	// If there is a cgroup will follow that
	memlimit.SetGoMemLimitWithOpts(memlimit.WithLogger(slog.New(l.Handler())))

	// Enable the profiling.
	setMutexBlockProfiling(l)

	// Immediately start the tracer.
	go func() {
		err := t.Run(ctx)
		if err != nil {
			level.Error(l).Log("msg", "running tracer returned an error", "err", err)
		}
	}()

	// TODO(rfratto): many of the dependencies we import register global metrics,
	// even when their code isn't being used. To reduce the number of series
	// generated by Alloy, we should switch to a custom registry.
	//
	// Before doing this, we need to ensure that anything using the default
	// registry that we want to keep can be given a custom registry so desired
	// metrics are still exposed.
	reg := prometheus.DefaultRegisterer
	reg.MustRegister(newResourcesCollector(l))

	// There's a cyclic dependency between the definition of the Alloy controller,
	// the reload/ready functions, and the HTTP service.
	//
	// To work around this, we lazily create variables for the functions the HTTP
	// service needs and set them after the Alloy controller exists.
	var (
		reload func() (*alloy_runtime.Source, error)
		ready  func() bool
	)

	clusterService, err := buildClusterService(clusterOptions{
		Log:     log.With(l, "service", "cluster"),
		Tracer:  t,
		Metrics: reg,

		EnableClustering:    fr.clusterEnabled,
		NodeName:            fr.clusterNodeName,
		AdvertiseAddress:    fr.clusterAdvAddr,
		ListenAddress:       fr.httpListenAddr,
		JoinPeers:           splitPeers(fr.clusterJoinAddr, ","),
		DiscoverPeers:       fr.clusterDiscoverPeers,
		RejoinInterval:      fr.clusterRejoinInterval,
		AdvertiseInterfaces: fr.clusterAdvInterfaces,
		ClusterMaxJoinPeers: fr.clusterMaxJoinPeers,
		ClusterName:         fr.clusterName,
		EnableTLS:           fr.clusterEnableTLS,
		TLSCertPath:         fr.clusterTLSCertPath,
		TLSCAPath:           fr.clusterTLSCAPath,
		TLSKeyPath:          fr.clusterTLSKeyPath,
		TLSServerName:       fr.clusterTLSServerName,
	})
	if err != nil {
		return err
	}

	runtimeFlags := []string{}
	if !fr.disableSupportBundle {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			runtimeFlags = append(runtimeFlags, fmt.Sprintf("%s=%s", f.Name, f.Value.String()))
		})
	}

	httpService := httpservice.New(httpservice.Options{
		Logger:   l,
		Tracer:   t,
		Gatherer: prometheus.DefaultGatherer,

		ReadyFunc:  func() bool { return ready() },
		ReloadFunc: func() (*alloy_runtime.Source, error) { return reload() },

		HTTPListenAddr:   fr.httpListenAddr,
		MemoryListenAddr: fr.inMemoryAddr,
		EnablePProf:      fr.enablePprof,
		MinStability:     fr.minStability,
		BundleContext: httpservice.SupportBundleContext{
			RuntimeFlags:         runtimeFlags,
			DisableSupportBundle: fr.disableSupportBundle,
		},
	})

	remoteCfgService, err := remotecfgservice.New(remotecfgservice.Options{
		Logger:      log.With(l, "service", "remotecfg"),
		ConfigPath:  "",
		StoragePath: fr.storagePath,
		Metrics:     reg,
	})
	if err != nil {
		return fmt.Errorf("failed to create the remotecfg service: %w", err)
	}

	liveDebuggingService := livedebugging.New()

	uiService := uiservice.New(uiservice.Options{
		UIPrefix:        fr.uiPrefix,
		CallbackManager: liveDebuggingService.Data().(livedebugging.CallbackManager),
	})

	otelService := otel_service.New(l)
	if otelService == nil {
		return fmt.Errorf("failed to create otel service")
	}

	labelService := labelstore.New(l, reg)
	alloyseed.Init(fr.storagePath, l)

	f := alloy_runtime.New(alloy_runtime.Options{
		Logger:               l,
		Tracer:               t,
		DataPath:             fr.storagePath,
		Reg:                  reg,
		MinStability:         fr.minStability,
		EnableCommunityComps: fr.enableCommunityComps,
		Services: []service.Service{
			clusterService,
			httpService,
			labelService,
			liveDebuggingService,
			otelService,
			remoteCfgService,
			uiService,
		},
	})

	ready = f.Ready
	reload = func() (*alloy_runtime.Source, error) {
		configPath, alloySource, err := loadAlloyRemoteSource(fr.storagePath, "TODO", "TODO", "60s", "TODO", "TODO")
		defer instrumentation.InstrumentConfig(err == nil, alloySource.SHA256(), fr.clusterName)

		if err != nil {
			return nil, fmt.Errorf("reading config path %q: %w", configPath, err)
		}
		if err := f.LoadSource(alloySource, nil, configPath); err != nil {
			return alloySource, fmt.Errorf("error during the initial load: %w", err)
		}

		return alloySource, nil
	}

	// Alloy controller
	{
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.Run(ctx)
		}()
	}

	// Report usage of enabled components
	if !fr.disableReporting {
		reporter, err := usagestats.NewReporter(l)
		if err != nil {
			return fmt.Errorf("failed to create reporter: %w", err)
		}
		go func() {
			err := reporter.Start(ctx, getEnabledComponentsFunc(f))
			if err != nil {
				level.Error(l).Log("msg", "failed to start reporter", "err", err)
			}
		}()
	}

	// Perform the initial reload. This is done after starting the HTTP server so
	// that /metric and pprof endpoints are available while the Alloy controller
	// is loading.
	if source, err := reload(); err != nil {
		var diags diag.Diagnostics
		if errors.As(err, &diags) {
			p := diag.NewPrinter(diag.PrinterConfig{
				Color:              !color.NoColor,
				ContextLinesBefore: 1,
				ContextLinesAfter:  1,
			})
			_ = p.Fprint(os.Stderr, source.RawConfigs(), diags)

			// Print newline after the diagnostics.
			fmt.Println()

			return fmt.Errorf("could not perform the initial load successfully")
		}

		// Exit if the initial load fails.
		return err
	}

	// By now, have either joined or started a new cluster.
	// Nodes initially join in the Viewer state. After the graph has been
	// loaded successfully, we can move to the Participant state to signal that
	// we wish to participate in reading or writing data.
	err = clusterService.ChangeState(ctx, peer.StateParticipant)
	if err != nil {
		return fmt.Errorf("failed to set clusterer state to Participant after initial load")
	}

	reloadSignal := make(chan os.Signal, 1)
	signal.Notify(reloadSignal, syscall.SIGHUP)
	defer signal.Stop(reloadSignal)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-reloadSignal:
			if _, err := reload(); err != nil {
				level.Error(l).Log("msg", "failed to reload config", "err", err)
			} else {
				level.Info(l).Log("msg", "config reloaded")
			}
		}
	}
}

func loadAlloyRemoteSource(storagePath string, remoteUrl string, id string, pollFrequency string, username string, password string) (string, *alloy_runtime.Source, error) {
	httpClient, err := commonconfig.NewClientFromConfig(commonconfig.HTTPClientConfig{BasicAuth: &commonconfig.BasicAuth{Username: username, Password: commonconfig.Secret(password)}}, "remoteconfig")
	if err != nil {
		return "", nil, err
	}
	client := collectorv1connect.NewCollectorServiceClient(
		httpClient,
		remoteUrl,
	)

	req := connect.NewRequest(&collectorv1.GetConfigRequest{
		Id:         id,
		Attributes: map[string]string{},
		Hash:       "",
	})

	gcr, err := client.GetConfig(context.Background(), req)
	if err != nil {
		return "", nil, err
	}

	content := gcr.Msg.Content

	path := storagePath + "/run-remote/" + "config.alloy"
	err = os.MkdirAll(storagePath+"/run-remote", os.ModePerm)
	if err != nil {
		return "", nil, err
	}
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", nil, err
	}

	source, err := alloy_runtime.ParseSource(path, []byte(content))
	return path, source, err
}
