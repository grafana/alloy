package config

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/grafana/agent/internal/static/config/instrumentation"
	"github.com/prometheus/common/config"
)

// supported remote config provider schemes
const (
	httpScheme  = "http"
	httpsScheme = "https"
)

// remoteOpts struct contains agent remote config options
type remoteOpts struct {
	url              *url.URL
	HTTPClientConfig *config.HTTPClientConfig
	headers          map[string]string
}

// remoteProvider interface should be implemented by config providers
type remoteProvider interface {
	retrieve() ([]byte, http.Header, error)
}

// newRemoteProvider constructs a new remote configuration provider. The rawURL is parsed
// and a provider is constructed based on the URL's scheme.
func newRemoteProvider(rawURL string, opts *remoteOpts) (remoteProvider, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing rawURL %s: %w", rawURL, err)
	}
	if opts == nil {
		// Default provider opts
		opts = &remoteOpts{}
	}
	opts.url = u

	switch u.Scheme {
	case "":
		// if no scheme, assume local file path, return nil and let caller handle.
		return nil, nil
	case httpScheme, httpsScheme:
		httpP, err := newHTTPProvider(opts)
		if err != nil {
			return nil, fmt.Errorf("error constructing httpProvider: %w", err)
		}
		return httpP, nil
	default:
		return nil, fmt.Errorf("remote config scheme not supported: %s", u.Scheme)
	}
}

// Remote Config Providers
// httpProvider - http/https provider
type httpProvider struct {
	myURL      *url.URL
	headers    map[string]string
	httpClient *http.Client
}

// newHTTPProvider constructs a new httpProvider
func newHTTPProvider(opts *remoteOpts) (*httpProvider, error) {
	httpClientConfig := config.HTTPClientConfig{}
	if opts.HTTPClientConfig != nil {
		err := opts.HTTPClientConfig.Validate()
		if err != nil {
			return nil, err
		}
		httpClientConfig = *opts.HTTPClientConfig
	}
	httpClient, err := config.NewClientFromConfig(httpClientConfig, "remote-config")
	if err != nil {
		return nil, err
	}
	return &httpProvider{
		myURL:      opts.url,
		httpClient: httpClient,
		headers:    opts.headers,
	}, nil
}

type retryAfterError struct {
	retryAfter time.Duration
}

func (r retryAfterError) Error() string {
	return fmt.Sprintf("server indicated to retry after %s", r.retryAfter)
}

type notModifiedError struct{}

func (n notModifiedError) Error() string {
	return "server indicated no changes"
}

// retrieve implements remoteProvider and fetches the config
func (p httpProvider) retrieve() ([]byte, http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, p.myURL.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating request: %w", err)
	}
	for header, headerVal := range p.headers {
		req.Header.Set(header, headerVal)
	}
	response, err := p.httpClient.Do(req)
	if err != nil {
		instrumentation.InstrumentRemoteConfigFetchError()
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()

	instrumentation.InstrumentRemoteConfigFetch(response.StatusCode)

	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
		retryAfter := response.Header.Get("Retry-After")
		if retryAfter == "" {
			return nil, nil, fmt.Errorf("server indicated to retry, but no Retry-After header was provided")
		}
		retryAfterDuration, err := time.ParseDuration(retryAfter)
		if err != nil {
			return nil, nil, fmt.Errorf("server indicated to retry, but Retry-After header was not a valid duration: %w", err)
		}
		return nil, nil, retryAfterError{retryAfter: retryAfterDuration}
	}

	if response.StatusCode == http.StatusNotModified {
		return nil, nil, notModifiedError{}
	}

	if response.StatusCode/100 != 2 {
		return nil, nil, fmt.Errorf("error fetching config: status code: %d", response.StatusCode)
	}
	bb, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}
	return bb, response.Header, nil
}
