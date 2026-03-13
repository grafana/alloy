// Package sampling provides rate based sampling for use by
// components that need to include a fraction of items in a "sampled" set (e.g.
// loki.secretfilter for processing rate, loki.process.stages.sampling for drop rate).
package sampling

import (
	"fmt"
	"math/rand"
	"time"
)

// maxRandomNumber is the maximum value used for the sampling boundary (0x7fffffffffffffff).
const maxRandomNumber = ^(uint64(1) << 63)

// ValidateRate returns an error if rate is not in [0.0, 1.0].
func ValidateRate(rate float64) error {
	if rate < 0.0 || rate > 1.0 {
		return fmt.Errorf("rate must be between 0.0 and 1.0, received %f", rate)
	}
	return nil
}

// Sampler decides probabilistically whether an item should be included in the sample (ShouldSample returns true).
// Rate is the probability of inclusion; 0 = never, 1 = always, 0.5 = ~50%.
type Sampler struct {
	rate     float64
	boundary uint64
	source   rand.Source
}

// NewSampler returns a Sampler for the given rate. Rate must be in [0.0, 1.0];
// call ValidateRate first or the sampler behavior for out-of-range rate is undefined.
func NewSampler(rate float64) *Sampler {
	s := &Sampler{}
	s.Update(rate)
	return s
}

// Update updates the sampler for a new rate (e.g. on component config change).
// Rate must be in [0.0, 1.0]; call ValidateRate first or behavior is undefined.
func (s *Sampler) Update(rate float64) {
	s.rate = rate
	if rate > 0 && rate < 1 {
		s.boundary = uint64(float64(maxRandomNumber) * rate)
		s.source = rand.NewSource(time.Now().UnixNano())
	} else {
		s.boundary = 0
		s.source = nil
	}
}

// ShouldSample returns true with probability equal to the rate used to create or update the sampler.
// Rate 0 → always false; rate 1 → always true; otherwise uses the same probabilistic algorithm as Jaeger's ProbabilisticSampler.
func (s *Sampler) ShouldSample() bool {
	if s.rate >= 1.0 {
		return true
	}
	if s.rate <= 0.0 {
		return false
	}
	return s.boundary >= s.randomID()&maxRandomNumber
}

// randomID returns a random uint64 in [1, maxRandomNumber] for sampling.
func (s *Sampler) randomID() uint64 {
	if s.source == nil {
		return maxRandomNumber
	}
	val := uint64(s.source.Int63())
	for val == 0 {
		val = uint64(s.source.Int63())
	}
	return val
}
