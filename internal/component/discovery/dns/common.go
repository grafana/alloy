package dns

import (
	"net"
	"strconv"
	"strings"

	"github.com/prometheus/common/model"
)

const (
	dnsNameLabel            = model.MetaLabelPrefix + "dns_name"
	dnsSrvRecordPrefix      = model.MetaLabelPrefix + "dns_srv_record_"
	dnsSrvRecordTargetLabel = dnsSrvRecordPrefix + "target"
	dnsSrvRecordPortLabel   = dnsSrvRecordPrefix + "port"
	dnsMxRecordPrefix       = model.MetaLabelPrefix + "dns_mx_record_"
	dnsMxRecordTargetLabel  = dnsMxRecordPrefix + "target"
	dnsNsRecordPrefix       = model.MetaLabelPrefix + "dns_ns_record_"
	dnsNsRecordTargetLabel  = dnsNsRecordPrefix + "target"

	dnsMetricsNamespace = "prometheus"
)

func hostPort(host string, port int) model.LabelValue {
	return model.LabelValue(net.JoinHostPort(host, strconv.Itoa(port)))
}

func trimRootedName(name string) string {
	return strings.TrimRight(name, ".")
}

func buildIPTarget(name string, ip net.IP, port int) model.LabelSet {
	return model.LabelSet{
		model.AddressLabel: hostPort(ip.String(), port),
		dnsNameLabel:       model.LabelValue(name),
	}
}

func buildSRVTarget(name string, record *net.SRV) model.LabelSet {
	return model.LabelSet{
		model.AddressLabel:      hostPort(trimRootedName(record.Target), int(record.Port)),
		dnsNameLabel:            model.LabelValue(name),
		dnsSrvRecordTargetLabel: model.LabelValue(record.Target),
		dnsSrvRecordPortLabel:   model.LabelValue(strconv.Itoa(int(record.Port))),
	}
}

func buildMXTarget(name string, record *net.MX, port int) model.LabelSet {
	return model.LabelSet{
		model.AddressLabel:     hostPort(trimRootedName(record.Host), port),
		dnsNameLabel:           model.LabelValue(name),
		dnsMxRecordTargetLabel: model.LabelValue(record.Host),
	}
}

func buildNSTarget(name string, record *net.NS, port int) model.LabelSet {
	return model.LabelSet{
		model.AddressLabel:     hostPort(trimRootedName(record.Host), port),
		dnsNameLabel:           model.LabelValue(name),
		dnsNsRecordTargetLabel: model.LabelValue(record.Host),
	}
}
