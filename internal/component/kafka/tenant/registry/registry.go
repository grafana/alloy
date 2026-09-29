// Package registry holds the static tenant → partition mapping shared by
// kafka.tenant_producer and kafka.tenant_consumer.
package registry

import (
	"errors"
	"fmt"

	"go.uber.org/atomic"
	"gopkg.in/yaml.v3"
)

// Registry maps each tenant to exactly one partition of a single topic.
// A Registry is immutable once parsed.
type Registry struct {
	topic      string
	partitions int32
	byTenant   map[string]int32
	byPart     map[int32]string
	retired    map[int32]struct{}
}

type file struct {
	Topic      string           `yaml:"topic"`
	Partitions int32            `yaml:"partitions"`
	Tenants    map[string]int32 `yaml:"tenants"`
	Retired    []int32          `yaml:"retired"`
}

// Parse parses and validates a tenant registry file.
func Parse(b []byte) (*Registry, error) {
	var f file
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parsing tenant registry: %w", err)
	}

	if f.Topic == "" {
		return nil, errors.New("tenant registry: topic must be set")
	}
	if f.Partitions <= 0 {
		return nil, errors.New("tenant registry: partitions must be greater than 0")
	}

	r := &Registry{
		topic:      f.Topic,
		partitions: f.Partitions,
		byTenant:   make(map[string]int32, len(f.Tenants)),
		byPart:     make(map[int32]string, len(f.Tenants)),
		retired:    make(map[int32]struct{}, len(f.Retired)),
	}

	for _, p := range f.Retired {
		if p < 0 || p >= f.Partitions {
			return nil, fmt.Errorf("tenant registry: retired partition %d out of range [0, %d)", p, f.Partitions)
		}
		r.retired[p] = struct{}{}
	}

	for tenant, p := range f.Tenants {
		if tenant == "" {
			return nil, errors.New("tenant registry: tenant IDs must not be empty")
		}
		if p < 0 || p >= f.Partitions {
			return nil, fmt.Errorf("tenant registry: tenant %q partition %d out of range [0, %d)", tenant, p, f.Partitions)
		}
		if other, ok := r.byPart[p]; ok {
			return nil, fmt.Errorf("tenant registry: partition %d assigned to both %q and %q", p, other, tenant)
		}
		if _, ok := r.retired[p]; ok {
			return nil, fmt.Errorf("tenant registry: tenant %q assigned to retired partition %d", tenant, p)
		}
		r.byTenant[tenant] = p
		r.byPart[p] = tenant
	}

	return r, nil
}

// Topic returns the topic all tenants are written to.
func (r *Registry) Topic() string { return r.topic }

// Partitions returns the number of partitions the topic is expected to have.
func (r *Registry) Partitions() int32 { return r.partitions }

// PartitionFor returns the partition assigned to tenant.
func (r *Registry) PartitionFor(tenant string) (int32, bool) {
	p, ok := r.byTenant[tenant]
	return p, ok
}

// TenantFor returns the tenant assigned to partition.
func (r *Registry) TenantFor(partition int32) (string, bool) {
	t, ok := r.byPart[partition]
	return t, ok
}

// MaxAssignedPartition returns the highest partition assigned to any tenant,
// or -1 if no tenants are assigned.
func (r *Registry) MaxAssignedPartition() int32 {
	maxPart := int32(-1)
	for p := range r.byPart {
		maxPart = max(maxPart, p)
	}
	return maxPart
}

// Holder stores the current Registry so it can be swapped lock-free on
// component updates.
type Holder struct {
	p atomic.Pointer[Registry]
}

// Load returns the current Registry. It may be nil if none was stored.
func (h *Holder) Load() *Registry { return h.p.Load() }

// Store replaces the current Registry.
func (h *Holder) Store(r *Registry) { h.p.Store(r) }
