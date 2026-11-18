// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"context"
	"regexp"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// appendable translates Prometheus scraping diffs into OpenTelemetry format.
type appendable struct {
	sink                 consumer.Metrics
	useStartTimeMetric   bool
	useMetadata          bool
	trimSuffixes         bool
	startTimeMetricRegex *regexp.Regexp
	externalLabels       labels.Labels

	settings receiver.Settings
	obsrecv  *receiverhelper.ObsReport
}

// NewAppendable returns a storage.AppendableV2 instance that emits metrics to the sink.
func NewAppendable(
	sink consumer.Metrics,
	set receiver.Settings,
	useStartTimeMetric bool,
	startTimeMetricRegex *regexp.Regexp,
	useMetadata bool,
	externalLabels labels.Labels,
	trimSuffixes bool,
) (storage.AppendableV2, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverID: set.ID, Transport: transport, ReceiverCreateSettings: set})
	if err != nil {
		return nil, err
	}

	return &appendable{
		sink:                 sink,
		settings:             set,
		useStartTimeMetric:   useStartTimeMetric,
		useMetadata:          useMetadata,
		startTimeMetricRegex: startTimeMetricRegex,
		externalLabels:       externalLabels,
		obsrecv:              obsrecv,
		trimSuffixes:         trimSuffixes,
	}, nil
}

func (o *appendable) Appender(ctx context.Context) storage.Appender {
	return newTransaction(ctx, o.sink, o.externalLabels, o.settings, o.obsrecv, o.trimSuffixes, o.useMetadata)
}

// AppenderV2 satisfies the AppendableV2 interface.
//
// TODO(v2 migration step 5): migrate the otelcol.receiver.prometheus
// transaction to use AppenderV2 natively. It currently panics because
// nothing calls it in production yet; all active append paths still go
// through Appender (V1). See https://github.com/grafana/alloy/issues/6896
func (o *appendable) AppenderV2(_ context.Context) storage.AppenderV2 {
	panic("AppenderV2 not yet implemented for otelcol.receiver.prometheus")
}
