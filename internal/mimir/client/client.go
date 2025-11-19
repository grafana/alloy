package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/mimir/client/internal"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/dskit/instrument"
	"github.com/grafana/dskit/user"
	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
)

var (
	ErrUnrecoverable = errors.New("unrecoverable error response")
)

// IsRecoverable returns true for errors from API requests that can be retried, false otherwise.
func IsRecoverable(err error) bool {
	return !errors.Is(err, ErrUnrecoverable)
}

// Config is used to configure a MimirClient.
type Config struct {
	ID                   string
	Address              string
	UseLegacyRoutes      bool
	HTTPClientConfig     config.HTTPClientConfig
	PrometheusHTTPPrefix string
}

type RulerInterface interface {
	CreateRuleGroup(ctx context.Context, namespace string, rg MimirRuleGroup) error
	DeleteRuleGroup(ctx context.Context, namespace, groupName string) error
	ListRules(ctx context.Context, namespace string) (map[string][]MimirRuleGroup, error)
}

type AlertmanagerInterface interface {
	CreateAlertmanagerConfigs(ctx context.Context, conf *alertmgr_cfg.Config, templateFiles map[string]string) error
}

// MimirClient is a client to the Mimir API.
type MimirClient struct {
	id string

	endpoint *url.URL
	client   internal.Requester
	apiPath  string
	logger   log.Logger
}

// New returns a new MimirClient.
func New(logger log.Logger, cfg Config, timingHistogram *prometheus.HistogramVec) (*MimirClient, error) {
	endpoint, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, err
	}
	client, err := config.NewClientFromConfig(cfg.HTTPClientConfig, useragent.ProductName, config.WithHTTP2Disabled())
	if err != nil {
		return nil, err
	}

	path, err := url.JoinPath(cfg.PrometheusHTTPPrefix, "/config/v1/rules")
	if err != nil {
		return nil, err
	}
	if cfg.UseLegacyRoutes {
		path = "/api/v1/rules"
	}

	collector := instrument.NewHistogramCollector(timingHistogram)
	timedClient := internal.NewTimedClient(client, collector)

	return &MimirClient{
		id:       cfg.ID,
		endpoint: endpoint,
		client:   timedClient,
		apiPath:  path,
		logger:   logger,
	}, nil
}

func (r *MimirClient) doRequest(operation, path, method string, payload []byte) (*http.Response, error) {
	req, err := buildRequest(operation, path, method, *r.endpoint, payload)
	if err != nil {
		return nil, err
	}

	if r.id != "" {
		req.Header.Add(user.OrgIDHeaderName, r.id)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	if err := checkResponse(resp); err != nil {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("error %s %s: %w", method, path, err)
	}

	return resp, nil
}

// checkResponse checks an API response for errors.
func checkResponse(r *http.Response) error {
	if 200 <= r.StatusCode && r.StatusCode <= 299 {
		return nil
	}

	var msg, errMsg string
	scanner := bufio.NewScanner(io.LimitReader(r.Body, 512))
	if scanner.Scan() {
		msg = scanner.Text()
	}

	if msg == "" {
		errMsg = fmt.Sprintf("server returned HTTP status %s", r.Status)
	} else {
		errMsg = fmt.Sprintf("server returned HTTP status %s: %s", r.Status, msg)
	}

	if r.StatusCode/100 == 4 && r.StatusCode != http.StatusTooManyRequests {
		return fmt.Errorf("%w: %s", ErrUnrecoverable, errMsg)
	}

	return errors.New(errMsg)
}

func joinPath(baseURLPath, targetPath string) string {
	// trim exactly one slash at the end of the base URL, this expects target
	// path to always start with a slash
	return strings.TrimSuffix(baseURLPath, "/") + targetPath
}

func buildRequest(op, p, m string, endpoint url.URL, payload []byte) (*http.Request, error) {
	// parse path parameter again (as it already contains escaped path information
	pURL, err := url.Parse(p)
	if err != nil {
		return nil, err
	}

	// if path or endpoint contains escaping that requires RawPath to be populated, also join rawPath
	if pURL.RawPath != "" || endpoint.RawPath != "" {
		endpoint.RawPath = joinPath(endpoint.EscapedPath(), pURL.EscapedPath())
	}
	endpoint.Path = joinPath(endpoint.Path, pURL.Path)
	r, err := http.NewRequest(m, endpoint.String(), bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	r = r.WithContext(context.WithValue(r.Context(), internal.OperationNameContextKey, op))

	return r, nil
}
