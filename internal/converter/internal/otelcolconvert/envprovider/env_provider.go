package envprovider

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/confmap"
)

const SchemeName = "env"

// provider is a custom environment variable provider
type provider struct{}

// NewFactory returns a custom environment provider factory
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(confmap.ProviderSettings) confmap.Provider {
	return &provider{}
}

// Retrieve processes an incoming environment variable reference and returns an escaped version of
// it.
func (s *provider) Retrieve(_ context.Context, val string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(val, s.Scheme()+":") {
		return nil, fmt.Errorf("%q environment variable scheme is not supported by %q provider", val, s.Scheme())
	}

	// $$ is an escaped OTel dollar sign. It's used here to prevent infinite recursion of the OTel
	// config parser.
	return confmap.NewRetrieved(fmt.Sprintf("$${%s}", val))
}

func (*provider) Scheme() string {
	return SchemeName
}

func (s *provider) Shutdown(context.Context) error {
	return nil
}
