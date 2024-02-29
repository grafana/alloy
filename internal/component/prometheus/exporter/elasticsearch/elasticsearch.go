package elasticsearch

import (
	"time"

	"github.com/grafana/agent/internal/component"
	commonCfg "github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/elasticsearch_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.elasticsearch",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "elasticsearch"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds non-zero default options for Arguments when it is
// unmarshaled from river.
var DefaultArguments = Arguments{
	Address:                   "http://localhost:9200",
	Timeout:                   5 * time.Second,
	Node:                      "_local",
	ExportClusterInfoInterval: 5 * time.Minute,
	IncludeAliases:            true,
}

type Arguments struct {
	Address                   string               `river:"address,attr,optional"`
	Timeout                   time.Duration        `river:"timeout,attr,optional"`
	AllNodes                  bool                 `river:"all,attr,optional"`
	Node                      string               `river:"node,attr,optional"`
	ExportIndices             bool                 `river:"indices,attr,optional"`
	ExportIndicesSettings     bool                 `river:"indices_settings,attr,optional"`
	ExportClusterSettings     bool                 `river:"cluster_settings,attr,optional"`
	ExportShards              bool                 `river:"shards,attr,optional"`
	IncludeAliases            bool                 `river:"aliases,attr,optional"`
	ExportSnapshots           bool                 `river:"snapshots,attr,optional"`
	ExportClusterInfoInterval time.Duration        `river:"clusterinfo_interval,attr,optional"`
	CA                        string               `river:"ca,attr,optional"`
	ClientPrivateKey          string               `river:"client_private_key,attr,optional"`
	ClientCert                string               `river:"client_cert,attr,optional"`
	InsecureSkipVerify        bool                 `river:"ssl_skip_verify,attr,optional"`
	ExportDataStreams         bool                 `river:"data_stream,attr,optional"`
	ExportSLM                 bool                 `river:"slm,attr,optional"`
	BasicAuth                 *commonCfg.BasicAuth `river:"basic_auth,block,optional"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *elasticsearch_exporter.Config {
	return &elasticsearch_exporter.Config{
		Address:                   a.Address,
		Timeout:                   a.Timeout,
		AllNodes:                  a.AllNodes,
		Node:                      a.Node,
		ExportIndices:             a.ExportIndices,
		ExportIndicesSettings:     a.ExportIndicesSettings,
		ExportClusterSettings:     a.ExportClusterSettings,
		ExportShards:              a.ExportShards,
		IncludeAliases:            a.IncludeAliases,
		ExportSnapshots:           a.ExportSnapshots,
		ExportClusterInfoInterval: a.ExportClusterInfoInterval,
		CA:                        a.CA,
		ClientPrivateKey:          a.ClientPrivateKey,
		ClientCert:                a.ClientCert,
		InsecureSkipVerify:        a.InsecureSkipVerify,
		ExportDataStreams:         a.ExportDataStreams,
		ExportSLM:                 a.ExportSLM,
		BasicAuth:                 a.BasicAuth.Convert(),
	}
}
