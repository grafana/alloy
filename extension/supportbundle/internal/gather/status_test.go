package gather

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
	"gopkg.in/yaml.v3"
)

func TestStatusGathererNoEvents(t *testing.T) {
	g := NewStatus()
	files, err := g.Gather(context.Background(), Options{})
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestStatusGathererRecordsStatuses(t *testing.T) {
	g := NewStatus()

	traces := pipeline.NewID(pipeline.SignalTraces)
	receiver := componentstatus.NewInstanceID(component.MustNewID("otlp"), component.KindReceiver, traces)
	exporter := componentstatus.NewInstanceID(component.MustNewID("otlp"), component.KindExporter, traces)

	g.Record(receiver, componentstatus.NewEvent(componentstatus.StatusOK))
	g.Record(exporter, componentstatus.NewPermanentErrorEvent(errors.New("connection refused")))

	files, err := g.Gather(context.Background(), Options{})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "component-status.yaml")

	var list []componentStatus
	require.NoError(t, yaml.Unmarshal(m["component-status.yaml"], &list))
	require.Len(t, list, 2)

	byKind := make(map[string]componentStatus, len(list))
	for _, cs := range list {
		byKind[cs.Kind] = cs
	}
	require.Equal(t, "StatusOK", byKind["Receiver"].Status)
	require.Equal(t, "StatusPermanentError", byKind["Exporter"].Status)
	require.Equal(t, "connection refused", byKind["Exporter"].Error)
	require.Contains(t, byKind["Exporter"].Pipelines, "traces")
}

func TestStatusGathererKeepsLatest(t *testing.T) {
	g := NewStatus()
	id := componentstatus.NewInstanceID(component.MustNewID("batch"), component.KindProcessor)

	g.Record(id, componentstatus.NewEvent(componentstatus.StatusOK))
	g.Record(id, componentstatus.NewRecoverableErrorEvent(errors.New("transient")))

	files, err := g.Gather(context.Background(), Options{})
	require.NoError(t, err)

	var list []componentStatus
	require.NoError(t, yaml.Unmarshal(gatherToMap(t, files)["component-status.yaml"], &list))
	require.Len(t, list, 1)
	require.Equal(t, "StatusRecoverableError", list[0].Status)
}
