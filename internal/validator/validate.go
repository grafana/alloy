package validator

import (
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
)

type state int

const (
	stateOK state = iota
	stateParseError
)

func Validate(sources map[string][]byte) error {
	_, err := alloy_runtime.ParseSources(sources)
	if err != nil {
		return err
	}

	return nil
}
