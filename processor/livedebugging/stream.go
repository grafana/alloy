// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package livedebugging

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const pathStream = "/debug/livedebugging/v1/stream"

// streamServer optionally exposes a laptop-pullable HTTP stream of the
// traces observed by a tracesProcessor. It never blocks the pipeline: a
// clone is handed to a bounded per-subscriber channel via a non-blocking
// send, and a hard per-stream spans/second budget drops excess rather
// than queuing it.
type streamServer struct {
	cfg    StreamConfig
	logger *zap.Logger

	server   *http.Server
	stopCh   chan struct{}
	listener net.Listener

	mu          sync.Mutex
	subscribers []*subscriber
	slots       chan struct{}
}

type subscriber struct {
	tapID   string // unused placeholder for future multi-point support
	batches chan ptrace.Traces
	limiter *rateLimiter
}

func newStreamServer(cfg StreamConfig, logger *zap.Logger) *streamServer {
	return &streamServer{
		cfg:    cfg,
		logger: logger,
		slots:  make(chan struct{}, cfg.MaxConcurrentStreams),
	}
}

func (s *streamServer) start(ctx context.Context, host component.Host, telemetry component.TelemetrySettings) error {
	mux := http.NewServeMux()
	mux.HandleFunc(pathStream, s.handleStream)

	ln, err := s.cfg.ToListener(ctx)
	if err != nil {
		return err
	}
	s.listener = ln
	s.server, err = s.cfg.ToServer(ctx, host.GetExtensions(), telemetry, mux)
	if err != nil {
		return err
	}

	s.stopCh = make(chan struct{})
	go func() {
		defer close(s.stopCh)
		if errHTTP := s.server.Serve(ln); errHTTP != nil && !errors.Is(errHTTP, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()
	return nil
}

func (s *streamServer) shutdown(context.Context) error {
	if s.server == nil {
		return nil
	}
	err := s.server.Close()
	if s.stopCh != nil {
		<-s.stopCh
	}
	return err
}

// publish hands traces to every active subscriber. It never blocks: a
// full subscriber buffer or an over-budget rate limiter just drops the
// batch for that subscriber.
func (s *streamServer) publish(traces ptrace.Traces) {
	s.mu.Lock()
	subs := append([]*subscriber(nil), s.subscribers...)
	s.mu.Unlock()
	if len(subs) == 0 {
		return
	}

	spanCount := traces.SpanCount()
	for _, sub := range subs {
		if !sub.limiter.allow(spanCount) {
			continue
		}
		clone := ptrace.NewTraces()
		traces.CopyTo(clone)
		select {
		case sub.batches <- clone:
		default:
		}
	}
}

func (s *streamServer) handleStream(w http.ResponseWriter, r *http.Request) {
	duration := s.cfg.DefaultStreamDuration
	if raw := r.URL.Query().Get("duration"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			http.Error(w, "invalid \"duration\": "+err.Error(), http.StatusBadRequest)
			return
		}
		duration = parsed
	}
	if duration > s.cfg.MaxStreamDuration {
		duration = s.cfg.MaxStreamDuration
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		http.Error(w, "max_concurrent_streams reached", http.StatusConflict)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), duration)
	defer cancel()

	sub := &subscriber{
		batches: make(chan ptrace.Traces, s.cfg.StreamBufferSize),
		limiter: newRateLimiter(s.cfg.MaxSpansPerSecond),
	}
	s.mu.Lock()
	s.subscribers = append(s.subscribers, sub)
	s.mu.Unlock()
	defer s.removeSubscriber(sub)

	w.Header().Set("Content-Type", "application/vnd.opentelemetry.tracetap.v1+octet-stream")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	marshaler := ptrace.ProtoMarshaler{}
	for {
		select {
		case <-ctx.Done():
			return
		case tr := <-sub.batches:
			if err := writeFrame(w, marshaler, tr); err != nil {
				s.logger.Debug("live_debugging stream write failed, stopping stream", zap.Error(err))
				return
			}
			flusher.Flush()
		}
	}
}

func (s *streamServer) removeSubscriber(target *subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subscribers {
		if sub == target {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			return
		}
	}
}

// writeFrame writes traces as a 4-byte big-endian length prefix followed
// by its OTLP proto encoding — the same framing cmd/tracetapclient reads.
func writeFrame(w http.ResponseWriter, marshaler ptrace.ProtoMarshaler, traces ptrace.Traces) error {
	data, err := marshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data))) //nolint:gosec // frame length always fits uint32 in practice
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// rateLimiter is a simple fixed-window counter: at most max units may be
// consumed per rolling one-second window.
type rateLimiter struct {
	mu          sync.Mutex
	max         int
	windowStart time.Time
	count       int
	now         func() time.Time
}

func newRateLimiter(maxPerSecond int) *rateLimiter {
	return &rateLimiter{max: maxPerSecond, now: time.Now}
}

func (r *rateLimiter) allow(n int) bool {
	if r.max <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if now.Sub(r.windowStart) >= time.Second {
		r.windowStart = now
		r.count = 0
	}
	if r.count+n > r.max {
		return false
	}
	r.count += n
	return true
}
