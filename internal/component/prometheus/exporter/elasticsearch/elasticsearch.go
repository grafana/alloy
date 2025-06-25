package elasticsearch

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	commonCfg "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/elasticsearch_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.elasticsearch",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "elasticsearch"),
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
	Address:                   "http://localhost:9200",
	Timeout:                   5 * time.Second,
	Node:                      "_local",
	ExportClusterInfoInterval: 5 * time.Minute,
	IncludeAliases:            true,
}

type Arguments struct {
	Address                   string               `alloy:"address,attr,optional"`
	Timeout                   time.Duration        `alloy:"timeout,attr,optional"`
	AllNodes                  bool                 `alloy:"all,attr,optional"`
	Node                      string               `alloy:"node,attr,optional"`
	ExportIndices             bool                 `alloy:"indices,attr,optional"`
	ExportIndicesSettings     bool                 `alloy:"indices_settings,attr,optional"`
	ExportClusterSettings     bool                 `alloy:"cluster_settings,attr,optional"`
	ExportShards              bool                 `alloy:"shards,attr,optional"`
	IncludeAliases            bool                 `alloy:"aliases,attr,optional"`
	ExportSnapshots           bool                 `alloy:"snapshots,attr,optional"`
	ExportClusterInfoInterval time.Duration        `alloy:"clusterinfo_interval,attr,optional"`
	CA                        string               `alloy:"ca,attr,optional"`
	ClientPrivateKey          string               `alloy:"client_private_key,attr,optional"`
	ClientCert                string               `alloy:"client_cert,attr,optional"`
	InsecureSkipVerify        bool                 `alloy:"ssl_skip_verify,attr,optional"`
	ExportDataStreams         bool                 `alloy:"data_stream,attr,optional"`
	ExportSLM                 bool                 `alloy:"slm,attr,optional"`
	BasicAuth                 *commonCfg.BasicAuth `alloy:"basic_auth,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
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
