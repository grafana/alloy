//go:build windows

package dns

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/promslog"
	promdiscovery "github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/refresh"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/grafana/alloy/internal/component/discovery"
)

type windowsSDConfig struct {
	Names           []string
	RefreshInterval time.Duration
	Type            string
	Port            int
}

func newDiscovererConfig(args Arguments) discovery.DiscovererConfig {
	return &windowsSDConfig{
		Names:           args.Names,
		RefreshInterval: args.RefreshInterval,
		Type:            strings.ToUpper(args.Type),
		Port:            args.Port,
	}
}

func (*windowsSDConfig) Name() string {
	return "dns"
}

func (c *windowsSDConfig) NewDiscoverer(opts promdiscovery.DiscovererOptions) (promdiscovery.Discoverer, error) {
	m, ok := opts.Metrics.(*windowsDNSMetrics)
	if !ok {
		return nil, errors.New("invalid discovery metrics type")
	}

	logger := opts.Logger
	if logger == nil {
		logger = promslog.NewNopLogger()
	}

	d := &windowsDiscovery{
		names:    c.Names,
		recordTy: c.Type,
		port:     c.Port,
		logger:   logger,
		metrics:  m,
		resolver: net.DefaultResolver,
	}

	d.Discovery = refresh.NewDiscovery(refresh.Options{
		Logger:              logger,
		Mech:                "dns",
		SetName:             opts.SetName,
		Interval:            c.RefreshInterval,
		RefreshF:            d.refresh,
		MetricsInstantiator: m.refreshMetrics,
	})

	return d, nil
}

func (*windowsSDConfig) NewDiscovererMetrics(reg prometheus.Registerer, rmi promdiscovery.RefreshMetricsInstantiator) promdiscovery.DiscovererMetrics {
	return newWindowsDNSMetrics(reg, rmi)
}

type dnsResolver interface {
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupNS(ctx context.Context, name string) ([]*net.NS, error)
}

type windowsDiscovery struct {
	*refresh.Discovery

	names    []string
	recordTy string
	port     int
	logger   *slog.Logger
	metrics  *windowsDNSMetrics
	resolver dnsResolver
}

func (d *windowsDiscovery) refresh(ctx context.Context) ([]*targetgroup.Group, error) {
	var (
		wg  sync.WaitGroup
		ch  = make(chan *targetgroup.Group)
		tgs = make([]*targetgroup.Group, 0, len(d.names))
	)

	wg.Add(len(d.names))
	for _, name := range d.names {
		go func(name string) {
			defer wg.Done()

			tg, err := d.refreshOne(ctx, name)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					d.logger.Error("Error refreshing DNS targets", "err", err)
				}
				return
			}

			select {
			case <-ctx.Done():
			case ch <- tg:
			}
		}(name)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for tg := range ch {
		tgs = append(tgs, tg)
	}

	return tgs, nil
}

func (d *windowsDiscovery) refreshOne(ctx context.Context, name string) (*targetgroup.Group, error) {
	d.metrics.dnsSDLookupsCount.Inc()

	tg := &targetgroup.Group{Source: name}

	var err error
	switch d.recordTy {
	case "SRV":
		_, addrs, lookupErr := d.resolver.LookupSRV(ctx, "", "", name)
		err = lookupErr
		for _, addr := range addrs {
			tg.Targets = append(tg.Targets, buildSRVTarget(name, addr))
		}
	case "A":
		ips, lookupErr := d.resolver.LookupIP(ctx, "ip4", name)
		err = lookupErr
		for _, ip := range ips {
			tg.Targets = append(tg.Targets, buildIPTarget(name, ip, d.port))
		}
	case "AAAA":
		ips, lookupErr := d.resolver.LookupIP(ctx, "ip6", name)
		err = lookupErr
		for _, ip := range ips {
			tg.Targets = append(tg.Targets, buildIPTarget(name, ip, d.port))
		}
	case "MX":
		mxRecords, lookupErr := d.resolver.LookupMX(ctx, name)
		err = lookupErr
		for _, record := range mxRecords {
			tg.Targets = append(tg.Targets, buildMXTarget(name, record, d.port))
		}
	case "NS":
		nsRecords, lookupErr := d.resolver.LookupNS(ctx, name)
		err = lookupErr
		for _, record := range nsRecords {
			tg.Targets = append(tg.Targets, buildNSTarget(name, record, d.port))
		}
	default:
		err = fmt.Errorf("unsupported DNS record type %q", d.recordTy)
	}

	if err == nil || isNotFound(err) {
		return tg, nil
	}

	d.metrics.dnsSDLookupFailuresCount.Inc()
	return nil, err
}

func isNotFound(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsNotFound
	}

	return false
}

type windowsDNSMetrics struct {
	refreshMetrics promdiscovery.RefreshMetricsInstantiator

	dnsSDLookupsCount        prometheus.Counter
	dnsSDLookupFailuresCount prometheus.Counter

	metricRegisterer promdiscovery.MetricRegisterer
}

func newWindowsDNSMetrics(reg prometheus.Registerer, rmi promdiscovery.RefreshMetricsInstantiator) promdiscovery.DiscovererMetrics {
	m := &windowsDNSMetrics{
		refreshMetrics: rmi,
		dnsSDLookupsCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: dnsMetricsNamespace,
			Name:      "sd_dns_lookups_total",
			Help:      "The number of DNS-SD lookups.",
		}),
		dnsSDLookupFailuresCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: dnsMetricsNamespace,
			Name:      "sd_dns_lookup_failures_total",
			Help:      "The number of DNS-SD lookup failures.",
		}),
	}

	m.metricRegisterer = promdiscovery.NewMetricRegisterer(reg, []prometheus.Collector{
		m.dnsSDLookupsCount,
		m.dnsSDLookupFailuresCount,
	})

	return m
}

func (m *windowsDNSMetrics) Register() error {
	return m.metricRegisterer.RegisterMetrics()
}

func (m *windowsDNSMetrics) Unregister() {
	m.metricRegisterer.UnregisterMetrics()
}
