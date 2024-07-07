package types

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type MaxTimestamp struct {
	mtx   sync.Mutex
	value float64
	prometheus.Gauge
}

func (m *MaxTimestamp) Set(value float64) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if value > m.value {
		m.value = value
		m.Gauge.Set(value)
	}
}

func (m *MaxTimestamp) Get() float64 {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.value
}

func (m *MaxTimestamp) Collect(c chan<- prometheus.Metric) {
	if m.Get() > 0 {
		m.Gauge.Collect(c)
	}
}
