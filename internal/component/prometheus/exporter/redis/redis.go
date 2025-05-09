package redis

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/redis_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.redis",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "redis"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds non-zero default options for Arguments when it is
// unmarshaled from Alloy.
var DefaultArguments = Arguments{
	IncludeExporterMetrics:  false,
	Namespace:               "redis",
	ConfigCommand:           "CONFIG",
	ConnectionTimeout:       15 * time.Second,
	SetClientName:           true,
	CheckKeyGroupsBatchSize: 10000,
	MaxDistinctKeyGroups:    100,
	ExportKeyValues:         true,
}

type Arguments struct {
	IncludeExporterMetrics bool `alloy:"include_exporter_metrics,attr,optional"`

	// exporter-specific config.
	//
	// The exporter binary config differs to this, but these
	// are the only fields that are relevant to the exporter struct.
	RedisAddr               string            `alloy:"redis_addr,attr"`
	RedisUser               string            `alloy:"redis_user,attr,optional"`
	RedisPassword           alloytypes.Secret `alloy:"redis_password,attr,optional"`
	RedisPasswordFile       string            `alloy:"redis_password_file,attr,optional"`
	RedisPasswordMapFile    string            `alloy:"redis_password_map_file,attr,optional"`
	Namespace               string            `alloy:"namespace,attr,optional"`
	ConfigCommand           string            `alloy:"config_command,attr,optional"`
	CheckKeys               []string          `alloy:"check_keys,attr,optional"`
	CheckKeyGroups          []string          `alloy:"check_key_groups,attr,optional"`
	CheckKeyGroupsBatchSize int64             `alloy:"check_key_groups_batch_size,attr,optional"`
	MaxDistinctKeyGroups    int64             `alloy:"max_distinct_key_groups,attr,optional"`
	CheckSingleKeys         []string          `alloy:"check_single_keys,attr,optional"`
	CheckStreams            []string          `alloy:"check_streams,attr,optional"`
	CheckSingleStreams      []string          `alloy:"check_single_streams,attr,optional"`
	ExportKeyValues         bool              `alloy:"export_key_values,attr,optional"`
	CountKeys               []string          `alloy:"count_keys,attr,optional"`
	ScriptPath              string            `alloy:"script_path,attr,optional"`
	ScriptPaths             []string          `alloy:"script_paths,attr,optional"`
	ConnectionTimeout       time.Duration     `alloy:"connection_timeout,attr,optional"`
	TLSClientKeyFile        string            `alloy:"tls_client_key_file,attr,optional"`
	TLSClientCertFile       string            `alloy:"tls_client_cert_file,attr,optional"`
	TLSCaCertFile           string            `alloy:"tls_ca_cert_file,attr,optional"`
	SetClientName           bool              `alloy:"set_client_name,attr,optional"`
	IsTile38                bool              `alloy:"is_tile38,attr,optional"`
	IsCluster               bool              `alloy:"is_cluster,attr,optional"`
	ExportClientList        bool              `alloy:"export_client_list,attr,optional"`
	ExportClientPort        bool              `alloy:"export_client_port,attr,optional"`
	RedisMetricsOnly        bool              `alloy:"redis_metrics_only,attr,optional"`
	PingOnConnect           bool              `alloy:"ping_on_connect,attr,optional"`
	InclSystemMetrics       bool              `alloy:"incl_system_metrics,attr,optional"`
	SkipTLSVerification     bool              `alloy:"skip_tls_verification,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.ScriptPath != "" && len(a.ScriptPaths) > 0 {
		return fmt.Errorf("only one of script_path and script_paths should be specified")
	}
	return nil
}

func (a *Arguments) Convert() *redis_exporter.Config {
	var scriptPath string
	if a.ScriptPath != "" {
		scriptPath = a.ScriptPath
	} else if len(a.ScriptPaths) > 0 {
		scriptPath = strings.Join(a.ScriptPaths, ",")
	}

	return &redis_exporter.Config{
		IncludeExporterMetrics:  a.IncludeExporterMetrics,
		RedisAddr:               a.RedisAddr,
		RedisUser:               a.RedisUser,
		RedisPassword:           config_util.Secret(a.RedisPassword),
		RedisPasswordFile:       a.RedisPasswordFile,
		RedisPasswordMapFile:    a.RedisPasswordMapFile,
		Namespace:               a.Namespace,
		ConfigCommand:           a.ConfigCommand,
		CheckKeys:               strings.Join(a.CheckKeys, ","),
		CheckKeyGroups:          strings.Join(a.CheckKeyGroups, ","),
		CheckKeyGroupsBatchSize: a.CheckKeyGroupsBatchSize,
		MaxDistinctKeyGroups:    a.MaxDistinctKeyGroups,
		CheckSingleKeys:         strings.Join(a.CheckSingleKeys, ","),
		CheckStreams:            strings.Join(a.CheckStreams, ","),
		CheckSingleStreams:      strings.Join(a.CheckSingleStreams, ","),
		ExportKeyValues:         a.ExportKeyValues,
		CountKeys:               strings.Join(a.CountKeys, ","),
		ScriptPath:              scriptPath,
		ConnectionTimeout:       a.ConnectionTimeout,
		TLSClientKeyFile:        a.TLSClientKeyFile,
		TLSClientCertFile:       a.TLSClientCertFile,
		TLSCaCertFile:           a.TLSCaCertFile,
		SetClientName:           a.SetClientName,
		IsTile38:                a.IsTile38,
		IsCluster:               a.IsCluster,
		ExportClientList:        a.ExportClientList,
		ExportClientPort:        a.ExportClientPort,
		RedisMetricsOnly:        a.RedisMetricsOnly,
		PingOnConnect:           a.PingOnConnect,
		InclSystemMetrics:       a.InclSystemMetrics,
		SkipTLSVerification:     a.SkipTLSVerification,
	}
}
