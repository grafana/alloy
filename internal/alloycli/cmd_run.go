package alloycli

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/grafana/ckit/advertise"
	"github.com/grafana/ckit/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"golang.org/x/exp/maps"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/boringcrypto"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/converter"
	convert_diag "github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/readyctx"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
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
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/windowspriority"
	"github.com/grafana/alloy/syntax/diag"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

func newAlloyRun() *alloyRun {
	return &alloyRun{
		inMemoryAddr:          "alloy.internal:12345",
		httpListenAddr:        "127.0.0.1:12345",
		storagePath:           "data-alloy/",
		minStability:          featuregate.StabilityGenerallyAvailable,
		uiPrefix:              "/",
		disableReporting:      false,
		enablePprof:           true,
		configFormat:          "alloy",
		clusterAdvInterfaces:  advertise.DefaultInterfaces,
		clusterMaxJoinPeers:   5,
		clusterRejoinInterval: 60 * time.Second,
		disableSupportBundle:  false,
		windowsPriority:       windowspriority.PriorityNormal,
		taskShutdownDeadline:  10 * time.Minute,
	}
}

func RunCommand() *cobra.Command {
	r := newAlloyRun()

	cmd := &cobra.Command{
		Use:   "run [flags] path",
		Short: "Run Grafana Alloy with Default Engine",
		Long: `The run subcommand runs Grafana Alloy in the foreground until an interrupt
is received.

run must be provided an argument pointing at the Alloy configuration
directory or file path to use. If the configuration directory or file path
wasn't specified, can't be loaded, or contains errors, run will exit
immediately.

If path is a directory, all *.alloy files in that directory will be combined
into a single unit. Subdirectories are not recursively searched for further merging.

run starts an HTTP server which can be used to debug Grafana Alloy or
force it to reload (by sending a GET or POST request to /-/reload). The listen
address can be changed through the --server.http.listen-addr flag.

By default, the HTTP server exposes a debugging UI at /. The path of the
debugging UI can be changed by providing a different value to
--server.http.ui-path-prefix.

Additionally, the HTTP server exposes the following debug endpoints:

  /debug/pprof   Go performance profiling tools

If reloading the config dir/file-path fails, Grafana Alloy will continue running in
its last valid state. Components which failed may be be listed as unhealthy,
depending on the nature of the reload error.
`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			return r.runCommand(cmd, args[0])
		},
	}

	mountRunFlags(r, cmd.Flags())
	return cmd
}

type ExtensionModeParams struct {
	// ConfigContents is a set of config file names and contents.
	Configs map[string][]byte

	// ModulePath is a value that will be used as "module_path" keyword value in Alloy config.
	ModulePath string
}

// NewRunAsExtensionCommand returns a standalone cobra command to run Alloy inside OTel collector as an extension.
//
// In extension mode:
//   - Certain features like remote config are disabled.
//   - Config reload on NOHUP signal is disabled.
//   - Alloy config contents passed directly, instead of file path cli argument.
func NewRunAsExtensionCommand(params ExtensionModeParams) *cobra.Command {
	r := newAlloyRun()

	cmd := &cobra.Command{
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rp := runParams{
				newReloadSignal: func() chan os.Signal {
					// SIGHUP is reserved by otel collector. Thus, use nop.
					return nil
				},
				newRemoteConfigService: func(_ *logging.Logger, reg prometheus.Registerer) (service.Service, error) {
					// Otel uses OpAMP, thus remote config management is disabled.
					return remotecfgservice.NewStub(reg), nil
				},
				reloadConfig: func(*alloy_runtime.Runtime, *httpservice.Service) error {
					return errors.New("config reload is not supported when Alloy is running in extension mode")
				},
				getConfig: func(rt *alloy_runtime.Runtime, httpSvc *httpservice.Service) (map[string][]byte, error) {
					alloySource, err := alloy_runtime.ParseSources(params.Configs)
					defer instrumentation.InstrumentConfig(err == nil, hashSourceFiles(params.Configs), r.clusterName)
					if err != nil {
						return params.Configs, fmt.Errorf("failed to parse config: %w", err)
					}

					httpSvc.SetSources(alloySource.SourceFiles())

					if err := rt.LoadSource(alloySource, nil, params.ModulePath); err != nil {
						return params.Configs, fmt.Errorf("error during the initial load: %w", err)
					}

					return params.Configs, nil
				},
			}

			return r.run(cmd.Context(), cmd.Flags(), rp)
		},
	}

	mountRunFlags(r, cmd.Flags())
	return cmd
}

func mountRunFlags(r *alloyRun, fset *pflag.FlagSet) {
	// Server flags
	fset.
		StringVar(&r.httpListenAddr, "server.http.listen-addr", r.httpListenAddr, "Address to listen for HTTP traffic on")
	fset.StringVar(&r.inMemoryAddr, "server.http.memory-addr", r.inMemoryAddr, "Address to listen for in-memory HTTP traffic on. Change if it collides with a real address")
	fset.StringVar(&r.uiPrefix, "server.http.ui-path-prefix", r.uiPrefix, "Prefix to serve the HTTP UI at")
	fset.
		BoolVar(&r.enablePprof, "server.http.enable-pprof", r.enablePprof, "Enable /debug/pprof profiling endpoints.")
	fset.
		BoolVar(&r.disableSupportBundle, "server.http.disable-support-bundle", r.disableSupportBundle, "Disable /-/support support bundle retrieval.")
	fset.BoolVar(&r.enableGraphQL, "server.http.enable-graphql", r.enableGraphQL, "Enable the GraphQL API")
	fset.BoolVar(&r.enableGraphQLPlayground, "server.http.enable-graphql-playground", r.enableGraphQLPlayground, "Enable the GraphQL playground UI (/graphql/playground)")

	// Cluster flags
	fset.
		BoolVar(&r.clusterEnabled, "cluster.enabled", r.clusterEnabled, "Start in clustered mode")
	fset.
		StringVar(&r.clusterNodeName, "cluster.node-name", r.clusterNodeName, "The name to use for this node")
	fset.
		StringVar(&r.clusterAdvAddr, "cluster.advertise-address", r.clusterAdvAddr, "Address to advertise to the cluster")
	fset.
		StringVar(&r.clusterJoinAddr, "cluster.join-addresses", r.clusterJoinAddr, "Comma-separated list of addresses to join the cluster at")
	fset.
		StringVar(&r.clusterDiscoverPeers, "cluster.discover-peers", r.clusterDiscoverPeers, "List of key-value tuples for discovering peers")
	fset.
		StringSliceVar(&r.clusterAdvInterfaces, "cluster.advertise-interfaces", r.clusterAdvInterfaces, "List of interfaces used to infer an address to advertise")
	fset.
		DurationVar(&r.clusterRejoinInterval, "cluster.rejoin-interval", r.clusterRejoinInterval, "How often to rejoin the list of peers")
	fset.
		IntVar(&r.clusterMaxJoinPeers, "cluster.max-join-peers", r.clusterMaxJoinPeers, "Number of peers to join from the discovered set")
	fset.
		StringVar(&r.clusterName, "cluster.name", r.clusterName, "The name of the cluster to join")
	fset.
		BoolVar(&r.clusterEnableTLS, "cluster.enable-tls", r.clusterEnableTLS, "Specifies whether TLS should be used for communication between peers")
	fset.
		StringVar(&r.clusterTLSCAPath, "cluster.tls-ca-path", r.clusterTLSCAPath, "Path to the CA certificate file")
	fset.
		StringVar(&r.clusterTLSCertPath, "cluster.tls-cert-path", r.clusterTLSCertPath, "Path to the certificate file")
	fset.
		StringVar(&r.clusterTLSKeyPath, "cluster.tls-key-path", r.clusterTLSKeyPath, "Path to the key file")
	fset.
		StringVar(&r.clusterTLSServerName, "cluster.tls-server-name", r.clusterTLSServerName, "Server name to use for TLS communication")
	fset.
		IntVar(&r.clusterWaitForSize, "cluster.wait-for-size", r.clusterWaitForSize, "Wait for the cluster to reach the specified number of instances before allowing components that use clustering to begin processing. Zero means disabled")
	fset.
		DurationVar(&r.clusterWaitTimeout, "cluster.wait-timeout", 0, "Maximum duration to wait for minimum cluster size before proceeding with available nodes. Zero means wait forever, no timeout")

	// Config flags
	fset.StringVar(&r.configFormat, "config.format", r.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	fset.BoolVar(&r.configBypassConversionErrors, "config.bypass-conversion-errors", r.configBypassConversionErrors, "Enable bypassing errors when converting")
	fset.StringVar(&r.configExtraArgs, "config.extra-args", r.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")

	// Misc flags
	fset.
		BoolVar(&r.disableReporting, "disable-reporting", r.disableReporting, "Disable reporting of enabled components to Grafana.")
	fset.StringVar(&r.storagePath, "storage.path", r.storagePath, "Base directory where components can store data")
	fset.Var(&r.minStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))
	if runtime.GOOS == "windows" {
		fset.StringVar(&r.windowsPriority, "windows.priority", r.windowsPriority, fmt.Sprintf("Process priority to use when running on windows. This flag is currently in public preview. Supported values: %s", strings.Join(slices.Collect(windowspriority.PriorityValues()), ", ")))
	}

	// Feature flags
	fset.BoolVar(&r.enableCommunityComps, "feature.community-components.enabled", r.enableCommunityComps, "Enable community components.")
	fset.DurationVar(&r.taskShutdownDeadline, "feature.component-shutdown-deadline", r.taskShutdownDeadline, "Maximum duration to wait for a component to shut down before giving up and logging an error")
	fset.BoolVar(&r.enableDirectFanout, "feature.prometheus.direct-fanout.enabled", r.enableDirectFanout, "Enable experimental direct fanout for metric forwarding without a global label store")

	addDeprecatedFlags(fset)
}

type alloyRun struct {
	inMemoryAddr                 string
	httpListenAddr               string
	storagePath                  string
	minStability                 featuregate.Stability
	uiPrefix                     string
	enablePprof                  bool
	disableReporting             bool
	clusterEnabled               bool
	clusterNodeName              string
	clusterAdvAddr               string
	clusterJoinAddr              string
	clusterDiscoverPeers         string
	clusterAdvInterfaces         []string
	clusterRejoinInterval        time.Duration
	clusterMaxJoinPeers          int
	clusterName                  string
	clusterEnableTLS             bool
	clusterTLSCAPath             string
	clusterTLSCertPath           string
	clusterTLSKeyPath            string
	clusterTLSServerName         string
	clusterWaitForSize           int
	clusterWaitTimeout           time.Duration
	configFormat                 string
	configBypassConversionErrors bool
	configExtraArgs              string
	disableSupportBundle         bool
	windowsPriority              string
	// Feature flags
	enableCommunityComps    bool
	taskShutdownDeadline    time.Duration
	enableDirectFanout      bool
	enableGraphQL           bool
	enableGraphQLPlayground bool
}

func (fr *alloyRun) checkExperimentalFlags() error {
	if fr.minStability.Permits(featuregate.StabilityExperimental) {
		return nil
	}

	const errMsg = "can only be used at experimental stability level. Use --stability.level=experimental to enable."

	if fr.enableDirectFanout {
		return fmt.Errorf("'--feature.prometheus.direct-fanout.enabled' %s", errMsg)
	}

	if fr.enableGraphQL {
		return fmt.Errorf("'--server.http.enable-graphql' %s", errMsg)
	}

	if fr.enableGraphQLPlayground {
		return fmt.Errorf("'--server.http.enable-graphql-playground' %s", errMsg)
	}

	return nil
}

func (fr *alloyRun) runCommand(cmd *cobra.Command, configPath string) error {
	if configPath == "" {
		return fmt.Errorf("path argument not provided")
	}

	loadConfig := func(rt *alloy_runtime.Runtime, httpSvc *httpservice.Service) (map[string][]byte, error) {
		sources, err := loadSourceFiles(configPath, fr.configFormat, fr.configBypassConversionErrors, fr.configExtraArgs)
		if err != nil {
			instrumentation.InstrumentConfig(false, [32]byte{}, fr.clusterName)
			return nil, fmt.Errorf("reading config path %q: %w", configPath, err)
		}

		alloySource, err := alloy_runtime.ParseSources(sources)
		defer instrumentation.InstrumentConfig(err == nil, hashSourceFiles(sources), fr.clusterName)
		if err != nil {
			return sources, fmt.Errorf("reading config path %q: %w", configPath, err)
		}

		httpSvc.SetSources(alloySource.SourceFiles())
		if err := rt.LoadSource(alloySource, nil, configPath); err != nil {
			return sources, fmt.Errorf("error during the initial load: %w", err)
		}

		return sources, nil
	}

	rp := runParams{
		newReloadSignal: func() chan os.Signal {
			reloadSignal := make(chan os.Signal, 1)
			signal.Notify(reloadSignal, syscall.SIGHUP)
			return reloadSignal
		},
		newRemoteConfigService: func(l *logging.Logger, reg prometheus.Registerer) (service.Service, error) {
			return remotecfgservice.New(remotecfgservice.Options{
				Logger:      l.Slog().With("service", "remotecfg"),
				ConfigPath:  configPath,
				StoragePath: fr.storagePath,
				Metrics:     reg,
			})
		},
		getConfig: loadConfig,
		reloadConfig: func(rt *alloy_runtime.Runtime, httpSvc *httpservice.Service) error {
			_, err := loadConfig(rt, httpSvc)
			return err
		},
	}

	return fr.run(cmd.Context(), cmd.Flags(), rp)
}

// getEnabledComponentsFunc returns a function that gets the current enabled components
func getEnabledComponentsFunc(f *alloy_runtime.Runtime) func() map[string]any {
	return func() map[string]any {
		components := component.GetAllComponents(f, component.InfoOptions{})
		if remoteCfgHost, err := remotecfgservice.GetHost(f); err == nil {
			components = append(components, component.GetAllComponents(remoteCfgHost, component.InfoOptions{})...)
		}
		componentNames := map[string]struct{}{}
		for _, c := range components {
			if c.Type != component.TypeBuiltin {
				continue
			}
			componentNames[c.ComponentName] = struct{}{}
		}
		return map[string]any{"enabled-components": maps.Keys(componentNames)}
	}
}

type runParams struct {
	// newReloadSignal constructs and returns a new load signal channel.
	newReloadSignal func() chan os.Signal

	// newRemoteConfigService constructs remote configuration service.
	newRemoteConfigService func(l *logging.Logger, regs prometheus.Registerer) (service.Service, error)

	// reloadConfig is callback called on config reload.
	reloadConfig func(rt *alloy_runtime.Runtime, httpSvc *httpservice.Service) error

	// getConfig callback provides initial alloy config.
	getConfig func(rt *alloy_runtime.Runtime, httpSvc *httpservice.Service) (map[string][]byte, error)
}

func (fr *alloyRun) run(ctx context.Context, fset *pflag.FlagSet, params runParams) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := fr.checkExperimentalFlags(); err != nil {
		return err
	}

	// Buffer logs until log format has been determined
	l, err := logging.NewDeferred(os.Stderr)
	if err != nil {
		return fmt.Errorf("building logger: %w", err)
	}

	slogger := l.Slog()
	slogger.Info("Alloy is starting")

	t, err := tracing.New(tracing.DefaultOptions)
	if err != nil {
		return fmt.Errorf("building tracer: %w", err)
	}

	// The non-windows path for this is just a return nil, but to protect against
	// refactoring assumptions we confirm that we're running on windows before setting the priority.
	if runtime.GOOS == "windows" && fr.windowsPriority != "normal" {
		if err := featuregate.CheckAllowed(
			featuregate.StabilityPublicPreview,
			fr.minStability,
			"Windows process priority"); err != nil {
			return err
		}

		if err := windowspriority.SetPriority(fr.windowsPriority); err != nil {
			return fmt.Errorf("setting process priority: %w", err)
		} else {
			slogger.Info("set process priority", "priority", fr.windowsPriority)
		}
	}

	// Set the global tracer provider to catch global traces, but ideally things
	// use the tracer provider given to them so the appropriate attributes get
	// injected.
	otel.SetTracerProvider(t)

	slogger.Info("boringcrypto enabled", "enabled", boringcrypto.Enabled)

	// Set the memory limit, this will honor GOMEMLIMIT if set
	// If there is a cgroup on linux it will use that
	err = applyAutoMemLimit(l)
	if err != nil {
		slogger.Error("failed to apply memory limit", "err", err)
	}

	// Enable the profiling.
	setMutexBlockProfiling(slogger)

	// Immediately start the tracer.
	go func() {
		err := t.Run(ctx)
		if err != nil {
			slogger.Error("running tracer returned an error", "err", err)
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
	_ = util.MustRegisterOrGet(reg, newResourcesCollector(slogger))

	// There's a cyclic dependency between the definition of the Alloy controller,
	// the reload/ready functions, and the HTTP service.
	//
	// To work around this, we lazily create variables for the functions the HTTP
	// service needs and set them after the Alloy controller exists.
	var (
		reload func() error
		ready  func() bool
	)

	clusterService, err := buildClusterService(ClusterOptions{
		Log:     slogger.With("service", "cluster"),
		Tracer:  t,
		Metrics: reg,

		EnableClustering:       fr.clusterEnabled,
		NodeName:               fr.clusterNodeName,
		AdvertiseAddress:       fr.clusterAdvAddr,
		ListenAddress:          fr.httpListenAddr,
		JoinPeers:              splitPeers(fr.clusterJoinAddr, ","),
		DiscoverPeers:          fr.clusterDiscoverPeers,
		RejoinInterval:         fr.clusterRejoinInterval,
		AdvertiseInterfaces:    fr.clusterAdvInterfaces,
		ClusterMaxJoinPeers:    fr.clusterMaxJoinPeers,
		ClusterName:            fr.clusterName,
		EnableTLS:              fr.clusterEnableTLS,
		TLSCertPath:            fr.clusterTLSCertPath,
		TLSCAPath:              fr.clusterTLSCAPath,
		TLSKeyPath:             fr.clusterTLSKeyPath,
		TLSServerName:          fr.clusterTLSServerName,
		MinimumClusterSize:     fr.clusterWaitForSize,
		MinimumSizeWaitTimeout: fr.clusterWaitTimeout,
	})
	if err != nil {
		return err
	}

	runtimeFlags := []string{}
	if !fr.disableSupportBundle {
		fset.VisitAll(func(f *pflag.Flag) {
			runtimeFlags = append(runtimeFlags, fmt.Sprintf("%s=%s", f.Name, f.Value.String()))
		})
	}

	httpService := httpservice.New(httpservice.Options{
		Logger:   l,
		Tracer:   t,
		Gatherer: prometheus.DefaultGatherer,

		ReadyFunc: func() bool { return ready() },
		ReloadFunc: func() error {
			return reload()
		},

		HTTPListenAddr:   fr.httpListenAddr,
		MemoryListenAddr: fr.inMemoryAddr,
		EnablePProf:      fr.enablePprof,
		MinStability:     fr.minStability,
		BundleContext: httpservice.SupportBundleContext{
			RuntimeFlags:         runtimeFlags,
			DisableSupportBundle: fr.disableSupportBundle,
		},
	})

	remoteCfgService, err := params.newRemoteConfigService(l, reg)
	if err != nil {
		return fmt.Errorf("failed to create the remotecfg service: %w", err)
	}

	liveDebuggingService := livedebugging.New()

	uiService := uiservice.New(uiservice.Options{
		UIPrefix:                fr.uiPrefix,
		CallbackManager:         liveDebuggingService.Data().(livedebugging.CallbackManager),
		Logger:                  slogger.With("service", "ui"),
		EnableGraphQL:           fr.enableGraphQL,
		EnableGraphQLPlayground: fr.enableGraphQLPlayground,
	})

	otelService := otel_service.New(slogger.With("service", "otel"))
	if otelService == nil {
		return fmt.Errorf("failed to create otel service")
	}

	if fr.enableDirectFanout {
		slogger.Info("global label store is disabled")
	}

	labelService := labelstore.New(slogger, reg, !fr.enableDirectFanout)
	alloyseed.Init(fr.storagePath, slogger)

	f, err := alloy_runtime.New(alloy_runtime.Options{
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
		TaskShutdownDeadline: fr.taskShutdownDeadline,
	})
	if err != nil {
		return err
	}

	ready = f.Ready
	reload = func() error {
		return params.reloadConfig(f, httpService)
	}

	// Alloy controller
	wg.Go(func() {
		f.Run(ctx)
	})

	// Report usage of enabled components
	if !fr.disableReporting {
		reporter, err := usagestats.NewReporter(slogger)
		if err != nil {
			return fmt.Errorf("failed to create reporter: %w", err)
		}
		go func() {
			err := reporter.Start(ctx, getEnabledComponentsFunc(f))
			if err != nil && !errors.Is(err, context.Canceled) {
				slogger.Error("failed to start reporter", "err", err)
			}
		}()
	}

	// Perform the initial load. This is done after starting the HTTP server so
	// that /metric and pprof endpoints are available while the Alloy controller
	// is loading.
	if source, err := params.getConfig(f, httpService); err != nil {
		// TODO: map diagnostics positions to actual positions in YAML config of otel collector.
		if diags, ok := errors.AsType[diag.Diagnostics](err); ok {
			p := diag.NewPrinter(diag.PrinterConfig{
				Color:              !color.NoColor,
				ContextLinesBefore: 1,
				ContextLinesAfter:  1,
			})
			_ = p.Fprint(os.Stderr, source, diags)

			// Print newline after the diagnostics.
			fmt.Println()

			return fmt.Errorf("could not perform the initial load successfully")
		}

		// Exit if the initial load fails.
		return err
	}

	// Signal to the caller (e.g. alloyengine extension) that the default engine is running
	if fn, ok := readyctx.OnReadyFromContext(ctx); ok && fn != nil {
		fn()
	}

	// By now, have either joined or started a new cluster.
	// Nodes initially join in the Viewer state. After the graph has been
	// loaded successfully, we can move to the Participant state to signal that
	// we wish to participate in reading or writing data.
	err = clusterService.ChangeState(ctx, peer.StateParticipant)
	if err != nil {
		return fmt.Errorf("failed to set clusterer state to Participant after initial load")
	}

	slogger.Info("{^_^} Alloy is running")

	reloadSignal := params.newReloadSignal()
	if reloadSignal != nil {
		defer signal.Stop(reloadSignal)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-reloadSignal:
			if err := reload(); err != nil {
				slogger.Error("failed to reload config", "err", err)
			} else {
				slogger.Info("config reloaded")
			}
		}
	}
}

func loadSourceFiles(path string, converterSourceFormat string, converterBypassErrors bool, configExtraArgs string) (map[string][]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		sources := map[string][]byte{}
		err := filepath.WalkDir(path, func(curPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// Skip all directories and don't recurse into child dirs that aren't at top-level
			if d.IsDir() {
				if curPath != path {
					return filepath.SkipDir
				}
				return nil
			}
			// Ignore files not ending in .alloy extension
			if !strings.HasSuffix(curPath, ".alloy") {
				return nil
			}

			bb, err := os.ReadFile(curPath)
			sources[curPath] = bb
			return err
		})
		if err != nil {
			return nil, err
		}

		return sources, nil
	}

	bb, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if converterSourceFormat != "alloy" {
		var diags convert_diag.Diagnostics
		ea, err := parseExtraArgs(configExtraArgs)
		if err != nil {
			return nil, err
		}

		bb, diags = converter.Convert(bb, converter.Input(converterSourceFormat), ea)
		hasError := hasErrorLevel(diags, convert_diag.SeverityLevelError)
		hasCritical := hasErrorLevel(diags, convert_diag.SeverityLevelCritical)
		if hasCritical || (!converterBypassErrors && hasError) {
			return nil, diags
		}
	}

	return map[string][]byte{path: bb}, nil
}

func hashSourceFiles(sources map[string][]byte) [sha256.Size]byte {
	if len(sources) == 0 {
		return [sha256.Size]byte{}
	}

	// Combined hash of all the sources.
	hash := sha256.New()

	// Sort keys so they are always added in the same order.
	keys := maps.Keys(sources)
	slices.Sort(keys)
	for _, key := range keys {
		hash.Write(sources[key])
	}

	return [32]byte(hash.Sum(nil))
}

// addDeprecatedFlags adds flags that are deprecated, have no effect, but we keep them for backwards compatibility.
func addDeprecatedFlags(fset *pflag.FlagSet) {
	deprecateFlagByName := func(fset *pflag.FlagSet, name string) {
		msg := "This flag is deprecated and has no effect."
		_ = fset.Bool(name, false, msg)
		err := fset.MarkDeprecated(name, msg)
		if err != nil { // this should never fail
			panic(err)
		}
	}
	deprecateFlagByName(fset, "cluster.use-discovery-v1")
	deprecateFlagByName(fset, "feature.prometheus.metric-validation-scheme")
}

func splitPeers(s, sep string) []string {
	if len(s) == 0 {
		return []string{}
	}
	return strings.Split(s, sep)
}

func setMutexBlockProfiling(l *slog.Logger) {
	mutexPercent := os.Getenv("PPROF_MUTEX_PROFILING_PERCENT")
	if mutexPercent != "" {
		rate, err := strconv.Atoi(mutexPercent)
		if err == nil && rate > 0 {
			// The 100/rate is because the value is interpreted as 1/rate. So 50 would be 100/50 = 2 and become 1/2 or 50%.
			runtime.SetMutexProfileFraction(100 / rate)
		} else {
			l.Error("error setting PPROF_MUTEX_PROFILING_PERCENT", "err", err, "value", mutexPercent)
			runtime.SetMutexProfileFraction(1000)
		}
	} else {
		// Why 1000 because that is what istio defaults to and that seemed reasonable to start with. This is 00.1% sampling.
		runtime.SetMutexProfileFraction(1000)
	}
	blockRate := os.Getenv("PPROF_BLOCK_PROFILING_RATE")
	if blockRate != "" {
		rate, err := strconv.Atoi(blockRate)
		if err == nil && rate > 0 {
			runtime.SetBlockProfileRate(rate)
		} else {
			l.Error("error setting PPROF_BLOCK_PROFILING_RATE", "err", err, "value", blockRate)
			runtime.SetBlockProfileRate(10_000)
		}
	} else {
		// This should have a negligible impact. This will track anything over 10_000ns, and will randomly sample shorter durations.
		// Default taken from https://github.com/DataDog/go-profiler-notes/blob/main/block.md
		runtime.SetBlockProfileRate(10_000)
	}
}
