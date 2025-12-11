package parser

// This code is copied from Promtail (https://github.com/grafana/loki/commit/2e62abbf47c47041027baf240722b3d76e7bd9a3). The parser package is used to
// enable parsing entries from Azure Event Hubs  entries and forward them
// to other loki components.

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
)

type azureMonitorResourceLogs struct {
	Records []json.RawMessage `json:"records"`
}

// validate check if message contains records
func (l azureMonitorResourceLogs) validate() error {
	if len(l.Records) == 0 {
		return errors.New("records are empty")
	}

	return nil
}

// azureMonitorResourceLog used to unmarshal common schema for Azure resource logs
// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/resource-logs-schema
type azureMonitorResourceLog struct {
	Time string `json:"time"`
	// Some logs have `time` field, some have `timeStamp` field : https://github.com/grafana/loki/issues/14176
	TimeStamp     string `json:"timeStamp"`
	Category      string `json:"category"`
	ResourceID    string `json:"resourceId"`
	OperationName string `json:"operationName"`
}

// validate check if fields marked as required by schema for Azure resource log are not empty
func (l azureMonitorResourceLog) validate() error {
	valid := l.isTimeOrTimeStampFieldSet() &&
		len(l.Category) != 0 &&
		len(l.ResourceID) != 0 &&
		len(l.OperationName) != 0

	if !valid {
		return errors.New("required field or fields is empty")
	}

	return nil
}

func (l azureMonitorResourceLog) isTimeOrTimeStampFieldSet() bool {
	return len(l.Time) != 0 || len(l.TimeStamp) != 0
}

// getTime returns time from `time` or `timeStamp` field. If both fields are set, `time` is used. If both fields are empty, error is returned.
func (l azureMonitorResourceLog) getTime() (time.Time, error) {
	if len(l.Time) == 0 && len(l.TimeStamp) == 0 {
		var t time.Time
		return t, errors.New("time and timeStamp fields are empty")
	}

	if len(l.Time) != 0 {
		t, err := time.Parse(time.RFC3339, l.Time)
		if err != nil {
			return t, err
		}

		return t.UTC(), nil
	}

	t, err := time.Parse(time.RFC3339, l.TimeStamp)
	if err != nil {
		return t, err
	}

	return t.UTC(), nil
}

type AzureEventHubsTargetMessageParser struct {
	DisallowCustomMessages bool
}

func (e *AzureEventHubsTargetMessageParser) Parse(message *sarama.ConsumerMessage, labelSet model.LabelSet, relabels []*relabel.Config, useIncomingTimestamp bool) ([]loki.Entry, error) {
	messageTime := time.Now()
	if useIncomingTimestamp {
		messageTime = message.Timestamp
	}

	data, err := e.tryUnmarshal(message.Value)
	if err == nil {
		err = data.validate()
	}

	if err != nil {
		if e.DisallowCustomMessages {
			return []loki.Entry{}, err
		}

		return []loki.Entry{e.entryWithCustomPayload(message.Value, labelSet, messageTime)}, nil
	}

	return e.processRecords(labelSet, relabels, useIncomingTimestamp, data.Records, messageTime)
}

// tryUnmarshal tries to unmarshal raw message data, in case of error tries to fix it and unmarshal fixed data.
// If both attempts fail, return the initial unmarshal error.
func (e *AzureEventHubsTargetMessageParser) tryUnmarshal(message []byte) (*azureMonitorResourceLogs, error) {
	data := &azureMonitorResourceLogs{}
	err := json.Unmarshal(message, data)
	if err == nil {
		return data, nil
	}

	// try fix json as mentioned here:
	// https://learn.microsoft.com/en-us/answers/questions/1001797/invalid-json-logs-produced-for-function-apps?fbclid=IwAR3pK8Nj60GFBtKemqwfpiZyf3rerjowPH_j_qIuNrw_uLDesYvC4mTkfgs
	body := bytes.ReplaceAll(message, []byte(`'`), []byte(`"`))
	if json.Unmarshal(body, data) != nil {
		// return original error
		return nil, err
	}

	return data, nil
}

func (e *AzureEventHubsTargetMessageParser) entryWithCustomPayload(body []byte, labelSet model.LabelSet, messageTime time.Time) loki.Entry {
	return loki.Entry{
		Labels: labelSet,
		Entry: push.Entry{
			Timestamp: messageTime,
			Line:      string(body),
		},
	}
}

// processRecords handles the case when message is a valid json with a key `records`. It can be either a custom payload or a resource log.
func (e *AzureEventHubsTargetMessageParser) processRecords(labelSet model.LabelSet, relabels []*relabel.Config, useIncomingTimestamp bool, records []json.RawMessage, messageTime time.Time) ([]loki.Entry, error) {
	result := make([]loki.Entry, 0, len(records))
	for _, m := range records {
		entry, err := e.parseRecord(m, labelSet, relabels, useIncomingTimestamp, messageTime)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}

	return result, nil
}

// parseRecord parses a single value from the "records" in the original message.
// It can also handle a case when the record contains custom data and doesn't match the schema for Azure resource logs.
func (e *AzureEventHubsTargetMessageParser) parseRecord(record []byte, labelSet model.LabelSet, relabelConfig []*relabel.Config, useIncomingTimestamp bool, messageTime time.Time) (loki.Entry, error) {
	logRecord := &azureMonitorResourceLog{}
	err := json.Unmarshal(record, logRecord)
	if err == nil {
		err = logRecord.validate()
	}

	if err != nil {
		if e.DisallowCustomMessages {
			return loki.Entry{}, err
		}

		return e.entryWithCustomPayload(record, labelSet, messageTime), nil
	}

	logLabels := e.getLabels(logRecord, relabelConfig)
	ts := e.getTime(messageTime, useIncomingTimestamp, logRecord)

	return loki.Entry{
		Labels: labelSet.Merge(logLabels),
		Entry: push.Entry{
			Timestamp: ts,
			Line:      string(record),
		},
	}, nil
}

func (e *AzureEventHubsTargetMessageParser) getTime(messageTime time.Time, useIncomingTimestamp bool, logRecord *azureMonitorResourceLog) time.Time {
	if !useIncomingTimestamp || !logRecord.isTimeOrTimeStampFieldSet() {
		return messageTime
	}

	recordTime, err := logRecord.getTime()
	if err != nil {
		return messageTime
	}

	return recordTime
}

func (e *AzureEventHubsTargetMessageParser) getLabels(logRecord *azureMonitorResourceLog, relabelConfig []*relabel.Config) model.LabelSet {
	lbs := labels.New(labels.Label{
		Name:  "__azure_event_hubs_category",
		Value: logRecord.Category,
	})

	var processed labels.Labels
	// apply relabeling
	if len(relabelConfig) > 0 {
		processed, _ = relabel.Process(lbs, relabelConfig...)
	} else {
		processed = lbs
	}

	// final labelset that will be sent to loki
	resultLabels := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		// ignore internal labels
		if strings.HasPrefix(lbl.Name, "__") {
			return
		}
		// ignore invalid labels
		//nolint:staticcheck
		if !model.LabelName(lbl.Name).IsValid() || !model.LabelValue(lbl.Value).IsValid() {
			return
		}
		resultLabels[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	return resultLabels
}
