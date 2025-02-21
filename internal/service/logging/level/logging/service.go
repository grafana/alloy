// This service provides alloy log management for the application and components.
package logging

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/slogadapter"
)

// ServiceName defines the name used for the logging service.
const ServiceName = "logging"

type Service struct {
	logger *Logger

	mut  sync.Mutex
	args Options
}

var _ service.Service = (*Service)(nil)

func NewService() (*Service, error) {
	// Buffer logs until log format has been determined
	l, err := NewDeferred(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}

	return &Service{
		logger: l,
	}, nil
}

// Data implements service.Service.
// It returns the logger for the components to register to.
func (s *Service) Data() any {
	return s.logger
}

// Definition implements service.Service.
func (*Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: Options{},
		DependsOn:  []string{},
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

// Run implements service.Service.
func (s *Service) Run(ctx context.Context, _ service.Host) error {
	<-ctx.Done()
	return nil
}

// Update implements service.Service.
func (s *Service) Update(args any) error {
	newArgs := args.(Options)

	s.mut.Lock()
	defer s.mut.Unlock()
	s.args = newArgs

	switch newArgs.Format {
	case FormatLogfmt, FormatJSON:
		s.logger.hasLogFormat = true
	default:
		return fmt.Errorf("unrecognized log format %q", newArgs.Format)
	}

	s.logger.level.Set(slogLevel(newArgs.Level).Level())
	s.logger.format.Set(newArgs.Format)

	s.logger.writer.SetInnerWriter(s.logger.inner)
	if len(newArgs.WriteTo) > 0 {
		s.logger.writer.SetLokiWriter(&lokiWriter{newArgs.WriteTo})
	}

	// Build all our deferred handlers
	if s.logger.deferredSlog != nil {
		s.logger.deferredSlog.buildHandlers(nil)
	}
	// Print out the buffered logs since we determined the log format already
	for _, bufferedLogChunk := range s.logger.buffer {
		if len(bufferedLogChunk.kvps) > 0 {
			// the buffered logs are currently only sent to the standard output
			// because the components with the receivers are not running yet
			slogadapter.GoKit(s.logger.handler).Log(bufferedLogChunk.kvps...)
		} else {
			// We now can check to see if if our buffered log is at the right level.
			if bufferedLogChunk.handler.Enabled(context.Background(), bufferedLogChunk.record.Level) {
				// These will always be valid due to the build handlers call above.
				bufferedLogChunk.handler.Handle(context.Background(), bufferedLogChunk.record)
			}
		}
	}
	s.logger.buffer = nil

	return nil
}
