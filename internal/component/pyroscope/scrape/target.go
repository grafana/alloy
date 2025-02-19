// Copyright 2022 The Parca Authors
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

package scrape

import (
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
)

// TargetHealth describes the health state of a target.
type TargetHealth string

// The possible health states of a target based on the last performed scrape.
const (
	HealthUnknown TargetHealth = "unknown"
	HealthGood    TargetHealth = "up"
	HealthBad     TargetHealth = "down"
)

// Target refers to a singular HTTP or HTTPS endpoint.
type Target struct {
	// All labels of this target - public and private
	allLabels labels.Labels
	// Additional URL parameters that are part of the target URL.
	params url.Values
	hash   uint64
	url    string

	mtx                sync.RWMutex
	lastError          error
	lastScrape         time.Time
	lastScrapeDuration time.Duration
	health             TargetHealth
}

// NewTarget creates a reasonably configured target for querying.
func NewTarget(lbls labels.Labels, params url.Values) *Target {
	var publicLabels labels.Labels
	// lbls are sorted. Private labels goes before public labels.
	// find pivot to calculate publicLabels as subslice, with no allocations
	for i, l := range lbls {
		if strings.HasPrefix(l.Name, model.ReservedLabelPrefix) {
			// the fact that godeltaprof was used scraping should not be user visible
			if l.Name == model.MetricNameLabel {
				switch l.Value {
				case pprofGoDeltaProfMemory:
					lbls[i].Value = pprofMemory
				case pprofGoDeltaProfBlock:
					lbls[i].Value = pprofBlock
				case pprofGoDeltaProfMutex:
					lbls[i].Value = pprofMutex
				}
			}
			continue
		}
		publicLabels = lbls[i:]
		break
	}
	url := urlFromTarget(lbls, params)

	h := fnv.New64a()
	_, _ = h.Write([]byte(strconv.FormatUint(publicLabels.Hash(), 16)))
	_, _ = h.Write([]byte(url))

	return &Target{
		allLabels: lbls,
		url:       url,
		hash:      h.Sum64(),
		params:    params,
		health:    HealthUnknown,
	}
}

func urlFromTarget(lbls labels.Labels, params url.Values) string {
	newParams := url.Values{}

	for k, v := range params {
		newParams[k] = make([]string, len(v))
		copy(newParams[k], v)
	}
	for _, l := range lbls {
		if !strings.HasPrefix(l.Name, model.ParamLabelPrefix) {
			continue
		}
		ks := l.Name[len(model.ParamLabelPrefix):]

		if len(newParams[ks]) > 0 {
			newParams[ks][0] = l.Value
		} else {
			newParams[ks] = []string{l.Value}
		}
	}

	return (&url.URL{
		Scheme:   lbls.Get(model.SchemeLabel),
		Host:     lbls.Get(model.AddressLabel),
		RawQuery: newParams.Encode(),
	}).JoinPath(lbls.Get(ProfilePathPrefix), lbls.Get(ProfilePath)).String()
}

func (t *Target) String() string {
	return t.allLabels.String()
}

// Hash returns an identifying hash for the target, based on public labels and the URL.
func (t *Target) Hash() uint64 {
	return t.hash
}

// offset returns the time until the next scrape cycle for the target.
func (t *Target) offset(interval time.Duration) time.Duration {
	now := time.Now().UnixNano()

	var (
		base   = now % int64(interval)
		offset = t.hash % uint64(interval)
		next   = base + int64(offset)
	)

	if next > int64(interval) {
		next -= int64(interval)
	}
	return time.Duration(next)
}

// Params returns a copy of the set of all public params of the target.
func (t *Target) Params() url.Values {
	q := make(url.Values, len(t.params))
	for k, values := range t.params {
		q[k] = make([]string, len(values))
		copy(q[k], values)
	}
	return q
}

// URL returns the target's URL as string.
func (t *Target) URL() string {
	return t.url
}

// LastError returns the error encountered during the last scrape.
func (t *Target) LastError() error {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	return t.lastError
}

// LastScrape returns the time of the last scrape.
func (t *Target) LastScrape() time.Time {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	return t.lastScrape
}

// LastScrapeDuration returns how long the last scrape of the target took.
func (t *Target) LastScrapeDuration() time.Duration {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	return t.lastScrapeDuration
}

// Health returns the last known health state of the target.
func (t *Target) Health() TargetHealth {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	return t.health
}

// labelsByProfiles returns builders for each profiling type enabled in config
func labelsByProfiles(base labels.Labels, c *ProfilingConfig) []*labels.Builder {
	var res []*labels.Builder

	add := func(profileType string, cfgs ...ProfilingTarget) {
		for _, p := range cfgs {
			if p.Enabled {
				lb := labels.NewBuilder(base)
				setIfNotPresentAndNotEmpty(lb, ProfilePath, p.Path)
				setIfNotPresentAndNotEmpty(lb, ProfileName, profileType)
				setIfNotPresentAndNotEmpty(lb, ProfilePathPrefix, c.PathPrefix)
				res = append(res, lb)
			}
		}
	}

	for profilingType, profilingConfig := range c.AllTargets() {
		add(profilingType, profilingConfig)
	}

	return res
}

// Targets is a sortable list of targets.
type Targets []*Target

func (ts Targets) Len() int           { return len(ts) }
func (ts Targets) Less(i, j int) bool { return ts[i].URL() < ts[j].URL() }
func (ts Targets) Swap(i, j int)      { ts[i], ts[j] = ts[j], ts[i] }

const (
	ProfilePath         = "__profile_path__"
	ProfilePathPrefix   = "__profile_path_prefix__"
	ProfileName         = "__name__"
	serviceNameLabel    = "service_name"
	serviceNameK8SLabel = "__meta_kubernetes_pod_annotation_pyroscope_io_service_name"
)

// populateLabels builds a label set from the given label set and scrape configuration.
func populateLabels(lb *labels.Builder, base labels.Labels, cfg Arguments) (res labels.Labels, err error) {
	scrapeLabels := []labels.Label{
		{Name: model.JobLabel, Value: cfg.JobName},
		{Name: model.SchemeLabel, Value: cfg.Scheme},
	}

	for _, l := range scrapeLabels {
		setIfNotPresentAndNotEmpty(lb, l.Name, l.Value)
	}
	// Encode scrape query parameters as labels.
	for k, v := range cfg.Params {
		if len(v) > 0 {
			lb.Set(model.ParamLabelPrefix+k, v[0])
		}
	}
	addr := lb.Get(model.AddressLabel)
	if addr == "" {
		return nil, errors.New("no address")
	}

	// addPort checks whether we should add a default port to the address.
	// If the address is not valid, we don't append a port either.
	addPort := func(s string) bool {
		// If we can split, a port exists and we don't have to add one.
		if _, _, err := net.SplitHostPort(s); err == nil {
			return false
		}
		// If adding a port makes it valid, the previous error
		// was not due to an invalid address and we can append a port.
		_, _, err := net.SplitHostPort(s + ":1234")
		return err == nil
	}

	// If it's an address with no trailing port, infer it based on the used scheme.
	if addPort(addr) {
		// Addresses reaching this point are already wrapped in [] if necessary.
		switch lb.Get(model.SchemeLabel) {
		case "http", "":
			addr = addr + ":80"
		case "https":
			addr = addr + ":443"
		default:
			return nil, fmt.Errorf("invalid scheme: %q", cfg.Scheme)
		}
		lb.Set(model.AddressLabel, addr)
	}

	if err := config.CheckTargetAddress(model.LabelValue(addr)); err != nil {
		return nil, err
	}

	// Meta labels are deleted after relabelling. Other internal labels propagate to
	// the target which decides whether they will be part of their label set.
	for _, l := range base {
		if strings.HasPrefix(l.Name, model.MetaLabelPrefix) {
			lb.Del(l.Name)
		}
	}

	// Default the instance label to the target address.
	if v := lb.Get(model.InstanceLabel); v == "" {
		lb.Set(model.InstanceLabel, addr)
	}

	if serviceName := lb.Get(serviceNameLabel); serviceName == "" {
		lb.Set(serviceNameLabel, inferServiceName(base))
	}

	res = lb.Labels()
	for _, l := range res {
		// Check label values are valid, drop the target if not.
		if !model.LabelValue(l.Value).IsValid() {
			return nil, fmt.Errorf("invalid label value for %q: %q", l.Name, l.Value)
		}
	}

	return res, nil
}

// targetsFromGroup builds targets based on the given TargetGroup, config and target types map.
func targetsFromGroup(group *targetgroup.Group, cfg Arguments, targetTypes map[string]ProfilingTarget) ([]*Target, error) {
	var (
		targets = make([]*Target, 0, len(group.Targets))
	)

	for i, tlset := range group.Targets {

		lbls := make(labels.Labels, 0, len(tlset)+len(group.Labels))

		for ln, lv := range tlset {
			lbls = append(lbls, labels.Label{Name: string(ln), Value: string(lv)})
		}
		for ln, lv := range group.Labels {
			if _, ok := tlset[ln]; !ok {
				lbls = append(lbls, labels.Label{Name: string(ln), Value: string(lv)})
			}
		}
		slices.SortFunc(lbls, func(a, b labels.Label) int { return strings.Compare(a.Name, b.Name) })

		lsets := labelsByProfiles(lbls, &cfg.ProfilingConfig)

		for _, lset := range lsets {
			lbls, err := populateLabels(lset, lbls, cfg)
			if err != nil {
				return nil, fmt.Errorf("instance %d in group %s: %s", i, group, err)
			}
			if lbls != nil {
				profType := lbls.Get(ProfileName)
				params := cfg.Params
				if params == nil {
					params = url.Values{}
				}

				if pcfg, found := targetTypes[profType]; found && pcfg.Delta {
					seconds := (cfg.ScrapeInterval)/time.Second - 1
					if cfg.DeltaProfilingDuration != defaultProfilingDuration {
						seconds = (cfg.DeltaProfilingDuration) / time.Second
					}
					params.Add("seconds", strconv.Itoa(int(seconds)))
				}
				targets = append(targets, NewTarget(lbls, params))
			}
		}
	}

	return targets, nil
}

func inferServiceName(lset labels.Labels) string {
	k8sServiceName := lset.Get(serviceNameK8SLabel)
	if k8sServiceName != "" {
		return k8sServiceName
	}
	k8sNamespace := lset.Get("__meta_kubernetes_namespace")
	k8sContainer := lset.Get("__meta_kubernetes_pod_container_name")
	if k8sNamespace != "" && k8sContainer != "" {
		return fmt.Sprintf("%s/%s", k8sNamespace, k8sContainer)
	}
	dockerContainer := lset.Get("__meta_docker_container_name")
	if dockerContainer != "" {
		return dockerContainer
	}
	if swarmService := lset.Get("__meta_dockerswarm_container_label_service_name"); swarmService != "" {
		return swarmService
	}
	if swarmService := lset.Get("__meta_dockerswarm_service_name"); swarmService != "" {
		return swarmService
	}
	return "unspecified"
}

func setIfNotPresentAndNotEmpty(b *labels.Builder, k, v string) {
	if b.Get(k) != "" {
		return
	}
	if v == "" {
		return
	}
	b.Set(k, v)
}
