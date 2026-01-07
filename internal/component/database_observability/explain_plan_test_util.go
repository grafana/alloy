package database_observability

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logfmt/logfmt"
	"github.com/grafana/alloy/internal/component/common/loki"
)

// ExtractExplainPlanOutputFromLogMsg extracts the explain plan output from a log message. It is only used for testing by both mysql and postgres explain plan collectors.
func ExtractExplainPlanOutputFromLogMsg(lokiEntry loki.Entry) (ExplainPlanOutput, error) {
	var explainPlanOutput ExplainPlanOutput
	var explainPlanOutputString string
	decoder := logfmt.NewDecoder(strings.NewReader(lokiEntry.Line))
	for decoder.ScanRecord() {
		for decoder.ScanKeyval() {
			if string(decoder.Key()) == "explain_plan_output" {
				explainPlanOutputString = string(decoder.Value())
				break
			}
		}
	}
	if decoder.Err() != nil {
		return explainPlanOutput, fmt.Errorf("failed to decode logfmt: %v", decoder.Err())
	}
	base64Decoded, err := base64.StdEncoding.DecodeString(explainPlanOutputString)
	if err != nil {
		return explainPlanOutput, fmt.Errorf("failed to decode base64 explain plan output: %v", err)
	}
	if err := json.Unmarshal(base64Decoded, &explainPlanOutput); err != nil {
		return explainPlanOutput, fmt.Errorf("failed to unmarshal explain plan output: %v", err)
	}
	return explainPlanOutput, nil
}
