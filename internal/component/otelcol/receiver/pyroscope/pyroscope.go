// Package pyroscope provides an otelcol.receiver.pyroscope component.
package pyroscope

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"strings"
	"sync"

	"github.com/google/pprof/profile"
	pproftranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	alloypyroscope "github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfoclient"
	"github.com/grafana/alloy/internal/featuregate"
)

// ErrUnsupportedIngestFormat is returned when an opaque Pyroscope ingest
// request isn't a non-multipart pprof request.
var ErrUnsupportedIngestFormat = errors.New("unsupported Pyroscope ingest format")

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.pyroscope",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(o component.Options, a component.Arguments) (component.Component, error) {
			return New(o, a.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.pyroscope component.
type Arguments struct {
	// Output configures where to send received profiles. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

// Exports are the set of fields exposed by the otelcol.receiver.pyroscope component.
type Exports struct {
	Receiver alloypyroscope.Appendable `alloy:"receiver,attr"`
}

// Component is the otelcol.receiver.pyroscope component.
type Component struct {
	receiver *profilesReceiver
}

var _ component.Component = (*Component)(nil)

// New creates a new otelcol.receiver.pyroscope component.
func New(o component.Options, args Arguments) (*Component, error) {
	receiver := &profilesReceiver{}
	c := &Component{
		receiver: receiver,
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}

	o.OnStateChange(Exports{Receiver: receiver})
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	cfg := args.(Arguments)
	if cfg.Output == nil {
		return errors.New("output block must be provided")
	}

	hasProfilesConsumer := false
	for _, consumer := range cfg.Output.Profiles {
		if consumer != nil {
			hasProfilesConsumer = true
			break
		}
	}
	if !hasProfilesConsumer {
		return errors.New("output.profiles must contain at least one consumer")
	}

	c.receiver.setConsumer(fanoutconsumer.Profiles(cfg.Output.Profiles))
	return nil
}

var (
	_ alloypyroscope.Appendable = (*profilesReceiver)(nil)
	_ alloypyroscope.Appender   = (*profilesReceiver)(nil)
)

// profilesReceiver translates pprof profiles into OpenTelemetry profiles and
// forwards them to an OpenTelemetry profiles consumer.
type profilesReceiver struct {
	mut      sync.RWMutex
	consumer xconsumer.Profiles
}

func (r *profilesReceiver) setConsumer(consumer xconsumer.Profiles) {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.consumer = consumer
}

func (r *profilesReceiver) getConsumer() xconsumer.Profiles {
	r.mut.RLock()
	defer r.mut.RUnlock()
	return r.consumer
}

// Appender implements pyroscope.Appendable.
func (r *profilesReceiver) Appender() alloypyroscope.Appender {
	return r
}

// Append implements pyroscope.Appender.
func (r *profilesReceiver) Append(ctx context.Context, lbs labels.Labels, samples []*alloypyroscope.RawSample) error {
	profiles := pprofile.NewProfiles()
	for i, sample := range samples {
		if err := ctx.Err(); err != nil {
			return err
		}
		if sample == nil {
			return fmt.Errorf("convert sample %d: sample is nil", i)
		}

		converted, err := convertPprof(sample.RawProfile, lbs)
		if err != nil {
			if sample.ID == "" {
				return fmt.Errorf("convert sample %d: %w", i, err)
			}
			return fmt.Errorf(
				"convert sample %d (%q): %w",
				i,
				sample.ID,
				err,
			)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := converted.MergeTo(profiles); err != nil {
			return fmt.Errorf("merge sample %d: %w", i, err)
		}
	}

	if profiles.ResourceProfiles().Len() == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.getConsumer().ConsumeProfiles(ctx, profiles)
}

// AppendIngest implements pyroscope.Appender. Only non-multipart ingest
// requests that explicitly declare the pprof format are supported.
func (r *profilesReceiver) AppendIngest(ctx context.Context, incoming *alloypyroscope.IncomingProfile) error {
	if incoming == nil {
		return errors.New("ingested profile is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateRawPprofIngest(incoming); err != nil {
		return err
	}

	profiles, err := convertPprof(incoming.RawBody, incoming.Labels)
	if err != nil {
		return fmt.Errorf("convert ingested pprof: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.getConsumer().ConsumeProfiles(ctx, profiles)
}

func validateRawPprofIngest(incoming *alloypyroscope.IncomingProfile) error {
	if incoming.URL == nil {
		return fmt.Errorf("%w: URL is missing", ErrUnsupportedIngestFormat)
	}

	format := incoming.URL.Query().Get("format")
	if format != "pprof" {
		return unsupportedIngestFormatError(incoming, format, "format is not pprof")
	}

	for _, contentType := range incoming.ContentType {
		if contentType == "" {
			continue
		}

		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return unsupportedIngestFormatError(incoming, format, fmt.Sprintf("invalid content type: %v", err))
		}
		if mediaType == "multipart/form-data" {
			return unsupportedIngestFormatError(incoming, format, "multipart pprof is not supported")
		}
	}

	return nil
}

func unsupportedIngestFormatError(incoming *alloypyroscope.IncomingProfile, format, reason string) error {
	return fmt.Errorf(
		"%w: %s; format=%q content_type=%q path=%q",
		ErrUnsupportedIngestFormat,
		reason,
		format,
		incoming.ContentType,
		incoming.URL.EscapedPath(),
	)
}

// Upload implements debuginfo.Appender. OpenTelemetry profiles don't provide
// a transport for Pyroscope debug information.
func (*profilesReceiver) Upload(debuginfo.UploadJob) {}

// DebugInfoClients implements debuginfo.Appender. OpenTelemetry profiles don't
// provide a transport for Pyroscope debug information.
func (*profilesReceiver) DebugInfoClients() []*debuginfoclient.Client { return nil }

func convertPprof(raw []byte, lbs labels.Labels) (pprofile.Profiles, error) {
	parsed, err := profile.ParseData(raw)
	if err != nil {
		return pprofile.Profiles{}, fmt.Errorf("parse pprof profile: %w", err)
	}

	profiles, err := pproftranslator.ConvertPprofToProfiles(parsed)
	if err != nil {
		return pprofile.Profiles{}, fmt.Errorf("translate pprof profile: %w", err)
	}

	addLabels(*profiles, lbs)
	return *profiles, nil
}

func addLabels(profiles pprofile.Profiles, lbs labels.Labels) {
	scopeName := lbs.Get(alloypyroscope.LabelOtelScopeName)
	scopeVersion := lbs.Get(alloypyroscope.LabelOtelScopeVersion)

	for i := 0; i < profiles.ResourceProfiles().Len(); i++ {
		resourceProfiles := profiles.ResourceProfiles().At(i)
		resourceAttrs := resourceProfiles.Resource().Attributes()

		lbs.Range(func(label labels.Label) {
			switch label.Name {
			case alloypyroscope.LabelServiceName:
				if label.Value != "" {
					resourceAttrs.PutStr(string(semconv.ServiceNameKey), label.Value)
				}
			case alloypyroscope.LabelName:
				resourceAttrs.PutStr(label.Name, label.Value)
			case alloypyroscope.LabelOtelScopeName, alloypyroscope.LabelOtelScopeVersion:
				// These labels populate the OpenTelemetry instrumentation scope below.
			default:
				if !strings.HasPrefix(label.Name, model.ReservedLabelPrefix) {
					resourceAttrs.PutStr(label.Name, label.Value)
				}
			}
		})

		for j := 0; j < resourceProfiles.ScopeProfiles().Len(); j++ {
			scope := resourceProfiles.ScopeProfiles().At(j).Scope()
			scope.SetName(scopeName)
			scope.SetVersion(scopeVersion)
		}
	}
}
