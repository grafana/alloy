package dbo11yreceiver

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	labelJob      = "job"
	labelInstance = "instance"
	labelServerID = "server_id"

	attrServiceName       = "service.name"
	attrServiceInstanceID = "service.instance.id"
)

// serverIDOnResource is the one switch for where server_id lives on both
// signals (fork F-15). This branch is side b: server_id stays on every metric
// datapoint and every log record, as it ships today. It's a var so the
// always-false branches don't trip unparam.
var serverIDOnResource = false

// putResource sets the resource attributes of one database's output from its
// labels. job and instance become service.name and service.instance.id, which
// Prometheus OTLP ingest turns back into job and instance.
func putResource(res pcommon.Map, labels map[string]string) {
	res.PutStr(attrServiceName, labels[labelJob])
	res.PutStr(attrServiceInstanceID, labels[labelInstance])
	if serverIDOnResource {
		res.PutStr(labelServerID, labels[labelServerID])
	}
}

// datapointLabel reports whether a target label goes on every metric
// datapoint. job and instance are already on the resource.
func datapointLabel(name string) bool {
	switch name {
	case labelJob, labelInstance:
		return false
	case labelServerID:
		return !serverIDOnResource
	}
	return true
}

// logRecordLabel reports whether a Loki label becomes a log record attribute.
// job and instance stay on the record as well as the resource: Loki doesn't
// rebuild them from service.*.
func logRecordLabel(name string) bool {
	if name == labelServerID {
		return !serverIDOnResource
	}
	return true
}
