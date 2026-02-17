package metric

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const (
	counterInc = "inc"
	counterAdd = "add"

	errCounterActionRequired          = "counter action must be defined as either `inc` or `add`"
	errCounterInvalidAction           = "action %s is not valid, action must be either `inc` or `add`"
	errCounterInvalidMatchAll         = "`match_all: true` cannot be combined with `value`, please remove `match_all` or `value`"
	errCounterInvalidCountBytes       = "`count_entry_bytes: true` can only be set with `match_all: true`"
	errCounterInvalidCountBytesAction = "`count_entry_bytes: true` can only be used with `action: add`"
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
		return errors.New(errCounterActionRequired)
	}
	config.Action = strings.ToLower(config.Action)
	if config.Action != counterInc && config.Action != counterAdd {
		return fmt.Errorf(errCounterInvalidAction, config.Action)
	}
	if config.MatchAll != nil && *config.MatchAll && config.Value != nil {
		return errors.New(errCounterInvalidMatchAll)
	}
	if config.CountBytes != nil && *config.CountBytes && (config.MatchAll == nil || !*config.MatchAll) {
		return errors.New(errCounterInvalidCountBytes)
	}
	if config.CountBytes != nil && *config.CountBytes && config.Action != counterAdd {
		return errors.New(errCounterInvalidCountBytesAction)
	}
	return nil
}
