package stages

import (
	"errors"
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	json "github.com/json-iterator/go"
	"github.com/prometheus/common/model"
)

const (
	DeleteOp string = "delete"
	UpdateOp string = "update"
)

type JsonFieldOperation struct {
	Operation string `alloy:"operation,attr"`
	Field     string `alloy:"field,attr"`
	Source    string `alloy:"source,attr,optional"`
}

type JsonFieldConfig struct {
	Operations    []JsonFieldOperation `alloy:"operations,attr"`
	DropMalformed bool                 `alloy:"drop_malformed,attr,optional"`
}

const (
	ErrEmptyJsonFieldStageConfig = "json_field stage config cannot be empty"
	ErrWrongOperationType        = "json_field invalid operation cannot be %s"
)

func validateJsonFieldConfig(c JsonFieldConfig) ([]JsonFieldOperation, error) {
	ret := []JsonFieldOperation{}
	if len(c.Operations) == 0 {
		return nil, errors.New(ErrEmptyJsonFieldStageConfig)
	}

	for _, op := range c.Operations {
		jsonFieldOp := JsonFieldOperation{
			Operation: op.Operation,
			Field:     op.Field,
			Source:    op.Source,
		}
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !model.LabelName(op.Field).IsValid() {
			return nil, fmt.Errorf(ErrInvalidLabelName, op.Field)
		}
		// If no label source was specified, use the key name
		if op.Source == "" {
			jsonFieldOp.Source = op.Field
		}
		// Check if a supported operation is set
		if op.Operation != UpdateOp && op.Operation != DeleteOp {
			return nil, fmt.Errorf(ErrWrongOperationType, op.Operation)
		}
		ret = append(ret, jsonFieldOp)
	}
	return ret, nil
}

type jsonFieldStage struct {
	cfg        *JsonFieldConfig
	Operations []JsonFieldOperation
	logger     log.Logger
}

func newJsonFieldStage(logger log.Logger, cfg JsonFieldConfig) (Stage, error) {
	ops, err := validateJsonFieldConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &jsonFieldStage{
		cfg:        &cfg,
		Operations: ops,
		logger:     log.With(logger, "component", "stage", "type", "json_field"),
	}, nil
}

func (j *jsonFieldStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			err := j.processEntry(e.Extracted, &e.Line)
			if err != nil && j.cfg.DropMalformed {
				continue
			}
			out <- e
		}
	}()
	return out
}

func (j *jsonFieldStage) processEntry(extracted map[string]interface{}, entry *string) error {

	if entry == nil {
		if Debug {
			level.Debug(j.logger).Log("msg", "cannot parse a nil entry")
		}
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*entry), &data); err != nil {
		if Debug {
			level.Debug(j.logger).Log("msg", "failed to unmarshal log line", "err", err)
		}
		return errors.New(ErrMalformedJSON)
	}

	for _, op := range j.Operations {
		switch op.Operation {
		case UpdateOp:
			if _, ok := extracted[op.Source]; !ok {
				if Debug {
					level.Debug(j.logger).Log("msg", "field does not exist in the set of extracted values", "field", op.Field)
				}
				continue
			}
			data[op.Field] = extracted[op.Source]
		case DeleteOp:
			delete(data, op.Field)
		}
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		if Debug {
			level.Debug(j.logger).Log("msg", "failed to marshal log line", "err", err)
		}
		return errors.New(ErrMalformedJSON)
	}

	newEntry := string(dataBytes)
	*entry = newEntry
	return nil
}

// Name implements Stage
func (j *jsonFieldStage) Name() string {
	return StageTypeJsonField
}

// Cleanup implements Stage.
func (*jsonFieldStage) Cleanup() {
	// no-op
}
