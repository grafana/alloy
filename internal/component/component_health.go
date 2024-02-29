package component

import (
	"encoding"
	"fmt"
	"time"
)

// HealthComponent is an optional extension interface for Components which
// report health information.
//
// Health information is exposed to the end user for informational purposes and
// cannot be referened in a River expression.
type HealthComponent interface {
	Component

	// CurrentHealth returns the current Health status for the component.
	//
	// CurrentHealth may be overridden by the Flow controller if there is a
	// higher-level issue, such as a config file being invalid or a Component
	// shutting down unexpectedly.
	CurrentHealth() Health
}

// Health is the reported health state of a component. It can be encoded to
// River.
type Health struct {
	// The specific health value.
	Health HealthType `river:"state,attr"`

	// An optional message to describe the health; useful to say why a component
	// is unhealthy.
	Message string `river:"message,attr,optional"`

	// An optional time to indicate when the component last modified something
	// which updated its health.
	UpdateTime time.Time `river:"update_time,attr,optional"`
}

// HealthType holds the health value for a component.
type HealthType uint8

var (
	_ encoding.TextMarshaler   = HealthType(0)
	_ encoding.TextUnmarshaler = (*HealthType)(nil)
)

const (
	// HealthTypeUnknown is the initial health of components, set when they're
	// first created.
	HealthTypeUnknown HealthType = iota

	// HealthTypeHealthy represents a component which is working as expected.
	HealthTypeHealthy

	// HealthTypeUnhealthy represents a component which is not working as
	// expected.
	HealthTypeUnhealthy

	// HealthTypeExited represents a component which has stopped running.
	HealthTypeExited
)

// String returns the string representation of ht.
func (ht HealthType) String() string {
	switch ht {
	case HealthTypeHealthy:
		return "healthy"
	case HealthTypeUnhealthy:
		return "unhealthy"
	case HealthTypeExited:
		return "exited"
	default:
		return "unknown"
	}
}

// MarshalText implements encoding.TextMarshaler.
func (ht HealthType) MarshalText() (text []byte, err error) {
	return []byte(ht.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ht *HealthType) UnmarshalText(text []byte) error {
	switch string(text) {
	case "healthy":
		*ht = HealthTypeHealthy
	case "unhealthy":
		*ht = HealthTypeUnhealthy
	case "unknown":
		*ht = HealthTypeUnknown
	case "exited":
		*ht = HealthTypeExited
	default:
		return fmt.Errorf("invalid health type %q", string(text))
	}
	return nil
}

// LeastHealthy returns the Health from the provided arguments which is
// considered to be the least healthy.
//
// Health types are first prioritized by [HealthTypeExited], followed by
// [HealthTypeUnhealthy], [HealthTypeUnknown], and [HealthTypeHealthy].
//
// If multiple arguments have the same Health type, the Health with the most
// recent timestamp is returned.
//
// Finally, if multiple arguments have the same Health type and the same
// timestamp, the earlier argument is chosen.
func LeastHealthy(h Health, hh ...Health) Health {
	if len(hh) == 0 {
		return h
	}

	leastHealthy := h

	for _, compareHealth := range hh {
		switch {
		case healthPriority[compareHealth.Health] > healthPriority[leastHealthy.Health]:
			// Higher health precedence.
			leastHealthy = compareHealth
		case compareHealth.Health == leastHealthy.Health:
			// Same health precedence; check timestamp.
			if compareHealth.UpdateTime.After(leastHealthy.UpdateTime) {
				leastHealthy = compareHealth
			}
		}
	}

	return leastHealthy
}

// healthPriority maps a HealthType to its priority; higher numbers means "less
// healthy."
var healthPriority = [...]int{
	HealthTypeHealthy:   0,
	HealthTypeUnknown:   1,
	HealthTypeUnhealthy: 2,
	HealthTypeExited:    3,
}
