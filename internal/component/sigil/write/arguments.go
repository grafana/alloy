package write

import (
	"fmt"
	"net/url"
	"time"

	"github.com/grafana/alloy/internal/component/common/config"
)

type Arguments struct {
	Endpoints []*EndpointOptions `alloy:"endpoint,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{}
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if len(a.Endpoints) == 0 {
		return fmt.Errorf("at least one endpoint block must be specified")
	}
	return nil
}

type EndpointOptions struct {
	Name              string                   `alloy:"name,attr,optional"`
	URL               string                   `alloy:"url,attr"`
	RemoteTimeout     time.Duration            `alloy:"remote_timeout,attr,optional"`
	Headers           map[string]string        `alloy:"headers,attr,optional"`
	HTTPClientConfig  *config.HTTPClientConfig `alloy:",squash"`
	MinBackoff        time.Duration            `alloy:"min_backoff_period,attr,optional"`
	MaxBackoff        time.Duration            `alloy:"max_backoff_period,attr,optional"`
	MaxBackoffRetries int                      `alloy:"max_backoff_retries,attr,optional"`
	TenantID          string                   `alloy:"tenant_id,attr,optional"`
}

func GetDefaultEndpointOptions() EndpointOptions {
	return EndpointOptions{
		RemoteTimeout:     10 * time.Second,
		MinBackoff:        500 * time.Millisecond,
		MaxBackoff:        5 * time.Minute,
		MaxBackoffRetries: 10,
		HTTPClientConfig:  config.CloneDefaultHTTPClientConfig(),
	}
}

// SetToDefault implements syntax.Defaulter.
func (e *EndpointOptions) SetToDefault() {
	*e = GetDefaultEndpointOptions()
}

// Validate implements syntax.Validator.
func (e *EndpointOptions) Validate() error {
	if e.URL == "" {
		return fmt.Errorf("url must not be empty")
	}
	if _, err := url.Parse(e.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if e.MinBackoff > e.MaxBackoff {
		return fmt.Errorf("min_backoff_period (%s) must not exceed max_backoff_period (%s)", e.MinBackoff, e.MaxBackoff)
	}
	if e.MaxBackoffRetries < 0 {
		return fmt.Errorf("max_backoff_retries must be non-negative")
	}
	if e.HTTPClientConfig != nil {
		return e.HTTPClientConfig.Validate()
	}
	return nil
}
