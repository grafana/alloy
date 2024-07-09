package internal

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/v3/pkg/logproto"
	yacepromutil "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/promutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	lokiClient "github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	gzipID1     = 0x1f
	gzipID2     = 0x8b
	gzipDeflate = 8

	successResponseTemplate = `{"requestId": "%s", "timestamp": %d}`
	errorResponseTemplate   = `{"requestId": "%s", "timestamp": %d, "errorMessage": "%s"}`

	millisecondsPerSecond = 1000

	commonAttributesHeader      = "X-Amz-Firehose-Common-Attributes"
	commonAttributesLabelPrefix = "lbl_"
)

// RecordOrigin is a type that tells from which origin the data received from AWS Firehose comes.
type RecordOrigin string

const (
	OriginCloudwatchLogs RecordOrigin = "cloudwatch-logs"
	OriginDirectPUT      RecordOrigin = "direct-put"
	OriginUnknown        RecordOrigin = "unknown"
)

// Sender is an interface that decouples the Firehose request handler from the destination where read loki entries
// should be written to.
type Sender interface {
	Send(ctx context.Context, entry loki.Entry)
}

// commonAttributes is a struct to Unmarshal value from the "X-Amz-Firehose-Common-Attributes" header.
// Specification of the value: https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#:~:text=HTTP%20Headers%20%2D%20X%2DAmz%2DFirehose%2DCommon%2DAttributes
type commonAttributes struct {
	CommonAttributes map[string]string `json:"commonAttributes"`
}

// Handler implements a http.Handler that is able to receive records from a Firehose HTTP destination.
type Handler struct {
	metrics       *Metrics
	logger        log.Logger
	sender        Sender
	relabelRules  []*relabel.Config
	useIncomingTs bool
	accessKey     string
}

// NewHandler creates a new handler.
func NewHandler(sender Sender, logger log.Logger, metrics *Metrics, rbs []*relabel.Config, useIncomingTs bool, accessKey string) *Handler {
	return &Handler{
		metrics:       metrics,
		logger:        logger,
		sender:        sender,
		relabelRules:  rbs,
		useIncomingTs: useIncomingTs,
		accessKey:     accessKey,
	}
}

// ServeHTTP satisfies the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error
	defer req.Body.Close()
	level.Info(h.logger).Log("msg", "handling request")

	// authenticate request if the component has an access key configured
	if len(h.accessKey) > 0 {
		apiHeader := req.Header.Get("X-Amz-Firehose-Access-Key")

		if subtle.ConstantTimeCompare([]byte(apiHeader), []byte(h.accessKey)) != 1 {
			http.Error(w, "access key not provided or incorrect", http.StatusUnauthorized)
			return
		}
	}

	var bodyReader io.Reader = req.Body
	// firehose allows the user to configure gzip content-encoding, in that case
	// decompress in the reader during unmarshalling
	if req.Header.Get("Content-Encoding") == "gzip" {
		bodyReader, err = gzip.NewReader(req.Body)
		if err != nil {
			h.metrics.errorsAPIRequest.WithLabelValues("pre_read").Inc()
			level.Error(h.logger).Log("msg", "failed to create gzip reader", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	firehoseReq := FirehoseRequest{}
	err = json.NewDecoder(bodyReader).Decode(&firehoseReq)
	if err != nil {
		h.metrics.errorsAPIRequest.WithLabelValues("read_or_format").Inc()
		level.Error(h.logger).Log("msg", "failed to unmarshall request", "err", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// common labels contains all record-wide labels
	commonLabels := labels.NewBuilder(nil)
	commonLabels.Set("__aws_firehose_request_id", req.Header.Get("X-Amz-Firehose-Request-Id"))
	commonLabels.Set("__aws_firehose_source_arn", req.Header.Get("X-Amz-Firehose-Source-Arn"))

	// if present, use the tenantID header
	tenantID := req.Header.Get("X-Scope-OrgID")
	if tenantID != "" {
		commonLabels.Set(lokiClient.ReservedLabelTenantID, tenantID)
	}

	requestStaticLabels := h.tryToGetStaticLabelsFromRequest(req, tenantID)
	for l, v := range requestStaticLabels {
		commonLabels.Set(string(l), string(v))
	}

	h.metrics.batchSize.WithLabelValues().Observe(float64(len(firehoseReq.Records)))

	for _, rec := range firehoseReq.Records {
		// cleanup err since it might have failed in the previous iteration
		err = nil

		decodedRecord, recordType, err := h.decodeRecord(rec.Data)
		if err != nil {
			h.metrics.errorsRecord.WithLabelValues(getReason(err)).Inc()
			level.Error(h.logger).Log("msg", "failed to decode request record", "err", err.Error())
			continue
		}

		ts := time.Now()
		if h.useIncomingTs {
			ts = time.Unix(firehoseReq.Timestamp/millisecondsPerSecond, 0)
		}

		h.metrics.recordsReceived.WithLabelValues(string(recordType)).Inc()

		switch recordType {
		case OriginDirectPUT:
			h.sender.Send(req.Context(), loki.Entry{
				Labels: h.postProcessLabels(commonLabels.Labels()),
				Entry: logproto.Entry{
					Timestamp: ts,
					Line:      string(decodedRecord),
				},
			})
		case OriginCloudwatchLogs:
			err = h.handleCloudwatchLogsRecord(req.Context(), decodedRecord, commonLabels.Labels(), ts)
		}
		if err != nil {
			h.metrics.errorsRecord.WithLabelValues(getReason(err)).Inc()
			level.Error(h.logger).Log("msg", "failed to handle cloudwatch record", "err", err.Error())
			continue
		}
	}

	sendAPIResponse(w, firehoseReq.RequestID, "", http.StatusOK)
}

// postProcessLabels applies relabels, then drops not relabeled internal and invalid labels.
func (h *Handler) postProcessLabels(lbs labels.Labels) model.LabelSet {
	// apply relabel rules if any
	if len(h.relabelRules) > 0 {
		lbs, _ = relabel.Process(lbs, h.relabelRules...)
	}

	entryLabels := make(model.LabelSet)
	for _, lbl := range lbs {
		// if internal label and not reserved, drop
		if strings.HasPrefix(lbl.Name, "__") && lbl.Name != lokiClient.ReservedLabelTenantID {
			continue
		}

		// ignore invalid labels
		if !model.LabelName(lbl.Name).IsValid() || !model.LabelValue(lbl.Value).IsValid() {
			continue
		}

		entryLabels[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	}
	return entryLabels
}

// sendAPIResponse responds to AWS Firehose API in the expected response format. To simplify error handling,
// it uses a string template instead of marshalling a struct.
func sendAPIResponse(w http.ResponseWriter, firehoseID, errMsg string, status int) {
	timestamp := time.Now().Unix()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if errMsg != "" {
		_, _ = w.Write([]byte(fmt.Sprintf(errorResponseTemplate, firehoseID, timestamp, errMsg)))
	} else {
		_, _ = w.Write([]byte(fmt.Sprintf(successResponseTemplate, firehoseID, timestamp)))
	}
}

// decodeRecord handled the decoding of the base-64 encoded records. It handles the special case of CloudWatch
// log records, which are always gzipped before base-64 encoded.
// See https://docs.aws.amazon.com/firehose/latest/dev/writing-with-cloudwatch-logs.html for details.
func (h *Handler) decodeRecord(rec string) ([]byte, RecordOrigin, error) {
	decodedRec, err := base64.StdEncoding.DecodeString(rec)
	if err != nil {
		return nil, OriginUnknown, errWithReason{
			err:    err,
			reason: "base64-decode",
		}
	}

	// Using the same header check as the gzip library, but inlining the check to avoid unnecessary boilerplate
	// code from creating the reader.
	//
	// https://github.com/golang/go/blob/master/src/compress/gzip/gunzip.go#L185
	if !(decodedRec[0] == gzipID1 && decodedRec[1] == gzipID2 && // the first two represent the 1f8b magic bytes
		decodedRec[2] == gzipDeflate) { // the third byte represents the gzip compression method DEFLATE
		// no gzip, return decoded data
		return decodedRec, OriginDirectPUT, nil
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(decodedRec))
	if err != nil {
		return nil, OriginCloudwatchLogs, fmt.Errorf("error creating gzip reader: %w", err)
	}
	defer gzipReader.Close()

	b := bytes.Buffer{}
	if _, err := io.Copy(bufio.NewWriter(&b), gzipReader); err != nil {
		return nil, OriginCloudwatchLogs, errWithReason{
			err:    err,
			reason: "gzip-deflate",
		}
	}

	return b.Bytes(), OriginCloudwatchLogs, nil
}

// handleCloudwatchLogsRecord explodes the cloudwatch logs record into each log message. Also, it adds all properties
// sent in the envelope as internal labels, available for relabel.
func (h *Handler) handleCloudwatchLogsRecord(ctx context.Context, data []byte, commonLabels labels.Labels, timestamp time.Time) error {
	cwRecord := CloudwatchLogsRecord{}
	if err := json.Unmarshal(data, &cwRecord); err != nil {
		return errWithReason{
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

	for _, event := range cwRecord.LogEvents {
		h.sender.Send(ctx, loki.Entry{
			Labels: h.postProcessLabels(cwLogsLabels.Labels()),
			Entry: logproto.Entry{
				Timestamp: timestamp,
				Line:      event.Message,
			},
		})
	}

	return nil
}

// tryToGetStaticLabelsFromRequest tries to extract static labels from the request header.
func (h *Handler) tryToGetStaticLabelsFromRequest(req *http.Request, tenantID string) model.LabelSet {
	var staticLabels model.LabelSet
	commonAttributesHeaderValue := req.Header.Get(commonAttributesHeader)
	if len(commonAttributesHeaderValue) == 0 {
		return staticLabels
	}

	ca := commonAttributes{
		CommonAttributes: make(map[string]string),
	}

	err := json.Unmarshal([]byte(commonAttributesHeaderValue), &ca)
	if err != nil {
		// log error, increase metric value and ignore the header values
		level.Debug(h.logger).Log(fmt.Sprintf("error decoding %s: %s", commonAttributesHeader, err.Error()))
		h.metrics.invalidStaticLabelsCount.WithLabelValues(reasonInvalidJsonFormat, tenantID).Inc()
		return nil
	}

	staticLabels = make(model.LabelSet)
	for name, value := range ca.CommonAttributes {
		// check if the value is suppose to be a label name
		if !strings.HasPrefix(name, commonAttributesLabelPrefix) {
			continue
		}

		// construct model.LabelName from the header value, if the raw data is not valid label name, try to fix it and use
		rawLabelName := strings.TrimPrefix(name, commonAttributesLabelPrefix)
		labelName := model.LabelName(rawLabelName)
		if !labelName.IsValid() {
			level.Debug(h.logger).Log(fmt.Sprintf("label name is not valid, trying to fix: %s", rawLabelName))

			// try to sanitize label name
			sanitizedLabelName := yacepromutil.PromString(rawLabelName)
			labelName = model.LabelName(sanitizedLabelName)
			if !labelName.IsValid() {
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
				h.metrics.invalidStaticLabelsCount.WithLabelValues(reasonInvalidLabelName, tenantID).Inc()
				level.Debug(h.logger).Log(fmt.Sprintf("label name is not valid, can not create a prom string from: %s", rawLabelName))
				continue
			}
		}

		labelValue := model.LabelValue(value)
		if !labelValue.IsValid() {
			h.metrics.invalidStaticLabelsCount.WithLabelValues(reasonInvalidLabelValue, tenantID).Inc()
			level.Debug(h.logger).Log(fmt.Sprintf("label %s has invalid value: %s", labelName, value))
			continue
		}

		staticLabels[labelName] = labelValue
	}

	return staticLabels
}
