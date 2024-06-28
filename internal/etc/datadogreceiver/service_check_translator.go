// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type ServiceCheck struct {
	Check     string                       `json:"check"`
	HostName  string                       `json:"host_name"`
	Status    datadogV1.ServiceCheckStatus `json:"status"`
	Timestamp int64                        `json:"timestamp,omitempty"`
	Tags      []string                     `json:"tags,omitempty"`
}

func handleCheckRunPayload(req *http.Request) ([]ServiceCheck, error) {
	buf := getBuffer()
	defer putBuffer(buf)
	var services []ServiceCheck
	if _, err := io.Copy(buf, req.Body); err != nil {
		return services, err
	}
	if err := req.Body.Close(); err != nil {
		return services, err
	}
	if err := json.Unmarshal(buf.Bytes(), &services); err != nil {
		// TODO(alexg): add a zap fo the incoming buffer to help debugging?
		return services, err
	}

	return services, nil
}

// More information on Datadog service checks: https://docs.datadoghq.com/api/latest/service-checks/
func translateServices(services []ServiceCheck, mt *MetricsTranslator) pmetric.Metrics {
	bt := newBatcher()
	bt.Metrics = pmetric.NewMetrics()

	for _, service := range services {
		metricProperties := parseSeriesProperties("service_check", "service_check", service.Tags, service.HostName, mt.buildInfo.Version)
		metric, metricID := bt.Lookup(metricProperties) // TODO(alexg): proper name

		dps := metric.Gauge().DataPoints()
		dps.EnsureCapacity(1)

		dp := dps.AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(service.Timestamp * 1_000_000_000)) // OTel uses nanoseconds, while Datadog uses seconds
		metricProperties.dpAttrs.CopyTo(dp.Attributes())
		dp.SetIntValue(int64(service.Status))

		// TODO(alexg): Do this stream thing for service check metrics?
		stream := identity.OfStream(metricID, dp)
		ts, ok := mt.streamHasTimestamp(stream)
		if ok {
			dp.SetStartTimestamp(ts)
		}
		mt.updateLastTsForStream(stream, dp.Timestamp())
	}
	return bt.Metrics
}
