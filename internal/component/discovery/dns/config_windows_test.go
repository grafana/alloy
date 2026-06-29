//go:build windows

package dns

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	lookupSRV func(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
	lookupIP  func(ctx context.Context, network, host string) ([]net.IP, error)
	lookupMX  func(ctx context.Context, name string) ([]*net.MX, error)
	lookupNS  func(ctx context.Context, name string) ([]*net.NS, error)
}

func (f fakeResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	return f.lookupSRV(ctx, service, proto, name)
}

func (f fakeResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	return f.lookupIP(ctx, network, host)
}

func (f fakeResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	return f.lookupMX(ctx, name)
}

func (f fakeResolver) LookupNS(ctx context.Context, name string) ([]*net.NS, error) {
	return f.lookupNS(ctx, name)
}

func TestWindowsDiscoveryRefreshOneSRV(t *testing.T) {
	d := &windowsDiscovery{
		recordTy: "SRV",
		metrics:  newTestWindowsDNSMetrics(),
		resolver: fakeResolver{
			lookupSRV: func(_ context.Context, service, proto, name string) (string, []*net.SRV, error) {
				require.Empty(t, service)
				require.Empty(t, proto)
				require.Equal(t, "_ldap._tcp.example.com", name)
				return "", []*net.SRV{{Target: "dc1.example.com.", Port: 389}}, nil
			},
			lookupIP: func(context.Context, string, string) ([]net.IP, error) {
				return nil, errors.New("unexpected LookupIP call")
			},
			lookupMX: func(context.Context, string) ([]*net.MX, error) { return nil, errors.New("unexpected LookupMX call") },
			lookupNS: func(context.Context, string) ([]*net.NS, error) { return nil, errors.New("unexpected LookupNS call") },
		},
	}

	tg, err := d.refreshOne(context.Background(), "_ldap._tcp.example.com")
	require.NoError(t, err)
	require.Equal(t, "_ldap._tcp.example.com", tg.Source)
	require.Len(t, tg.Targets, 1)
	require.Equal(t, model.LabelValue("dc1.example.com:389"), tg.Targets[0][model.AddressLabel])
	require.Equal(t, model.LabelValue("dc1.example.com."), tg.Targets[0][dnsSrvRecordTargetLabel])
}

func TestWindowsDiscoveryRefreshOneNotFoundReturnsEmptyGroup(t *testing.T) {
	d := &windowsDiscovery{
		recordTy: "A",
		port:     8080,
		metrics:  newTestWindowsDNSMetrics(),
		resolver: fakeResolver{
			lookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
				return "", nil, errors.New("unexpected LookupSRV call")
			},
			lookupIP: func(_ context.Context, network, host string) ([]net.IP, error) {
				require.Equal(t, "ip4", network)
				require.Equal(t, "missing.example.com", host)
				return nil, &net.DNSError{IsNotFound: true}
			},
			lookupMX: func(context.Context, string) ([]*net.MX, error) { return nil, errors.New("unexpected LookupMX call") },
			lookupNS: func(context.Context, string) ([]*net.NS, error) { return nil, errors.New("unexpected LookupNS call") },
		},
	}

	tg, err := d.refreshOne(context.Background(), "missing.example.com")
	require.NoError(t, err)
	require.Equal(t, "missing.example.com", tg.Source)
	require.Empty(t, tg.Targets)
	assertCounterValue(t, d.metrics.dnsSDLookupsCount, 1)
	assertCounterValue(t, d.metrics.dnsSDLookupFailuresCount, 0)
}

func TestWindowsDiscoveryRefreshOneCountsFailures(t *testing.T) {
	d := &windowsDiscovery{
		recordTy: "AAAA",
		port:     8080,
		metrics:  newTestWindowsDNSMetrics(),
		resolver: fakeResolver{
			lookupSRV: func(context.Context, string, string, string) (string, []*net.SRV, error) {
				return "", nil, errors.New("unexpected LookupSRV call")
			},
			lookupIP: func(context.Context, string, string) ([]net.IP, error) { return nil, errors.New("resolver failed") },
			lookupMX: func(context.Context, string) ([]*net.MX, error) { return nil, errors.New("unexpected LookupMX call") },
			lookupNS: func(context.Context, string) ([]*net.NS, error) { return nil, errors.New("unexpected LookupNS call") },
		},
	}

	tg, err := d.refreshOne(context.Background(), "broken.example.com")
	require.Nil(t, tg)
	require.EqualError(t, err, "resolver failed")
	assertCounterValue(t, d.metrics.dnsSDLookupsCount, 1)
	assertCounterValue(t, d.metrics.dnsSDLookupFailuresCount, 1)
}

func newTestWindowsDNSMetrics() *windowsDNSMetrics {
	return &windowsDNSMetrics{
		dnsSDLookupsCount:        prometheus.NewCounter(prometheus.CounterOpts{Name: "test_dns_lookups_total"}),
		dnsSDLookupFailuresCount: prometheus.NewCounter(prometheus.CounterOpts{Name: "test_dns_lookup_failures_total"}),
	}
}

func assertCounterValue(t *testing.T, counter prometheus.Counter, want float64) {
	t.Helper()
	require.Equal(t, want, testutil.ToFloat64(counter))
}
