package dns

import (
	"net"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestBuildSRVTarget(t *testing.T) {
	labels := buildSRVTarget("_ldap._tcp.example.com", &net.SRV{Target: "dc1.example.com.", Port: 389})

	require.Equal(t, model.LabelValue("dc1.example.com:389"), labels[model.AddressLabel])
	require.Equal(t, model.LabelValue("_ldap._tcp.example.com"), labels[dnsNameLabel])
	require.Equal(t, model.LabelValue("dc1.example.com."), labels[dnsSrvRecordTargetLabel])
	require.Equal(t, model.LabelValue("389"), labels[dnsSrvRecordPortLabel])
}

func TestBuildMXTarget(t *testing.T) {
	labels := buildMXTarget("example.com", &net.MX{Host: "mail.example.com."}, 25)

	require.Equal(t, model.LabelValue("mail.example.com:25"), labels[model.AddressLabel])
	require.Equal(t, model.LabelValue("example.com"), labels[dnsNameLabel])
	require.Equal(t, model.LabelValue("mail.example.com."), labels[dnsMxRecordTargetLabel])
}

func TestBuildNSTarget(t *testing.T) {
	labels := buildNSTarget("example.com", &net.NS{Host: "ns1.example.com."}, 53)

	require.Equal(t, model.LabelValue("ns1.example.com:53"), labels[model.AddressLabel])
	require.Equal(t, model.LabelValue("example.com"), labels[dnsNameLabel])
	require.Equal(t, model.LabelValue("ns1.example.com."), labels[dnsNsRecordTargetLabel])
}

func TestBuildIPTarget(t *testing.T) {
	labels := buildIPTarget("example.com", net.ParseIP("2001:db8::1"), 8080)

	require.Equal(t, model.LabelValue("[2001:db8::1]:8080"), labels[model.AddressLabel])
	require.Equal(t, model.LabelValue("example.com"), labels[dnsNameLabel])
}
