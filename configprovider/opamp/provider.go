// Copyright Grafana Labs and OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opamp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

const schemeName = "opamp"

type provider struct {
	logger *zap.Logger
}

// NewFactory returns a confmap.ProviderFactory for the opamp bootstrap+bridge scheme.
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(ps confmap.ProviderSettings) confmap.Provider {
	log := ps.Logger
	if log == nil {
		log = zap.NewNop()
	}
	return &provider{logger: log}
}

func (*provider) Scheme() string {
	return schemeName
}

var bridgesMu sync.Mutex

var bridges = map[string]*Bridge{}

func (p *provider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (*confmap.Retrieved, error) {
	path, err := parseURI(uri)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("opamp provider: %w", err)
	}

	bridgesMu.Lock()
	b, ok := bridges[abs]
	if !ok {
		b = newBridge(abs, p.logger)
		bridges[abs] = b
	}
	bridgesMu.Unlock()

	if watcher != nil {
		b.setWatcher(watcher)
	}

	if err := b.ensureStarted(ctx); err != nil {
		return nil, err
	}

	return confmap.NewRetrievedFromYAML([]byte(b.getMergedYAML()))
}

func (*provider) Shutdown(ctx context.Context) error {
	bridgesMu.Lock()
	defer bridgesMu.Unlock()
	var errs error
	for k, b := range bridges {
		errs = errors.Join(errs, b.shutdown(ctx))
		delete(bridges, k)
	}
	return errs
}
