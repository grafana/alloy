// Copyright (c) 2018 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package strategy_store

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/atomic"
	"go.uber.org/zap"

	ss "github.com/jaegertracing/jaeger/cmd/collector/app/sampling/samplingstrategy"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
)

type strategyStore struct {
	storedStrategies atomic.Value // holds *storedStrategies
	logger           *zap.Logger
}

type storedStrategies struct {
	defaultStrategy   *api_v2.SamplingStrategyResponse
	serviceStrategies map[string]*api_v2.SamplingStrategyResponse
}

type strategyLoader func() ([]byte, error)

// NewStrategyStore creates a strategy store that holds static sampling strategies.
func NewStrategyStore(strats string, logger *zap.Logger) (ss.Provider, error) {
	h := &strategyStore{
		logger: logger,
	}
	h.storedStrategies.Store(defaultStrategies())

	loadFn := func() ([]byte, error) {
		return []byte(strats), nil
	}

	strategies, err := loadStrategies(loadFn)
	if err != nil {
		return nil, err
	}
	h.parseStrategies(strategies)

	return h, nil
}

// GetSamplingStrategy implements StrategyStore#GetSamplingStrategy.
func (h *strategyStore) GetSamplingStrategy(_ context.Context, serviceName string) (*api_v2.SamplingStrategyResponse, error) {
	ss := h.storedStrategies.Load().(*storedStrategies)
	serviceStrategies := ss.serviceStrategies
	if strategy, ok := serviceStrategies[serviceName]; ok {
		return strategy, nil
	}
	return ss.defaultStrategy, nil
}

// Close stops updating the strategies
func (h *strategyStore) Close() error {
	return nil
}

// TODO good candidate for a global util function
func loadStrategies(loadFn strategyLoader) (*strategies, error) {
	strategyBytes, err := loadFn()
	if err != nil {
		return nil, err
	}

	var strategies *strategies
	if err := json.Unmarshal(strategyBytes, &strategies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal strategies: %w", err)
	}
	return strategies, nil
}

func (h *strategyStore) parseStrategies(strategies *strategies) {
	if strategies == nil {
		h.logger.Info("No sampling strategies provided or URL is unavailable, using defaults")
		return
	}
	newStore := defaultStrategies()
	if strategies.DefaultStrategy != nil {
		newStore.defaultStrategy = h.parseServiceStrategies(strategies.DefaultStrategy)
	}

	merge := true
	if newStore.defaultStrategy.OperationSampling == nil ||
		newStore.defaultStrategy.OperationSampling.PerOperationStrategies == nil {

		merge = false
	}

	for _, s := range strategies.ServiceStrategies {
		newStore.serviceStrategies[s.Service] = h.parseServiceStrategies(s)

		// Merge with the default operation strategies, because only merging with
		// the default strategy has no effect on service strategies (the default strategy
		// is not merged with and only used as a fallback).
		opS := newStore.serviceStrategies[s.Service].OperationSampling
		if opS == nil {
			if newStore.defaultStrategy.OperationSampling == nil ||
				newStore.serviceStrategies[s.Service].ProbabilisticSampling == nil {

				continue
			}
			// Service has no per-operation strategies, so just reference the default settings and change default samplingRate.
			newOpS := *newStore.defaultStrategy.OperationSampling
			newOpS.DefaultSamplingProbability = newStore.serviceStrategies[s.Service].ProbabilisticSampling.SamplingRate
			newStore.serviceStrategies[s.Service].OperationSampling = &newOpS
			continue
		}
		if merge {
			opS.PerOperationStrategies = mergePerOperationSamplingStrategies(
				opS.PerOperationStrategies,
				newStore.defaultStrategy.OperationSampling.PerOperationStrategies)
		}
	}
	h.storedStrategies.Store(newStore)
}

// mergePerOperationStrategies merges two operation strategies a and b, where a takes precedence over b.
func mergePerOperationSamplingStrategies(
	a, b []*api_v2.OperationSamplingStrategy,
) []*api_v2.OperationSamplingStrategy {

	m := make(map[string]bool)
	for _, aOp := range a {
		m[aOp.Operation] = true
	}
	for _, bOp := range b {
		if m[bOp.Operation] {
			continue
		}
		a = append(a, bOp)
	}
	return a
}

func (h *strategyStore) parseServiceStrategies(strategy *serviceStrategy) *api_v2.SamplingStrategyResponse {
	resp := h.parseStrategy(&strategy.strategy)
	if len(strategy.OperationStrategies) == 0 {
		return resp
	}
	opS := &api_v2.PerOperationSamplingStrategies{
		DefaultSamplingProbability: defaultSamplingProbability,
	}
	if resp.StrategyType == api_v2.SamplingStrategyType_PROBABILISTIC {
		opS.DefaultSamplingProbability = resp.ProbabilisticSampling.SamplingRate
	}
	for _, operationStrategy := range strategy.OperationStrategies {
		s, ok := h.parseOperationStrategy(operationStrategy, opS)
		if !ok {
			continue
		}

		opS.PerOperationStrategies = append(opS.PerOperationStrategies,
			&api_v2.OperationSamplingStrategy{
				Operation:             operationStrategy.Operation,
				ProbabilisticSampling: s.ProbabilisticSampling,
			})
	}
	resp.OperationSampling = opS
	return resp
}

func (h *strategyStore) parseOperationStrategy(
	strategy *operationStrategy,
	parent *api_v2.PerOperationSamplingStrategies,
) (s *api_v2.SamplingStrategyResponse, ok bool) {

	s = h.parseStrategy(&strategy.strategy)
	if s.StrategyType == api_v2.SamplingStrategyType_RATE_LIMITING {
		// TODO OperationSamplingStrategy only supports probabilistic sampling
		h.logger.Warn(
			fmt.Sprintf(
				"Operation strategies only supports probabilistic sampling at the moment,"+
					"'%s' defaulting to probabilistic sampling with probability %f",
				strategy.Operation, parent.DefaultSamplingProbability),
			zap.Any("strategy", strategy))
		return nil, false
	}
	return s, true
}

func (h *strategyStore) parseStrategy(strategy *strategy) *api_v2.SamplingStrategyResponse {
	switch strategy.Type {
	case samplerTypeProbabilistic:
		return &api_v2.SamplingStrategyResponse{
			StrategyType: api_v2.SamplingStrategyType_PROBABILISTIC,
			ProbabilisticSampling: &api_v2.ProbabilisticSamplingStrategy{
				SamplingRate: strategy.Param,
			},
		}
	case samplerTypeRateLimiting:
		return &api_v2.SamplingStrategyResponse{
			StrategyType: api_v2.SamplingStrategyType_RATE_LIMITING,
			RateLimitingSampling: &api_v2.RateLimitingSamplingStrategy{
				MaxTracesPerSecond: int32(strategy.Param),
			},
		}
	default:
		h.logger.Warn("Failed to parse sampling strategy", zap.Any("strategy", strategy))
		return defaultStrategyResponse()
	}
}
