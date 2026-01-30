package promtailconvert

import (
	"bytes"
	"flag"
	"fmt"
	"strings"

	promtailcfg "github.com/grafana/alloy/internal/loki/promtail/config"
	"github.com/grafana/alloy/internal/loki/promtail/file"
	"github.com/grafana/alloy/internal/loki/promtail/limit"
	"github.com/grafana/alloy/internal/loki/promtail/positions"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
	lokicfgutil "github.com/grafana/alloy/internal/loki/util/cfg"
	"github.com/grafana/dskit/flagext"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/promtailconvert/internal/build"
	"github.com/grafana/alloy/syntax/token/builder"
)

type Config struct {
	promtailcfg.Config `yaml:",inline"`
}

// Clone takes advantage of pass-by-value semantics to return a distinct *Config.
// This is primarily used to parse a different flag set without mutating the original *Config.
func (c *Config) Clone() flagext.Registerer {
	return func(c Config) *Config {
		return &c
	}(*c)
}

// Convert implements a Promtail config converter.
//
// extraArgs are supported to mirror the other converter params due to shared
// testing code but they should be passed empty to this converter.
func Convert(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	var (
		diags diag.Diagnostics
		cfg   Config
	)

	if len(extraArgs) > 0 {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("extra arguments are not supported for the promtail converter: %s", extraArgs))
		return nil, diags
	}

	// Set default values first.
	flagSet := flag.NewFlagSet("", flag.PanicOnError)
	err := lokicfgutil.Unmarshal(&cfg,
		lokicfgutil.Defaults(flagSet),
	)
	if err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to set default Promtail config values: %s", err))
		return nil, diags
	}

	// Unmarshall explicitly specified values
	if err := yaml.UnmarshalStrict(in, &cfg); err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to parse Promtail config: %s", err))
		return nil, diags
	}

	// Replicate promtails' handling of this deprecated field.
	if cfg.ClientConfig.URL.URL != nil {
		// if a single client config is used we add it to the multiple client config for backward compatibility
		cfg.ClientConfigs = append(cfg.ClientConfigs, cfg.ClientConfig)
	}

	f := builder.NewFile()
	diags = AppendAll(f, &cfg.Config, "", diags)
	diags.AddAll(common.ValidateNodes(f))

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to render Alloy config: %s", err.Error()))
		return nil, diags
	}

	if len(buf.Bytes()) == 0 {
		return nil, diags
	}

	prettyByte, newDiags := common.PrettyPrint([]byte(fixFunctionUse(buf.String())))
	diags.AddAll(newDiags)
	return prettyByte, diags
}

// There is not a way to express a function call in a struct to encode back into Alloy config, so we need to fix it
func fixFunctionUse(buf string) string {
	return strings.ReplaceAll(buf,
		`"\"spec.nodeName=\" + coalesce(sys.env(\"HOSTNAME\"), constants.hostname)"`,
		`"spec.nodeName=" + coalesce(sys.env("HOSTNAME"), constants.hostname)`,
	)
}

// AppendAll analyzes the entire promtail config in memory and transforms it
// into Alloy components. It then appends each argument to the file builder.
func AppendAll(f *builder.File, cfg *promtailcfg.Config, labelPrefix string, diags diag.Diagnostics) diag.Diagnostics {
	validateTopLevelConfig(cfg, &diags)

	writeReceivers := make([]loki.LogsReceiver, len(cfg.ClientConfigs))
	writeBlocks := make([]*builder.Block, len(cfg.ClientConfigs))
	// Each client config needs to be a separate remote_write,
	// because they may have different ExternalLabels fields.
	for i, cc := range cfg.ClientConfigs {
		writeBlocks[i], writeReceivers[i] = build.NewLokiWrite(&cc, &diags, i, labelPrefix)
	}

	gc := &build.GlobalContext{
		WriteReceivers:   writeReceivers,
		TargetSyncPeriod: cfg.TargetConfig.SyncPeriod,
		LabelPrefix:      labelPrefix,
	}

	for _, sc := range cfg.ScrapeConfig {
		appendScrapeConfig(f, &sc, &diags, gc, &cfg.Global.FileWatch, &cfg.PositionsConfig)
	}

	for _, write := range writeBlocks {
		f.Body().AppendBlock(write)
	}

	return diags
}

func DefaultPositionsConfig() positions.Config {
	// We obtain the default by registering the flags
	cfg := positions.Config{}
	cfg.RegisterFlags(flag.NewFlagSet("", flag.PanicOnError))
	return cfg
}

func DefaultLimitsConfig() limit.Config {
	cfg := limit.Config{}
	cfg.RegisterFlagsWithPrefix("", flag.NewFlagSet("", flag.PanicOnError))
	return cfg
}

func appendScrapeConfig(
	f *builder.File,
	cfg *scrapeconfig.Config,
	diags *diag.Diagnostics,
	gctx *build.GlobalContext,
	watchConfig *file.WatchConfig,
	positionsCfg *positions.Config,
) {

	b := build.NewScrapeConfigBuilder(f, diags, cfg, gctx)
	b.Sanitize()

	// Append all the SD components
	b.AppendSDs()
	// ConsulAgent does not come from Prometheus but only from Promtail.
	b.AppendConsulAgentSDs()

	// Append loki.source.file to process all SD components' targets.
	// If any relabelling is required, it will be done via a discovery.relabel component.
	// The files will be watched and the globs in file paths will be expanded using discovery.file component.
	// The log entries are sent to loki.process if processing is needed, or directly to loki.write components.
	b.AppendLokiSourceFile(watchConfig, positionsCfg)

	// Append all the components that produce logs directly.
	// If any relabelling is required, it will be done via a loki.relabel component.
	// The logs are sent to loki.process if processing is needed, or directly to loki.write components.
	b.AppendCloudFlareConfig()
	b.AppendJournalConfig()
	b.AppendPushAPI()
	b.AppendSyslogConfig()
	b.AppendGCPLog()
	b.AppendWindowsEventsConfig()
	b.AppendKafka()
	b.AppendAzureEventHubs()
	b.AppendGelfConfig()
	b.AppendHerokuDrainConfig()

	// Docker has a special treatment in Promtail, we replicate it here.
	b.AppendDockerPipeline()
}
