package alloycli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/fatih/color"
	"github.com/go-kit/log"
	"github.com/grafana/ckit/advertise"
	"github.com/grafana/ckit/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"golang.org/x/exp/maps"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/boringcrypto"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/converter"
	convert_diag "github.com/grafana/alloy/internal/converter/diag"
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

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

func runCommand() *cobra.Command {
	r := &alloyRun{
		inMemoryAddr:          "alloy.internal:12345",
		httpListenAddr:        "127.0.0.1:12345",
		storagePath:           "data-alloy/",
		minStability:          featuregate.StabilityGenerallyAvailable,
		uiPrefix:              "/",
		disableReporting:      false,
		enablePprof:           true,
		configFormat:          "alloy",
		clusterAdvInterfaces:  advertise.DefaultInterfaces,
		ClusterMaxJoinPeers:   5,
		clusterRejoinInterval: 60 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "run [flags] path",
		Short: "Run Grafana Alloy",
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
			return r.Run(args[0])
		},
	}

	// Server flags
	cmd.Flags().
		StringVar(&r.httpListenAddr, "server.http.listen-addr", r.httpListenAddr, "Address to listen for HTTP traffic on")
	cmd.Flags().StringVar(&r.inMemoryAddr, "server.http.memory-addr", r.inMemoryAddr, "Address to listen for in-memory HTTP traffic on. Change if it collides with a real address")
	cmd.Flags().StringVar(&r.uiPrefix, "server.http.ui-path-prefix", r.uiPrefix, "Prefix to serve the HTTP UI at")
	cmd.Flags().
		BoolVar(&r.enablePprof, "server.http.enable-pprof", r.enablePprof, "Enable /debug/pprof profiling endpoints.")

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
		IntVar(&r.ClusterMaxJoinPeers, "cluster.max-join-peers", r.ClusterMaxJoinPeers, "Number of peers to join from the discovered set")
	cmd.Flags().
		StringVar(&r.clusterName, "cluster.name", r.clusterName, "The name of the cluster to join")

	// Config flags
	cmd.Flags().StringVar(&r.configFormat, "config.format", r.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	cmd.Flags().BoolVar(&r.configBypassConversionErrors, "config.bypass-conversion-errors", r.configBypassConversionErrors, "Enable bypassing errors when converting")
	cmd.Flags().StringVar(&r.configExtraArgs, "config.extra-args", r.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")

	// Misc flags
	cmd.Flags().
		BoolVar(&r.disableReporting, "disable-reporting", r.disableReporting, "Disable reporting of enabled components to Grafana.")
	cmd.Flags().StringVar(&r.storagePath, "storage.path", r.storagePath, "Base directory where components can store data")
	cmd.Flags().Var(&r.minStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))
	cmd.Flags().BoolVar(&r.enableCommunityComps, "feature.community-components.enabled", r.enableCommunityComps, "Enable community components.")
	return cmd
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
	ClusterMaxJoinPeers          int
	clusterName                  string
	configFormat                 string
	configBypassConversionErrors bool
	configExtraArgs              string
	enableCommunityComps         bool
}

func (fr *alloyRun) Run(configPath string) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := interruptContext()
	defer cancel()

	if configPath == "" {
		return fmt.Errorf("path argument not provided")
	}

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
	if fr.minStability.Permits(featuregate.StabilityPublicPreview) {
		memlimit.SetGoMemLimitWithOpts(memlimit.WithLogger(slog.New(l.Handler())))
	}

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
		ClusterMaxJoinPeers: fr.ClusterMaxJoinPeers,
		ClusterName:         fr.clusterName,
		//TODO(alloy/#1274): graduate to GA once we have more confidence in this feature
		EnableStateUpdatesLimiter: fr.minStability.Permits(featuregate.StabilityPublicPreview),
		//TODO(alloy/#1274): graduate to GA once we have more confidence in this feature
		EnableDiscoveryV2: fr.minStability.Permits(featuregate.StabilityPublicPreview),
	})
	if err != nil {
		return err
	}

	httpService := httpservice.New(httpservice.Options{
		Logger:   log.With(l, "service", "http"),
		Tracer:   t,
		Gatherer: prometheus.DefaultGatherer,

		ReadyFunc:  func() bool { return ready() },
		ReloadFunc: func() (*alloy_runtime.Source, error) { return reload() },

		HTTPListenAddr:   fr.httpListenAddr,
		MemoryListenAddr: fr.inMemoryAddr,
		EnablePProf:      fr.enablePprof,
	})

	remoteCfgService, err := remotecfgservice.New(remotecfgservice.Options{
		Logger:      log.With(l, "service", "remotecfg"),
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
		alloySource, err := loadAlloySource(configPath, fr.configFormat, fr.configBypassConversionErrors, fr.configExtraArgs)
		defer instrumentation.InstrumentSHA256(alloySource.SHA256())
		defer instrumentation.InstrumentLoad(err == nil)

		if err != nil {
			return nil, fmt.Errorf("reading config path %q: %w", configPath, err)
		}
		if err := f.LoadSource(alloySource, nil); err != nil {
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

// getEnabledComponentsFunc returns a function that gets the current enabled components
func getEnabledComponentsFunc(f *alloy_runtime.Runtime) func() map[string]interface{} {
	return func() map[string]interface{} {
		components := component.GetAllComponents(f, component.InfoOptions{})
		componentNames := map[string]struct{}{}
		for _, c := range components {
			if c.Type != component.TypeBuiltin {
				continue
			}
			componentNames[c.ComponentName] = struct{}{}
		}
		return map[string]interface{}{"enabled-components": maps.Keys(componentNames)}
	}
}

func loadAlloySource(path string, converterSourceFormat string, converterBypassErrors bool, configExtraArgs string) (*alloy_runtime.Source, error) {
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

		return alloy_runtime.ParseSources(sources)
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

	instrumentation.InstrumentConfig(bb)

	return alloy_runtime.ParseSource(path, bb)
}

func interruptContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		select {
		case <-sig:
		case <-ctx.Done():
		}
		signal.Stop(sig)

		fmt.Fprintln(os.Stderr, "interrupt received")
	}()

	return ctx, cancel
}

func splitPeers(s, sep string) []string {
	if len(s) == 0 {
		return []string{}
	}
	return strings.Split(s, sep)
}

func setMutexBlockProfiling(l log.Logger) {
	mutexPercent := os.Getenv("PPROF_MUTEX_PROFILING_PERCENT")
	if mutexPercent != "" {
		rate, err := strconv.Atoi(mutexPercent)
		if err == nil && rate > 0 {
			// The 100/rate is because the value is interpreted as 1/rate. So 50 would be 100/50 = 2 and become 1/2 or 50%.
			runtime.SetMutexProfileFraction(100 / rate)
		} else {
			level.Error(l).Log("msg", "error setting PPROF_MUTEX_PROFILING_PERCENT", "err", err, "value", mutexPercent)
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
			level.Error(l).Log("msg", "error setting PPROF_BLOCK_PROFILING_RATE", "err", err, "value", blockRate)
			runtime.SetBlockProfileRate(10_000)
		}
	} else {
		// This should have a negligible impact. This will track anything over 10_000ns, and will randomly sample shorter durations.
		// Default taken from https://github.com/DataDog/go-profiler-notes/blob/main/block.md
		runtime.SetBlockProfileRate(10_000)
	}
}
