package instance

import (
	"context"
	"net/http"

	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
)

// NoOpInstance implements the Instance interface in pkg/prom
// but does not do anything. Useful for tests.
type NoOpInstance struct{}

// Run implements Instance.
func (NoOpInstance) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Ready implements Instance.
func (NoOpInstance) Ready() bool {
	return true
}

// Update implements Instance.
func (NoOpInstance) Update(_ Config) error {
	return nil
}

// TargetsActive implements Instance.
func (NoOpInstance) TargetsActive() map[string][]*scrape.Target {
	return nil
}

// StorageDirectory implements Instance.
func (NoOpInstance) StorageDirectory() string {
	return ""
}

// WriteHandler implements Instance.
func (NoOpInstance) WriteHandler() http.Handler {
	return nil
}

// Appender implements Instance
func (NoOpInstance) Appender(_ context.Context) storage.Appender {
	return nil
}
