package gather

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/confmap"
)

// maxMetricsSize caps the scraped metrics response.
const maxMetricsSize = 8 << 20 // 8 MiB

// metricsTruncationNotice marks a scrape that hit the size cap.
const metricsTruncationNotice = "\n# [support bundle: metrics truncated at size limit]\n"

// Metrics scrapes the collector's own telemetry metrics endpoint. It is an
// async gatherer: it scrapes once at the start of the window and, when the
// window is non-zero, again at the end. Two samples let an engineer compute
// counter deltas over the window. It reads the endpoint from the config
// snapshot, so it works only when the collector exposes a pull (Prometheus)
// metrics reader. It is best effort.
type Metrics struct {
	Conf   func() *confmap.Conf
	Client *http.Client
}

func (Metrics) Name() string { return "metrics" }

func (g Metrics) Start(ctx context.Context, opts Options) (FinishFunc, error) {
	endpoint := metricsEndpoint(g.Conf())
	if endpoint == "" {
		// No pull metrics endpoint is configured. Nothing to scrape.
		return nil, nil
	}
	if strings.Contains(endpoint, "${") {
		// The address still has an unresolved config reference. Skip it.
		return nil, nil
	}

	// Sample at the start of the window.
	startBody, startErr := scrapeMetrics(ctx, g.Client, endpoint)

	finish := func(fctx context.Context) ([]File, error) {
		// With no window, one sample is all there is.
		if opts.Duration <= 0 {
			if startErr != nil {
				return nil, startErr
			}
			return metricsFile("metrics.txt", startBody), nil
		}

		// With a window, keep the start sample and add an end sample.
		var files []File
		var errs []error
		if startErr != nil {
			errs = append(errs, fmt.Errorf("start: %w", startErr))
		} else {
			files = append(files, metricsFile("metrics-start.txt", startBody)...)
		}

		endBody, endErr := scrapeMetrics(fctx, g.Client, endpoint)
		if endErr != nil {
			errs = append(errs, fmt.Errorf("end: %w", endErr))
		} else {
			files = append(files, metricsFile("metrics-end.txt", endBody)...)
		}

		return files, errors.Join(errs...)
	}

	return finish, nil
}

// scrapeMetrics fetches the metrics endpoint. It caps and marks large bodies.
// The endpoint comes from the effective (expanded) config, so its errors must
// not include the address: the underlying transport error embeds it, and the
// error reaches the bundle's errors.txt. The messages here are address-free.
func scrapeMetrics(ctx context.Context, client *http.Client, endpoint string) ([]byte, error) {
	url := "http://" + endpoint + "/metrics"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New("build metrics request failed")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("metrics scrape request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics scrape returned status %d", resp.StatusCode)
	}

	// Read one byte past the cap to detect truncation.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetricsSize+1))
	if err != nil {
		return nil, errors.New("reading metrics response failed")
	}
	if len(body) > maxMetricsSize {
		body = append(body[:maxMetricsSize], metricsTruncationNotice...)
	}
	return body, nil
}

// metricsFile wraps a scraped body as a bundle file, or nothing when empty.
func metricsFile(path string, body []byte) []File {
	if len(body) == 0 {
		return nil
	}
	return []File{{Path: path, Content: body}}
}

// metricsEndpoint finds a scrapeable telemetry metrics endpoint in the config.
// It checks the legacy service::telemetry::metrics::address, then the pull
// Prometheus reader. It returns an empty string when it finds neither.
func metricsEndpoint(conf *confmap.Conf) string {
	if conf == nil {
		return ""
	}
	metrics, ok := dig(conf.ToStringMap(), "service", "telemetry", "metrics").(map[string]any)
	if !ok {
		return ""
	}

	if addr, ok := metrics["address"].(string); ok && addr != "" {
		return addr
	}

	readers, ok := metrics["readers"].([]any)
	if !ok {
		return ""
	}
	for _, r := range readers {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		prom, ok := dig(rm, "pull", "exporter", "prometheus").(map[string]any)
		if !ok {
			continue
		}
		host, _ := prom["host"].(string)
		if host == "" {
			host = "localhost"
		}
		// The config may already bracket an IPv6 host. Strip the brackets so
		// net.JoinHostPort does not double them.
		host = strings.Trim(host, "[]")
		if port := prom["port"]; port != nil {
			return net.JoinHostPort(host, fmt.Sprintf("%v", port))
		}
	}
	return ""
}

// dig walks nested map[string]any values by key. It returns nil if a key is
// missing or a value is not a map.
func dig(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		asMap, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = asMap[k]
	}
	return cur
}
