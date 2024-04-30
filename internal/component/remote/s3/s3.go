package s3

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/grafana/alloy/internal/component"
	aws_common_config "github.com/grafana/alloy/internal/component/common/config/aws"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	component.Register(component.Registration{
		Name:      "remote.s3",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Component handles reading content from a file located in an Component-compatible system.
type Component struct {
	mut     sync.Mutex
	opts    component.Options
	args    Arguments
	health  component.Health
	content string

	watcher      *watcher
	updateChan   chan result
	s3Errors     prometheus.Counter
	lastAccessed prometheus.Gauge
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New initializes the S3 component.
func New(o component.Options, args Arguments) (*Component, error) {
	awsCfg, err := aws_common_config.GenerateAWSConfig(args.Options.Client)
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(*awsCfg, func(s3o *s3.Options) {
		s3o.UsePathStyle = args.Options.UsePathStyle
	})

	bucket, file := getPathBucketAndFile(args.Path)
	s := &Component{
		opts:       o,
		args:       args,
		health:     component.Health{},
		updateChan: make(chan result),
		s3Errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "remote_s3_errors_total",
			Help: "The number of errors while accessing s3",
		}),
		lastAccessed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_s3_timestamp_last_accessed_unix_seconds",
			Help: "The last successful access in unix seconds",
		}),
	}

	w := newWatcher(bucket, file, s.updateChan, args.PollFrequency, s3Client)
	s.watcher = w

	if err := o.Registerer.Register(s.s3Errors); err != nil {
		return nil, err
	}
	if err := o.Registerer.Register(s.lastAccessed); err != nil {
		return nil, err
	}

	content, err := w.downloadSynchronously()
	s.handleContentPolling(content, err)
	return s, nil
}

// Run activates the content handler and watcher.
func (s *Component) Run(ctx context.Context) error {
	go s.handleContentUpdate(ctx)
	go s.watcher.run(ctx)
	<-ctx.Done()

	return nil
}

// Update is called whenever the arguments have changed.
func (s *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	awsCfg, err := aws_common_config.GenerateAWSConfig(newArgs.Options.Client)
	if err != nil {
		return nil
	}
	s3Client := s3.NewFromConfig(*awsCfg, func(s3o *s3.Options) {
		s3o.UsePathStyle = newArgs.Options.UsePathStyle
	})

	bucket, file := getPathBucketAndFile(newArgs.Path)

	s.mut.Lock()
	defer s.mut.Unlock()
	s.args = newArgs
	s.watcher.updateValues(bucket, file, newArgs.PollFrequency, s3Client)

	return nil
}

// CurrentHealth returns the health of the component.
func (s *Component) CurrentHealth() component.Health {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.health
}

// handleContentUpdate reads from the update and error channels setting as appropriate
func (s *Component) handleContentUpdate(ctx context.Context) {
	for {
		select {
		case r := <-s.updateChan:
			// r.result will never be nil,
			s.handleContentPolling(string(r.result), r.err)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Component) handleContentPolling(newContent string, err error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	if err == nil {
		s.opts.OnStateChange(Exports{
			Content: alloytypes.OptionalSecret{
				IsSecret: s.args.IsSecret,
				Value:    newContent,
			},
		})
		s.lastAccessed.SetToCurrentTime()
		s.content = newContent
		s.health.Health = component.HealthTypeHealthy
		s.health.Message = "s3 file updated"
	} else {
		s.s3Errors.Inc()
		s.health.Health = component.HealthTypeUnhealthy
		s.health.Message = err.Error()
	}
	s.health.UpdateTime = time.Now()
}

// getPathBucketAndFile takes the path and splits it into a bucket and file.
func getPathBucketAndFile(path string) (bucket, file string) {
	parts := strings.Split(path, "/")
	file = strings.Join(parts[3:], "/")
	bucket = strings.Join(parts[:3], "/")
	bucket = strings.ReplaceAll(bucket, "s3://", "")
	return
}
