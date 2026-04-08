package aws_firehose

import (
	"compress/gzip"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	yacepromutil "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	lokiClient "github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/loki/pkg/push"
)

type firehoseRoute struct {
	metrics   *metrics
	accessKey string
}

func newRoutes(metrics *metrics, accessKey string) []source.LogsRoute {
	return []source.LogsRoute{
		&firehoseRoute{
			metrics:   metrics,
			accessKey: accessKey,
		},
	}
}

const (
	pathPush = "/awsfirehose/api/v1/push"

	millisecondsPerSecond = 1000
)

func (r *firehoseRoute) Path() string {
	return pathPush
}

func (r *firehoseRoute) Method() string {
	return http.MethodPost
}

func (r *firehoseRoute) Logs(req *http.Request, cfg *source.LogsConfig) ([]loki.Entry, int, error) {
	defer req.Body.Close()

	if len(r.accessKey) > 0 {
		apiHeader := req.Header.Get("X-Amz-Firehose-Access-Key")
		if subtle.ConstantTimeCompare([]byte(apiHeader), []byte(r.accessKey)) != 1 {
			return nil, http.StatusUnauthorized, fmt.Errorf("access key not provided or incorrect")
		}
	}

	var bodyReader io.ReadCloser = req.Body
	// firehose allows the user to configure gzip content-encoding, in that case
	// decompress in the reader during unmarshalling
	if req.Header.Get("Content-Encoding") == "gzip" {
		var err error
		bodyReader, err = gzip.NewReader(req.Body)
		if err != nil {
			r.metrics.IncRequestError("pre_read")
			return nil, http.StatusBadRequest, err
		}
	}
	defer bodyReader.Close()

	var firehoseReq firehoseRequest
	if err := json.NewDecoder(bodyReader).Decode(&firehoseReq); err != nil {
		r.metrics.IncRequestError("read_or_format")
		return nil, http.StatusBadRequest, err
	}

	commonLabels := labels.NewBuilder(labels.EmptyLabels())
	commonLabels.Set("__aws_firehose_request_id", req.Header.Get("X-Amz-Firehose-Request-Id"))
	commonLabels.Set("__aws_firehose_source_arn", req.Header.Get("X-Amz-Firehose-Source-Arn"))

	tenantID := req.Header.Get("X-Scope-OrgID")
	if tenantID != "" {
		commonLabels.Set(lokiClient.ReservedLabelTenantID, tenantID)
	}

	for l, v := range r.tryToGetStaticLabelsFromRequest(req, tenantID) {
		commonLabels.Set(string(l), string(v))
	}

	r.metrics.ObserveBatchSize(float64(len(firehoseReq.Records)))

	created := time.Now()
	entries := make([]loki.Entry, 0, len(firehoseReq.Records))
	for _, rec := range firehoseReq.Records {
		decodedRecord, recordType, err := decodeRecord(rec.Data)
		if err != nil {
			r.metrics.IncRecordError(getReason(err))
			continue
		}

		ts := time.Now()
		if cfg.UseIncomingTimestamp {
			ts = time.Unix(firehoseReq.Timestamp/millisecondsPerSecond, 0)
		}

		r.metrics.IncRecordsReceived(string(recordType))

		switch recordType {
		case originDirectPut:
			lset := postProcessLabels(commonLabels.Labels(), cfg)
			entries = append(entries, loki.NewEntryWithCreated(lset, created, push.Entry{
				Timestamp: ts,
				Line:      string(decodedRecord),
			}))
		case originCloudwatchLogs:
			recordEntries, err := r.handleCloudwatchLogsRecord(decodedRecord, commonLabels.Labels(), ts, created, cfg)
			if err != nil {
				r.metrics.IncRecordError(getReason(err))
				continue
			}
			entries = append(entries, recordEntries...)
		}
	}

	return entries, http.StatusOK, nil
}

func (r *firehoseRoute) WriteResponse(w http.ResponseWriter, req *http.Request, status int, err error) {
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
	}

	res := firehoseResponse{
		RequestID:    req.Header.Get("X-Amz-Firehose-Request-Id"),
		Timestamp:    time.Now().Unix(),
		ErrorMessage: errorMsg,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(&res)
}

func postProcessLabels(lbs labels.Labels, cfg *source.LogsConfig) model.LabelSet {
	if len(cfg.RelabelRules) > 0 {
		lbs, _ = relabel.Process(lbs, cfg.RelabelRules...)
	}

	entryLabels := make(model.LabelSet)
	lbs.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") && lbl.Name != lokiClient.ReservedLabelTenantID {
			return
		}

		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(lbl.Name).IsValidLegacy() || !model.LabelValue(lbl.Value).IsValid() {
			return
		}

		entryLabels[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	return entryLabels
}

func (r *firehoseRoute) handleCloudwatchLogsRecord(data []byte, commonLabels labels.Labels, timestamp, created time.Time, cfg *source.LogsConfig) ([]loki.Entry, error) {
	var cwRecord cwLogsRecord
	if err := json.Unmarshal(data, &cwRecord); err != nil {
		return nil, errWithReason{
			err:    err,
			reason: "cw-json-decode",
		}
	}

	cwLogsLabels := labels.NewBuilder(commonLabels)
	cwLogsLabels.Set("__aws_owner", cwRecord.Owner)
	cwLogsLabels.Set("__aws_cw_log_group", cwRecord.LogGroup)
	cwLogsLabels.Set("__aws_cw_log_stream", cwRecord.LogStream)
	cwLogsLabels.Set("__aws_cw_matched_filters", strings.Join(cwRecord.SubscriptionFilters, ","))
	cwLogsLabels.Set("__aws_cw_msg_type", cwRecord.MessageType)

	entries := make([]loki.Entry, 0, len(cwRecord.LogEvents))
	for _, event := range cwRecord.LogEvents {
		eventTimestamp := timestamp
		if cfg.UseIncomingTimestamp {
			eventTimestamp = time.UnixMilli(event.Timestamp)
		}

		lset := postProcessLabels(cwLogsLabels.Labels(), cfg)
		entries = append(entries, loki.NewEntryWithCreated(lset, created, push.Entry{
			Timestamp: eventTimestamp,
			Line:      event.Message,
		}))
	}

	return entries, nil
}

type commonAttributes struct {
	CommonAttributes map[string]string `json:"commonAttributes"`
}

const (
	commonAttributesHeader      = "X-Amz-Firehose-Common-Attributes"
	commonAttributesLabelPrefix = "lbl_"
)

func (r *firehoseRoute) tryToGetStaticLabelsFromRequest(req *http.Request, tenantID string) model.LabelSet {
	var staticLabels model.LabelSet
	commonAttributesHeaderValue := req.Header.Get(commonAttributesHeader)
	if len(commonAttributesHeaderValue) == 0 {
		return staticLabels
	}

	ca := commonAttributes{
		CommonAttributes: make(map[string]string),
	}

	if err := json.Unmarshal([]byte(commonAttributesHeaderValue), &ca); err != nil {
		r.metrics.IncInvalidStaticLabels("invalid_json_format", tenantID)
		return nil
	}

	staticLabels = make(model.LabelSet)
	for name, value := range ca.CommonAttributes {
		if !strings.HasPrefix(name, commonAttributesLabelPrefix) {
			continue
		}

		rawLabelName := strings.TrimPrefix(name, commonAttributesLabelPrefix)
		labelName := model.LabelName(rawLabelName)
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !labelName.IsValidLegacy() {
			// try to sanitize label name
			sanitizedLabelName := yacepromutil.PromString(rawLabelName)
			labelName = model.LabelName(sanitizedLabelName)

			// TODO: add support for different validation schemes.
			//nolint:staticcheck
			if !labelName.IsValidLegacy() {
				// This situation can happen when:
				// - the header with label information is a valid JSON
				// - the label name is not valid and can not be sanitized
				//
				// For example:
				// {
				//  "commonAttributes": {
				//   "lbl_0mylabel": "value"
				//  }
				// }
				r.metrics.IncInvalidStaticLabels("invalid_label_name", tenantID)
				continue
			}
		}

		labelValue := model.LabelValue(value)
		if !labelValue.IsValid() {
			r.metrics.IncInvalidStaticLabels("invalid_label_value", tenantID)
			continue
		}

		staticLabels[labelName] = labelValue
	}

	return staticLabels
}

type errWithReason struct {
	err    error
	reason string
}

func (e errWithReason) Error() string {
	return fmt.Sprintf("%s: %s", e.reason, e.err.Error())
}

func getReason(err error) string {
	er, ok := err.(errWithReason)
	if ok {
		return er.reason
	}
	return "unknown"
}
