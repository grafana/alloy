//go:build !cgo

package oracledb_exporter

import (
	"errors"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/integrations"
)

// New is a stub used when Alloy is built with CGO_ENABLED=0. The underlying
// Oracle driver (godror) requires CGO, so the integration cannot run in
// no-cgo builds; configs that reference it surface this error at construction.
func New(_ log.Logger, _ *Config) (integrations.Integration, error) {
	return nil, errors.New("oracledb integration requires CGO; this Alloy binary was built with CGO_ENABLED=0")
}
