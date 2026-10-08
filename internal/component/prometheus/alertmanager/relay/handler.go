package relay

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/prometheus/alertmanager/api/v2/models"
	"github.com/prometheus/alertmanager/notify/webhook"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
)

const (
	failureConnection  = "connection"
	failureTLS         = "tls"
	failureTimeout     = "timeout"
	failureStatus4xx   = "status_4xx"
	failureStatus5xx   = "status_5xx"
	failureStatusOther = "status_other"
)

type outboundFailure struct {
	reason     string
	statusCode int
	err        error
}

func (e *outboundFailure) Error() string {
	return e.err.Error()
}

func (e *outboundFailure) Unwrap() error {
	return e.err
}

func (c *Component) handleWebhook(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	c.metrics.webhookRequests.Inc()
	c.metrics.activeRequests.Inc()
	defer func() {
		c.metrics.webhookRequestDuration.Observe(time.Since(started).Seconds())
		c.metrics.activeRequests.Dec()
	}()

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	state := c.getState()
	message, err := decodeWebhook(w, r, state.maxRequestBodySize)
	if err != nil {
		statusCode := http.StatusBadRequest
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			statusCode = http.StatusRequestEntityTooLarge
		}
		c.opts.Logger.Warn("rejected Alertmanager webhook", "err", err, "status_code", statusCode)
		http.Error(w, http.StatusText(statusCode), statusCode)
		return
	}
	if message.Data == nil || len(message.Alerts) == 0 {
		c.opts.Logger.Warn("rejected Alertmanager webhook without alerts")
		http.Error(w, "webhook must contain at least one alert", http.StatusBadRequest)
		return
	}

	c.metrics.receivedAlerts.Add(float64(len(message.Alerts)))
	alerts, err := convertAlerts(message.Alerts)
	if err != nil {
		c.metrics.failedAlerts.Add(float64(len(message.Alerts)))
		c.opts.Logger.Warn("rejected invalid alert data", "err", err)
		http.Error(w, "webhook contains invalid alert data", http.StatusBadRequest)
		return
	}

	if err := c.forward(r.Context(), state, alerts); err != nil {
		c.metrics.failedAlerts.Add(float64(len(alerts)))
		failure := err.(*outboundFailure)
		loggerWithFailure(c.opts.Logger, failure).Error("failed to forward alerts", "err", failure.err)

		statusCode := http.StatusBadGateway
		if failure.reason == failureTimeout {
			statusCode = http.StatusGatewayTimeout
		}
		http.Error(w, "failed to forward alerts", statusCode)
		return
	}

	c.metrics.forwardedAlerts.Add(float64(len(alerts)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func decodeWebhook(w http.ResponseWriter, r *http.Request, maxBodySize int64) (*webhook.Message, error) {
	body := http.MaxBytesReader(w, r.Body, maxBodySize)
	defer body.Close()

	decoder := json.NewDecoder(body)
	var message webhook.Message
	if err := decoder.Decode(&message); err != nil {
		return nil, fmt.Errorf("decoding webhook JSON: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}

	return &message, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("webhook body must contain exactly one JSON value")
		}
		return fmt.Errorf("decoding trailing webhook data: %w", err)
	}
	return nil
}

func convertAlerts(in template.Alerts) (models.PostableAlerts, error) {
	out := make(models.PostableAlerts, 0, len(in))
	for i, alert := range in {
		if len(alert.Labels) == 0 {
			return nil, fmt.Errorf("alert %d has no labels", i)
		}
		if err := validateLabelSet(alert.Labels); err != nil {
			return nil, fmt.Errorf("alert %d has invalid labels: %w", i, err)
		}
		if err := validateLabelSet(alert.Annotations); err != nil {
			return nil, fmt.Errorf("alert %d has invalid annotations: %w", i, err)
		}

		postable := &models.PostableAlert{
			Annotations: models.LabelSet(alert.Annotations),
			StartsAt:    strfmt.DateTime(alert.StartsAt),
			EndsAt:      strfmt.DateTime(alert.EndsAt),
			Alert: models.Alert{
				GeneratorURL: strfmt.URI(alert.GeneratorURL),
				Labels:       models.LabelSet(alert.Labels),
			},
		}
		if err := postable.Validate(strfmt.Default); err != nil {
			return nil, fmt.Errorf("alert %d is invalid: %w", i, err)
		}
		out = append(out, postable)
	}

	return out, nil
}

func validateLabelSet(labelSet template.KV) error {
	for name, value := range labelSet {
		if !model.UTF8Validation.IsValidLabelName(name) {
			return fmt.Errorf("invalid label name %q", name)
		}
		if !model.LabelValue(value).IsValid() {
			return fmt.Errorf("label %q has an invalid value", name)
		}
	}
	return nil
}

func (c *Component) forward(ctx context.Context, state forwardingState, alerts models.PostableAlerts) error {
	body, err := json.Marshal(alerts)
	if err != nil {
		return &outboundFailure{reason: failureStatusOther, err: fmt.Errorf("encoding alerts: %w", err)}
	}

	requestCtx, cancel := context.WithTimeout(ctx, state.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, state.endpointURL, bytes.NewReader(body))
	if err != nil {
		return &outboundFailure{reason: failureStatusOther, err: fmt.Errorf("creating request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")

	c.metrics.outboundRequests.Inc()
	started := time.Now()
	resp, err := state.client.Do(req)
	c.metrics.outboundRequestDuration.Observe(time.Since(started).Seconds())
	if err != nil {
		reason := classifyRequestError(err)
		c.metrics.outboundRequestFailures.WithLabelValues(reason).Inc()
		return &outboundFailure{reason: reason, err: err}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		reason := classifyStatusCode(resp.StatusCode)
		c.metrics.outboundRequestFailures.WithLabelValues(reason).Inc()
		return &outboundFailure{
			reason:     reason,
			statusCode: resp.StatusCode,
			err:        fmt.Errorf("destination returned HTTP status %d", resp.StatusCode),
		}
	}

	return nil
}

func classifyRequestError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return failureTimeout
	}
	var timeoutError interface{ Timeout() bool }
	if errors.As(err, &timeoutError) && timeoutError.Timeout() {
		return failureTimeout
	}

	var (
		unknownAuthority x509.UnknownAuthorityError
		hostnameError    x509.HostnameError
		certificateError x509.CertificateInvalidError
		verificationErr  *tls.CertificateVerificationError
		recordHeaderErr  tls.RecordHeaderError
	)
	if errors.As(err, &unknownAuthority) ||
		errors.As(err, &hostnameError) ||
		errors.As(err, &certificateError) ||
		errors.As(err, &verificationErr) ||
		errors.As(err, &recordHeaderErr) {

		return failureTLS
	}

	return failureConnection
}

func classifyStatusCode(statusCode int) string {
	switch {
	case statusCode >= 400 && statusCode < 500:
		return failureStatus4xx
	case statusCode >= 500 && statusCode < 600:
		return failureStatus5xx
	default:
		return failureStatusOther
	}
}
