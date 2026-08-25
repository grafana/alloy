package relay

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/alecthomas/units"

	commonconfig "github.com/grafana/alloy/internal/component/common/config"
)

const defaultAlertsPath = "/api/v2/alerts"

// Arguments configures the prometheus.alertmanager.relay component.
type Arguments struct {
	ListenAddress      string           `alloy:"listen_address,attr,optional"`
	ListenPort         int              `alloy:"listen_port,attr,optional"`
	WebhookPath        string           `alloy:"webhook_path,attr,optional"`
	MaxRequestBodySize units.Base2Bytes `alloy:"max_request_body_size,attr,optional"`

	Endpoint EndpointArguments `alloy:"endpoint,block"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		ListenAddress:      "127.0.0.1",
		ListenPort:         5001,
		WebhookPath:        "/webhook",
		MaxRequestBodySize: units.MiB,
	}
	args.Endpoint.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.ListenPort < 1 || args.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be between 1 and 65535")
	}
	if args.WebhookPath == "" || !strings.HasPrefix(args.WebhookPath, "/") {
		return fmt.Errorf("webhook_path must start with /")
	}
	if path.Clean(args.WebhookPath) != args.WebhookPath {
		return fmt.Errorf("webhook_path must be a clean URL path")
	}
	if args.MaxRequestBodySize <= 0 {
		return fmt.Errorf("max_request_body_size must be greater than 0")
	}

	return args.Endpoint.Validate()
}

// EndpointArguments configures the destination Alertmanager.
type EndpointArguments struct {
	URL     string        `alloy:"url,attr"`
	Timeout time.Duration `alloy:"timeout,attr,optional"`

	HTTPClientConfig *commonconfig.HTTPClientConfig `alloy:",squash"`
}

// SetToDefault implements syntax.Defaulter.
func (args *EndpointArguments) SetToDefault() {
	*args = EndpointArguments{
		Timeout:          10 * time.Second,
		HTTPClientConfig: commonconfig.CloneDefaultHTTPClientConfig(),
	}
}

// Validate implements syntax.Validator.
func (args *EndpointArguments) Validate() error {
	if args.Timeout <= 0 {
		return fmt.Errorf("endpoint timeout must be greater than 0")
	}
	if _, err := normalizeEndpointURL(args.URL); err != nil {
		return err
	}
	if args.HTTPClientConfig == nil {
		return fmt.Errorf("endpoint HTTP client configuration is missing")
	}
	if err := args.HTTPClientConfig.Validate(); err != nil {
		return fmt.Errorf("invalid endpoint HTTP client configuration: %w", err)
	}

	return nil
}

func normalizeEndpointURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("endpoint URL scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("endpoint URL must include a host")
	}
	if u.User != nil {
		return "", fmt.Errorf("endpoint URL must not contain user information; configure an authentication block instead")
	}
	if u.Fragment != "" {
		return "", fmt.Errorf("endpoint URL must not contain a fragment")
	}
	if u.Path == "" {
		u.Path = defaultAlertsPath
	}

	return u.String(), nil
}
