package metric

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const (
	CounterInc = "inc"
	CounterAdd = "add"

	ErrCounterActionRequired          = "counter action must be defined as either `inc` or `add`"
	ErrCounterInvalidAction           = "action %s is not valid, action must be either `inc` or `add`"
	ErrCounterInvalidMatchAll         = "`match_all: true` cannot be combined with `value`, please remove `match_all` or `value`"
	ErrCounterInvalidCountBytes       = "`count_entry_bytes: true` can only be set with `match_all: true`"
	ErrCounterInvalidCountBytesAction = "`count_entry_bytes: true` can only be used with `action: add`"
)

type CounterConfig struct {
	MatchAll   *bool   `mapstructure:"match_all"`
	CountBytes *bool   `mapstructure:"count_entry_bytes"`
	Value      *string `mapstructure:"value"`
	Action     string  `mapstructure:"action"`
}

func ParseCounterConfig(config any) (*CounterConfig, error) {
	cfg := &CounterConfig{}

	if err := mapstructure.Decode(config, cfg); err != nil {
		return nil, err
	}

	if err := validateCounterConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateCounterConfig(config *CounterConfig) error {
	if config.Action == "" {
		return errors.New(ErrCounterActionRequired)
	}
	config.Action = strings.ToLower(config.Action)
	if config.Action != CounterInc && config.Action != CounterAdd {
		return fmt.Errorf(ErrCounterInvalidAction, config.Action)
	}
	if config.MatchAll != nil && *config.MatchAll && config.Value != nil {
		return fmt.Errorf(ErrCounterInvalidMatchAll)
	}
	if config.CountBytes != nil && *config.CountBytes && (config.MatchAll == nil || !*config.MatchAll) {
		return errors.New(ErrCounterInvalidCountBytes)
	}
	if config.CountBytes != nil && *config.CountBytes && config.Action != CounterAdd {
		return errors.New(ErrCounterInvalidCountBytesAction)
	}
	return nil
}
