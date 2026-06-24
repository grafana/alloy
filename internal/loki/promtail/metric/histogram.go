package metric

import (
	"github.com/go-viper/mapstructure/v2"
)

type HistogramConfig struct {
	Value   *string   `mapstructure:"value"`
	Buckets []float64 `mapstructure:"buckets"`
}

func ParseHistogramConfig(config any) (*HistogramConfig, error) {
	cfg := &HistogramConfig{}
	if err := mapstructure.Decode(config, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
