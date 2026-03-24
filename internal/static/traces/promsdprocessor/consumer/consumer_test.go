package consumer

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.5.0"
	"gotest.tools/assert"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/util"
)

func TestOperationType(t *testing.T) {
	const (
		attrKey = "key"
		attrIP  = "1.1.1.1"
	)
	testCases := []struct {
		name            string
		operationType   string
		attributeExists bool
		newValue        string
		expectedValue   string
	}{
		{
			name:            "Upsert updates the attribute already exists",
			operationType:   OperationTypeUpsert,
			attributeExists: true,
			newValue:        "new-value",
			expectedValue:   "new-value",
		},
		{
			name:            "Update updates the attribute already exists",
			operationType:   OperationTypeUpdate,
			attributeExists: true,
			newValue:        "new-value",
			expectedValue:   "new-value",
		},
		{
			name:            "Insert does not update the attribute if it's already present",
			operationType:   OperationTypeInsert,
			attributeExists: true,
			newValue:        "new-value",
			expectedValue:   "old-value",
		},
		{
			name:            "Upsert updates the attribute if it isn't present",
			operationType:   OperationTypeUpsert,
			attributeExists: false,
			newValue:        "new-value",
			expectedValue:   "new-value",
		},
		{
			name:            "Update updates the attribute already exists",
			operationType:   OperationTypeUpdate,
			attributeExists: false,
			newValue:        "new-value",
			expectedValue:   "",
		},
		{
			name:            "Insert updates the attribute if it isn't present",
			operationType:   OperationTypeInsert,
			attributeExists: false,
			newValue:        "new-value",
			expectedValue:   "new-value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockProcessor := new(consumertest.TracesSink)
			logger := util.TestLogger(t)
			podAssociations := []string{
				PodAssociationIPLabel,
				PodAssociationOTelIPLabel,
				PodAssociationk8sIPLabel,
				PodAssociationHostnameLabel,
				PodAssociationConnectionIP,
			}
			consumerOpts := Options{
				HostLabels: map[string]discovery.Target{
					attrIP: discovery.NewTargetFromMap(map[string]string{
						attrKey: tc.newValue,
					}),
				},
				OperationType:   tc.operationType,
				PodAssociations: podAssociations,
				NextConsumer:    mockProcessor,
			}
			c, err := NewConsumer(consumerOpts, logger)
			require.NoError(t, err)

			attrMap := pcommon.NewMap()
			if tc.attributeExists {
				attrMap.PutStr(attrKey, "old-value")
			}
			attrMap.PutStr(string(semconv.NetHostIPKey), attrIP)

			c.processAttributes(t.Context(), attrMap)

			actualAttrValue, _ := attrMap.Get(attrKey)
			assert.Equal(t, tc.expectedValue, actualAttrValue.Str())
		})
	}
}

func TestPodAssociation(t *testing.T) {
	const ipStr = "1.1.1.1"

	testCases := []struct {
		name            string
		podAssociations []string
		ctxFn           func(t *testing.T) context.Context
		attrMapFn       func(t *testing.T) pcommon.Map
		expectedIP      string
	}{
		{
			name: "connection IP (HTTP & gRPC)",
			ctxFn: func(t *testing.T) context.Context {
				info := client.Info{
					Addr: &net.IPAddr{
						IP: net.ParseIP(net.ParseIP(ipStr).String()),
					},
				}
				return client.NewContext(t.Context(), info)
			},
			attrMapFn:  func(*testing.T) pcommon.Map { return pcommon.NewMap() },
			expectedIP: ipStr,
		},
		{
			name: "connection IP that includes a port number",
			ctxFn: func(t *testing.T) context.Context {
				info := client.Info{
					Addr: &net.TCPAddr{
						IP:   net.ParseIP(net.ParseIP(ipStr).String()),
						Port: 1234,
					},
				}
				return client.NewContext(t.Context(), info)
			},
			attrMapFn:  func(*testing.T) pcommon.Map { return pcommon.NewMap() },
			expectedIP: ipStr,
		},
		{
			name:            "connection IP is empty",
			podAssociations: []string{PodAssociationConnectionIP},
			ctxFn: func(t *testing.T) context.Context {
				c := client.FromContext(t.Context())
				return client.NewContext(t.Context(), c)
			},
			attrMapFn:  func(*testing.T) pcommon.Map { return pcommon.NewMap() },
			expectedIP: "",
		},
		{
			name:  "ip attribute",
			ctxFn: func(t *testing.T) context.Context { return t.Context() },
			attrMapFn: func(*testing.T) pcommon.Map {
				attrMap := pcommon.NewMap()
				attrMap.PutStr("ip", ipStr)
				return attrMap
			},
			expectedIP: ipStr,
		},
		{
			name:  "net.host.ip attribute",
			ctxFn: func(t *testing.T) context.Context { return t.Context() },
			attrMapFn: func(*testing.T) pcommon.Map {
				attrMap := pcommon.NewMap()
				attrMap.PutStr(string(semconv.NetHostIPKey), ipStr)
				return attrMap
			},
			expectedIP: ipStr,
		},
		{
			name:  "k8s ip attribute",
			ctxFn: func(t *testing.T) context.Context { return t.Context() },
			attrMapFn: func(*testing.T) pcommon.Map {
				attrMap := pcommon.NewMap()
				attrMap.PutStr("k8s.pod.ip", ipStr)
				return attrMap
			},
			expectedIP: ipStr,
		},
		{
			name:  "ip from hostname",
			ctxFn: func(t *testing.T) context.Context { return t.Context() },
			attrMapFn: func(*testing.T) pcommon.Map {
				attrMap := pcommon.NewMap()
				attrMap.PutStr(string(semconv.HostNameKey), ipStr)
				return attrMap
			},
			expectedIP: ipStr,
		},
		{
			name: "uses attr before context (default associations)",
			ctxFn: func(t *testing.T) context.Context {
				info := client.Info{
					Addr: &net.IPAddr{
						IP: net.ParseIP("2.2.2.2"),
					},
				}
				return client.NewContext(t.Context(), info)
			},
			attrMapFn: func(*testing.T) pcommon.Map {
				attrMap := pcommon.NewMap()
				attrMap.PutStr(string(semconv.NetHostIPKey), ipStr)
				return attrMap
			},
			expectedIP: ipStr,
		},
		{
			name:  "uses attr before hostname (default associations)",
			ctxFn: func(t *testing.T) context.Context { return t.Context() },
			attrMapFn: func(*testing.T) pcommon.Map {
				attrMap := pcommon.NewMap()
				attrMap.PutStr(string(semconv.NetHostIPKey), ipStr)
				attrMap.PutStr(string(semconv.HostNameKey), "3.3.3.3")
				return attrMap
			},
			expectedIP: ipStr,
		},
		{
			name:            "ip attribute but not as pod association",
			podAssociations: []string{PodAssociationk8sIPLabel},
			ctxFn:           func(t *testing.T) context.Context { return t.Context() },
			attrMapFn: func(*testing.T) pcommon.Map {
				attrMap := pcommon.NewMap()
				attrMap.PutStr("ip", ipStr)
				return attrMap
			},
			expectedIP: "",
		},
		{
			name:            "uses hostname before attribute (reverse order from default)",
			podAssociations: []string{PodAssociationHostnameLabel, PodAssociationOTelIPLabel},
			ctxFn:           func(t *testing.T) context.Context { return t.Context() },
			attrMapFn: func(*testing.T) pcommon.Map {
				attrMap := pcommon.NewMap()
				attrMap.PutStr(string(semconv.NetHostIPKey), "3.3.3.3")
				attrMap.PutStr(string(semconv.HostNameKey), ipStr)
				return attrMap
			},
			expectedIP: ipStr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockProcessor := new(consumertest.TracesSink)
			logger := util.TestLogger(t)

			if len(tc.podAssociations) == 0 {
				tc.podAssociations = []string{
					PodAssociationIPLabel,
					PodAssociationOTelIPLabel,
					PodAssociationk8sIPLabel,
					PodAssociationHostnameLabel,
					PodAssociationConnectionIP,
				}
			}

			consumerOpts := Options{
				// Don't bother setting up labels - this is not needed for this unit test.
				HostLabels:      map[string]discovery.Target{},
				OperationType:   OperationTypeUpsert,
				PodAssociations: tc.podAssociations,
				NextConsumer:    mockProcessor,
			}
			c, err := NewConsumer(consumerOpts, logger)
			require.NoError(t, err)

			ip := c.getPodIP(tc.ctxFn(t), tc.attrMapFn(t))
			assert.Equal(t, tc.expectedIP, ip)
		})
	}
}
