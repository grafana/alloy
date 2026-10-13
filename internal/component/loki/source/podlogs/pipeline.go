package podlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/cespare/xxhash/v2"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/regexp"
	"github.com/grafana/alloy/internal/component/loki/process/stages"
	monitoringv1alpha2 "github.com/grafana/alloy/internal/component/loki/source/podlogs/internal/apis/monitoring/v1alpha2"
	"github.com/grafana/alloy/internal/featuregate"
)

// podLogsPipeline is the processing pipeline built from one PodLogs spec. It is
// shared by every tailer of that PodLogs, so its stages must be safe for
// concurrent use.
type podLogsPipeline struct {
	// id changes whenever the pipeline spec changes. Targets carry it so a
	// spec change restarts their tailers with the new pipeline.
	id       string
	consumer *stages.PipelineConsumer
}

// pipelineCache builds and reuses one podLogsPipeline per PodLogs object.
type pipelineCache struct {
	log          *slog.Logger
	registerer   prometheus.Registerer
	minStability featuregate.Stability
	out          chan<- loki.Entry

	pipelines map[client.ObjectKey]*podLogsPipeline
	// retired pipelines can still be in use by tailers until the next
	// successful SyncTargets, so they are only stopped by retain.
	retired []*podLogsPipeline
}

func newPipelineCache(l *slog.Logger, registerer prometheus.Registerer, minStability featuregate.Stability, out chan<- loki.Entry) *pipelineCache {
	return &pipelineCache{
		log:          l,
		registerer:   registerer,
		minStability: minStability,
		out:          out,

		pipelines: make(map[client.ObjectKey]*podLogsPipeline),
	}
}

// get returns the pipeline for podLogs, or nil when it has none. A pipeline
// whose spec did not change since the last call is returned as is.
func (pc *pipelineCache) get(podLogs *monitoringv1alpha2.PodLogs) (*podLogsPipeline, error) {
	key := client.ObjectKeyFromObject(podLogs)

	if podLogs.Spec.Pipeline == nil {
		pc.retire(key)
		return nil, nil
	}

	id, err := pipelineID(key, podLogs.Spec.Pipeline)
	if err != nil {
		return nil, err
	}
	if existing, ok := pc.pipelines[key]; ok && existing.id == id {
		return existing, nil
	}

	cfgs, err := convertPipeline(podLogs.Spec.Pipeline)
	if err != nil {
		return nil, err
	}
	consumer, err := stages.NewPipelineConsumer(pc.log.With("podlogs", key.String()), pc.registerer, pc.minStability, cfgs, channelConsumer{out: pc.out})
	if err != nil {
		return nil, err
	}

	pc.retire(key)
	p := &podLogsPipeline{id: id, consumer: consumer}
	pc.pipelines[key] = p
	return p, nil
}

// retain stops every retired pipeline and every pipeline whose key is not in
// keep. Callers must make sure no tailer uses those pipelines anymore.
func (pc *pipelineCache) retain(keep map[client.ObjectKey]struct{}) {
	for key := range pc.pipelines {
		if _, ok := keep[key]; !ok {
			pc.retire(key)
		}
	}
	for _, p := range pc.retired {
		p.consumer.Stop()
	}
	pc.retired = nil
}

func (pc *pipelineCache) retire(key client.ObjectKey) {
	if p, ok := pc.pipelines[key]; ok {
		pc.retired = append(pc.retired, p)
		delete(pc.pipelines, key)
	}
}

func pipelineID(key client.ObjectKey, spec *monitoringv1alpha2.PipelineSpec) (string, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("hashing pipeline: %w", err)
	}
	return key.String() + "/" + strconv.FormatUint(xxhash.Sum64(raw), 16), nil
}

// channelConsumer is the last step of a PodLogs pipeline. It forwards the
// entries that survived the stages to the component's shared entry channel.
type channelConsumer struct {
	out chan<- loki.Entry
}

func (c channelConsumer) Consume(ctx context.Context, batch loki.Batch) error {
	for _, stream := range batch.Streams() {
		for i, e := range stream.Entries {
			lset := stream.Labels
			// Entries of one stream share their labels, but every entry leaving the
			// channel must own its label set.
			if i > 0 {
				lset = lset.Clone()
			}
			entry := loki.NewEntryWithCreatedUnixMicro(lset, stream.Created(), e)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case c.out <- entry:
			}
		}
	}
	return nil
}

// convertPipeline maps the PodLogs pipeline spec onto loki.process stage
// configurations.
func convertPipeline(spec *monitoringv1alpha2.PipelineSpec) ([]stages.StageConfig, error) {
	res := make([]stages.StageConfig, 0, len(spec.Stages))

	for i, stage := range spec.Stages {
		var (
			cfg stages.StageConfig
			set int
		)

		if stage.Drop != nil {
			set++
			cfg.DropConfig = &stages.DropConfig{
				DropReason: stage.Drop.DropCounterReason,
				Source:     stage.Drop.Source,
				Value:      stage.Drop.Value,
				Separator:  stage.Drop.Separator,
				Expression: stage.Drop.Expression,
			}
		}
		if stage.Regex != nil {
			set++
			expr, err := regexp.Compile(stage.Regex.Expression)
			if err != nil {
				return nil, fmt.Errorf("stage %d: regex: %w", i, err)
			}
			cfg.RegexConfig = &stages.RegexConfig{
				Expression:       expr,
				Source:           stage.Regex.Source,
				LabelsFromGroups: stage.Regex.LabelsFromGroups,
			}
			if err := cfg.RegexConfig.Validate(); err != nil {
				return nil, fmt.Errorf("stage %d: regex: %w", i, err)
			}
		}
		if stage.Labels != nil {
			set++
			cfg.LabelsConfig = &stages.LabelsConfig{Values: optionalValues(stage.Labels.Values)}
			if err := cfg.LabelsConfig.Validate(); err != nil {
				return nil, fmt.Errorf("stage %d: labels: %w", i, err)
			}
		}
		if stage.StaticLabels != nil {
			set++
			cfg.StaticLabelsConfig = &stages.StaticLabelsConfig{Values: optionalValues(stage.StaticLabels.Values)}
			if err := cfg.StaticLabelsConfig.Validate(); err != nil {
				return nil, fmt.Errorf("stage %d: staticLabels: %w", i, err)
			}
		}

		if set != 1 {
			return nil, fmt.Errorf("stage %d: exactly one of drop, regex, labels, staticLabels must be set", i)
		}
		res = append(res, cfg)
	}

	return res, nil
}

// optionalValues converts a CRD map to the loki.process form, where a nil
// value means "no value given".
func optionalValues(in map[string]string) map[string]*string {
	res := make(map[string]*string, len(in))
	for k, v := range in {
		if v == "" {
			res[k] = nil
			continue
		}
		res[k] = &v
	}
	return res
}
