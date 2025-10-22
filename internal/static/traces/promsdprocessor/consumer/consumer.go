package consumer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/client"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.5.0"

	"github.com/grafana/alloy/internal/component/discovery"
)

const (
	// OperationTypeInsert inserts a new k/v if it isn't already present
	OperationTypeInsert = "insert"
	// OperationTypeUpdate only modifies an existing k/v
	OperationTypeUpdate = "update"
	// OperationTypeUpsert does both of above
	OperationTypeUpsert = "upsert"

	// TODO: It'd be cleaner to get these from the otel semver package?
	//      Not all are in semver though. E.g. "k8s.pod.ip" is internal inside the k8sattributesprocessor.
	PodAssociationIPLabel       = "ip"
	PodAssociationOTelIPLabel   = "net.host.ip"
	PodAssociationk8sIPLabel    = "k8s.pod.ip"
	PodAssociationHostnameLabel = "hostname"
	PodAssociationConnectionIP  = "connection"
)

func ValidateOperationType(operationType string) error {
	switch operationType {
	case
		OperationTypeUpsert,
		OperationTypeInsert,
		OperationTypeUpdate:
		// Valid configuration, do nothing.
	default:
		return fmt.Errorf("unknown operation type %s", operationType)
	}
	return nil
}

func ValidatePodAssociations(podAssociations []string) error {
	for _, podAssociation := range podAssociations {
		switch podAssociation {
		case
			PodAssociationIPLabel,
			PodAssociationOTelIPLabel,
			PodAssociationk8sIPLabel,
			PodAssociationHostnameLabel,
			PodAssociationConnectionIP:
			// Valid configuration, do nothing.
		default:
			return fmt.Errorf("unknown pod association %s", podAssociation)
		}
	}
	return nil
}

// TODO: Put a private member so that this can't be created without calling NewConsumer?
type Consumer struct {
	optsMut sync.RWMutex
	opts    Options

	logger log.Logger
}

// Options configure a Consumer
type Options struct {
	HostLabels      map[string]discovery.Target
	OperationType   string
	PodAssociations []string
	NextConsumer    otelconsumer.Traces
}

var _ otelconsumer.Traces = (*Consumer)(nil)

func NewConsumer(opts Options, logger log.Logger) (*Consumer, error) {
	c := &Consumer{
		logger: logger,
	}

	err := c.UpdateOptions(opts)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// UpdateOptions is used in Alloy where all options need to be updated.
func (c *Consumer) UpdateOptions(opts Options) error {
	c.optsMut.Lock()
	defer c.optsMut.Unlock()

	if opts.NextConsumer == nil {
		return errors.New("nil next Consumer")
	}

	err := ValidateOperationType(opts.OperationType)
	if err != nil {
		return err
	}

	err = ValidatePodAssociations(opts.PodAssociations)
	if err != nil {
		return err
	}

	c.opts = opts
	return nil
}

// UpdateOptionsHostLabels is used in static mode, where only the host labels need to be updated.
func (c *Consumer) UpdateOptionsHostLabels(hostLabels map[string]discovery.Target) {
	c.optsMut.Lock()
	defer c.optsMut.Unlock()

	c.opts.HostLabels = hostLabels
}

func (c *Consumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	c.optsMut.RLock()
	defer c.optsMut.RUnlock()

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)

		c.processAttributes(ctx, rs.Resource().Attributes())
	}

	return c.opts.NextConsumer.ConsumeTraces(ctx, td)
}

func (c *Consumer) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: true}
}

func (c *Consumer) processAttributes(ctx context.Context, attrs pcommon.Map) {
	ip := c.getPodIP(ctx, attrs)
	// have to have an ip for labels lookup
	if ip == "" {
		level.Debug(c.logger).Log("msg", "unable to find ip in span attributes, skipping attribute addition")
		return
	}

	labels, ok := c.opts.HostLabels[ip]
	if !ok {
		level.Debug(c.logger).Log("msg", "unable to find labels", "ip", ip)
		return
	}

	labels.ForEachLabel(func(label string, value string) bool {
		switch c.opts.OperationType {
		case OperationTypeUpsert:
			attrs.PutStr(label, value)
		case OperationTypeInsert:
			if _, ok := attrs.Get(label); !ok {
				attrs.PutStr(label, value)
			}
		case OperationTypeUpdate:
			if toVal, ok := attrs.Get(label); ok {
				toVal.SetStr(value)
			}
		}
		return true
	})
}

func (c *Consumer) getPodIP(ctx context.Context, attrs pcommon.Map) string {
	for _, podAssociation := range c.opts.PodAssociations {
		switch podAssociation {
		case PodAssociationIPLabel, PodAssociationOTelIPLabel, PodAssociationk8sIPLabel:
			ip := stringAttributeFromMap(attrs, podAssociation)
			if ip != "" {
				return ip
			}
		case PodAssociationHostnameLabel:
			hostname := stringAttributeFromMap(attrs, string(semconv.HostNameKey))
			if net.ParseIP(hostname) != nil {
				return hostname
			}
		case PodAssociationConnectionIP:
			ip := c.getConnectionIP(ctx)
			if ip != "" {
				return ip
			}
		}
	}
	return ""
}

func stringAttributeFromMap(attrs pcommon.Map, key string) string {
	if attr, ok := attrs.Get(key); ok {
		if attr.Type() == pcommon.ValueTypeStr {
			return attr.Str()
		}
	}
	return ""
}

func (c *Consumer) getConnectionIP(ctx context.Context) string {
	cl := client.FromContext(ctx)
	if cl.Addr == nil {
		return ""
	}

	host := cl.Addr.String()
	if strings.Contains(host, ":") {
		var err error
		splitHost, _, err := net.SplitHostPort(host)
		if err != nil {
			// It's normal for this to fail for IPv6 address strings that don't actually include a port.
			level.Debug(c.logger).Log("msg", "unable to split connection host and port", "host", host, "err", err)
		} else {
			host = splitHost
		}
	}

	return host
}

func GetHostFromLabels(labels discovery.Target) (string, error) {
	address, ok := labels.Get(model.AddressLabel)
	if !ok {
		return "", fmt.Errorf("unable to find address in labels %q", labels)
	}

	host := address
	if strings.Contains(host, ":") {
		var err error
		host, _, err = net.SplitHostPort(host)
		if err != nil {
			return "", fmt.Errorf("unable to split host and port in address %q: %w", address, err)
		}
	}

	return host, nil
}

func NewTargetsWithNonInternalLabels(target discovery.Target) discovery.Target {
	return discovery.NewTargetFromLabelSet(target.NonReservedLabelSet())
}
