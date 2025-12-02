package opampprovider // import "github.com/grafana/alloy/otelcol/provider/opampprovider"

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/confmap"
)

// Validate that provider implements the confmap.Provider interface.
var _ confmap.Provider = (*provider)(nil)

const (
	schemeName = "opamp"
)

type provider struct {
	logger *zap.Logger
}

func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(ps confmap.ProviderSettings) confmap.Provider {
	return &provider{
		logger: ps.Logger,
	}
}

func (p *provider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	// TODO: remove this and return something meaningful.
	val := uri[len(schemeName)+1:]
	p.logger.Info("OpAMP Provider Retrieve", zap.String("value", val))

	return confmap.NewRetrievedFromYAML([]byte(""))
}

func (p *provider) Scheme() string {
	return schemeName
}

func (p *provider) Shutdown(ctx context.Context) error {
	return nil
}
