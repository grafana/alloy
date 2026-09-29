// Package producer implements the kafka.tenant_producer component, which
// accepts telemetry over HTTP and writes each request to the Kafka partition
// assigned to the request's tenant.
package producer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/units"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/kafka/tenant/kafkaclient"
	"github.com/grafana/alloy/internal/component/kafka/tenant/registry"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "kafka.tenant_producer",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// HeaderOrgID is the HTTP header carrying the tenant ID.
const HeaderOrgID = "X-Scope-OrgID"

// Arguments configures kafka.tenant_producer.
type Arguments struct {
	Server   *fnet.ServerConfig        `alloy:",squash"`
	Registry alloytypes.OptionalSecret `alloy:"registry,attr"`

	MaxBodySize        units.Base2Bytes `alloy:"max_body_size,attr,optional"`
	ProduceTimeout     time.Duration    `alloy:"produce_timeout,attr,optional"`
	Linger             time.Duration    `alloy:"linger,attr,optional"`
	Compression        string           `alloy:"compression,attr,optional"`
	MaxBufferedRecords int              `alloy:"max_buffered_records,attr,optional"`

	Client kafkaclient.Arguments `alloy:"client,block"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Server:             fnet.DefaultServerConfig(),
		MaxBodySize:        units.MiB,
		ProduceTimeout:     10 * time.Second,
		Linger:             10 * time.Millisecond,
		Compression:        "none",
		MaxBufferedRecords: 10000,
	}
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.MaxBodySize <= 0 {
		return errors.New("max_body_size must be greater than 0")
	}
	if a.ProduceTimeout <= 0 {
		return errors.New("produce_timeout must be greater than 0")
	}
	if a.MaxBufferedRecords <= 0 {
		return errors.New("max_buffered_records must be greater than 0")
	}
	if _, err := compressionCodec(a.Compression); err != nil {
		return err
	}
	return nil
}

func compressionCodec(name string) (kgo.CompressionCodec, error) {
	switch name {
	case "", "none":
		return kgo.NoCompression(), nil
	case "gzip":
		return kgo.GzipCompression(), nil
	case "snappy":
		return kgo.SnappyCompression(), nil
	case "lz4":
		return kgo.Lz4Compression(), nil
	case "zstd":
		return kgo.ZstdCompression(), nil
	default:
		return kgo.CompressionCodec{}, fmt.Errorf("unsupported compression %q: valid values are none, gzip, snappy, lz4, zstd", name)
	}
}

// clientArgs is the subset of Arguments that requires recreating the Kafka
// client when changed.
type clientArgs struct {
	Client             kafkaclient.Arguments
	Linger             time.Duration
	Compression        string
	MaxBufferedRecords int
	MaxBodySize        units.Base2Bytes
	ProduceTimeout     time.Duration
}

func (a Arguments) clientArgs() clientArgs {
	return clientArgs{
		Client:             a.Client,
		Linger:             a.Linger,
		Compression:        a.Compression,
		MaxBufferedRecords: a.MaxBufferedRecords,
		MaxBodySize:        a.MaxBodySize,
		ProduceTimeout:     a.ProduceTimeout,
	}
}

func (ca clientArgs) opts() ([]kgo.Opt, error) {
	opts, err := ca.Client.Opts()
	if err != nil {
		return nil, err
	}
	codec, err := compressionCodec(ca.Compression)
	if err != nil {
		return nil, err
	}
	return append(opts,
		kgo.RecordPartitioner(kgo.ManualPartitioner()),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerLinger(ca.Linger),
		// Leave room for record headers and batch overhead.
		kgo.ProducerBatchMaxBytes(int32(ca.MaxBodySize)+64*1024),
		kgo.ProducerBatchCompression(codec),
		kgo.MaxBufferedRecords(ca.MaxBufferedRecords),
		kgo.RecordDeliveryTimeout(ca.ProduceTimeout),
	), nil
}

// Component implements kafka.tenant_producer.
type Component struct {
	opts               component.Options
	metrics            *metrics
	uncheckedCollector *util.UncheckedCollector

	registry registry.Holder
	client   atomic.Pointer[kgo.Client]
	topicOK  atomic.Bool
	recheck  chan struct{}

	mut    sync.Mutex
	args   Arguments
	server *fnet.TargetServer
	health component.Health
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
)

// New creates a new kafka.tenant_producer component.
func New(opts component.Options, args Arguments) (*Component, error) {
	uncheckedCollector := util.NewUncheckedCollector(nil)
	opts.Registerer.MustRegister(uncheckedCollector)

	c := &Component{
		opts:               opts,
		metrics:            newMetrics(opts.Registerer),
		uncheckedCollector: uncheckedCollector,
		recheck:            make(chan struct{}, 1),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.shutdownServer()
		if cl := c.client.Swap(nil); cl != nil {
			cl.Close()
		}
	}()

	// Verify the topic until it succeeds, and again whenever the client or
	// registry changes. Produce requests are rejected while it fails.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	c.checkTopic(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.recheck:
			c.checkTopic(ctx)
		case <-ticker.C:
			if !c.topicOK.Load() {
				c.checkTopic(ctx)
			}
		}
	}
}

func (c *Component) checkTopic(ctx context.Context) {
	cl, reg := c.client.Load(), c.registry.Load()
	if cl == nil || reg == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := kafkaclient.CheckTopic(ctx, cl, reg); err != nil {
		c.topicOK.Store(false)
		c.setHealth(component.HealthTypeUnhealthy, err.Error())
		c.opts.Logger.Warn("topic check failed, rejecting produce requests", "err", err)
		return
	}
	c.topicOK.Store(true)
	c.setHealth(component.HealthTypeHealthy, "topic verified")
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// Reject a bad registry as a whole; the previous one stays in use.
	reg, err := registry.Parse([]byte(newArgs.Registry.Value))
	if err != nil {
		return err
	}

	c.mut.Lock()
	defer c.mut.Unlock()

	// Re-verify the topic only when the client or the topic layout changed,
	// so that adding a tenant doesn't briefly reject requests.
	old := c.registry.Load()
	recheck := old == nil || old.Topic() != reg.Topic() || old.Partitions() != reg.Partitions()

	if c.client.Load() == nil || !reflect.DeepEqual(c.args.clientArgs(), newArgs.clientArgs()) {
		recheck = true
		opts, err := newArgs.clientArgs().opts()
		if err != nil {
			return err
		}
		cl, err := kgo.NewClient(opts...)
		if err != nil {
			return fmt.Errorf("creating kafka client: %w", err)
		}
		if old := c.client.Swap(cl); old != nil {
			// Close waits for buffered records, so don't block Update on it.
			go old.Close()
		}
	}
	c.registry.Store(reg)
	if recheck {
		c.topicOK.Store(false)
		select {
		case c.recheck <- struct{}{}:
		default:
		}
	}

	if c.server == nil || !reflect.DeepEqual(c.args.Server, newArgs.Server) {
		c.shutdownServer()
		if err := c.startServer(newArgs); err != nil {
			return err
		}
	}

	c.args = newArgs
	return nil
}

func (c *Component) startServer(args Arguments) error {
	// See prometheus.receive_http for why each server gets its own registry.
	serverRegistry := prometheus.NewRegistry()
	c.uncheckedCollector.SetCollector(serverRegistry)

	s, err := fnet.NewTargetServer(c.opts.Logger, "kafka_tenant_producer", serverRegistry, args.Server)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	c.server = s
	return c.server.MountAndRun(c.mountRoutes)
}

func (c *Component) mountRoutes(router *mux.Router) {
	for _, r := range routes {
		router.Path(r.path).Methods(http.MethodPost).Handler(c.handler(r))
	}
}

func (c *Component) shutdownServer() {
	if c.server != nil {
		c.server.StopAndShutdown()
		c.server = nil
	}
}

// CurrentHealth implements component.HealthComponent.
func (c *Component) CurrentHealth() component.Health {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.health
}

func (c *Component) setHealth(typ component.HealthType, msg string) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.health = component.Health{Health: typ, Message: msg, UpdateTime: time.Now()}
}

func (c *Component) currentArgs() Arguments {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.args
}

type route struct {
	path   string
	signal string
	format string
}

var routes = []route{
	{"/api/v1/push", kafkaclient.SignalMetrics, kafkaclient.FormatPromRWv1},
	{"/loki/api/v1/push", kafkaclient.SignalLogs, kafkaclient.FormatLokiPush},
	{"/ingest", kafkaclient.SignalProfiles, kafkaclient.FormatPyroscopeIngest},
	{"/v1/metrics", kafkaclient.SignalMetrics, kafkaclient.FormatOTLP},
	{"/v1/logs", kafkaclient.SignalLogs, kafkaclient.FormatOTLP},
	{"/v1/traces", kafkaclient.SignalTraces, kafkaclient.FormatOTLP},
}

func (c *Component) handler(rt route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, msg := c.handle(r, rt)
		c.metrics.requests.WithLabelValues(rt.signal, rt.format, strconv.Itoa(code)).Inc()
		if code >= 300 {
			http.Error(w, msg, code)
			return
		}
		writeSuccess(w, r, rt)
	})
}

// handle produces the request to Kafka and returns the status code to reply
// with. It only returns a 2xx code once the broker acknowledged the record.
func (c *Component) handle(r *http.Request, rt route) (int, string) {
	tenant := r.Header.Get(HeaderOrgID)
	if tenant == "" {
		c.metrics.rejected.WithLabelValues("no_tenant").Inc()
		return http.StatusUnauthorized, "missing " + HeaderOrgID + " header"
	}
	if strings.Contains(tenant, "|") {
		c.metrics.rejected.WithLabelValues("multi_tenant").Inc()
		return http.StatusBadRequest, "multi-tenant requests are not supported"
	}

	reg := c.registry.Load()
	partition, ok := reg.PartitionFor(tenant)
	if !ok {
		// Never fall back to hashing: that could put two tenants on one partition.
		c.metrics.rejected.WithLabelValues("unknown_tenant").Inc()
		return http.StatusForbidden, "unknown tenant"
	}

	cl := c.client.Load()
	if cl == nil || !c.topicOK.Load() {
		return http.StatusServiceUnavailable, "producer not ready"
	}

	args := c.currentArgs()
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, int64(args.MaxBodySize)))
	if err != nil {
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			c.metrics.rejected.WithLabelValues("too_large").Inc()
			return http.StatusRequestEntityTooLarge, "request body too large"
		}
		return http.StatusBadRequest, "reading request body: " + err.Error()
	}

	rec := &kgo.Record{
		Topic:     reg.Topic(),
		Partition: partition,
		Key:       []byte(tenant),
		Value:     body,
		Headers: []kgo.RecordHeader{
			{Key: kafkaclient.HeaderTenantID, Value: []byte(tenant)},
			{Key: kafkaclient.HeaderSignal, Value: []byte(rt.signal)},
			{Key: kafkaclient.HeaderFormat, Value: []byte(rt.format)},
			{Key: kafkaclient.HeaderContentType, Value: []byte(r.Header.Get("Content-Type"))},
			{Key: kafkaclient.HeaderContentEncoding, Value: []byte(r.Header.Get("Content-Encoding"))},
			{Key: kafkaclient.HeaderURL, Value: []byte(r.URL.RequestURI())},
			{Key: kafkaclient.HeaderSchemaVersion, Value: []byte(kafkaclient.SchemaVersion)},
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), args.ProduceTimeout)
	defer cancel()

	start := time.Now()
	err = produce(ctx, cl, rec)
	c.metrics.produceDuration.Observe(time.Since(start).Seconds())

	switch {
	case err == nil:
		c.metrics.producedBytes.WithLabelValues(rt.signal).Add(float64(len(body)))
		return http.StatusNoContent, ""
	case errors.Is(err, kgo.ErrMaxBuffered):
		return http.StatusTooManyRequests, "producer buffer full"
	default:
		c.opts.Logger.Warn("failed to produce record", "partition", partition, "err", err)
		return http.StatusServiceUnavailable, "failed to produce record"
	}
}

// produce buffers rec without blocking on a full buffer and waits for the
// broker to acknowledge it.
func produce(ctx context.Context, cl *kgo.Client, rec *kgo.Record) error {
	done := make(chan error, 1)
	cl.TryProduce(ctx, rec, func(_ *kgo.Record, err error) { done <- err })
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// writeSuccess writes a success response each client type understands.
func writeSuccess(w http.ResponseWriter, r *http.Request, rt route) {
	switch rt.format {
	case kafkaclient.FormatOTLP:
		// An empty body is a valid empty protobuf Export*ServiceResponse.
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	case kafkaclient.FormatPyroscopeIngest:
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
