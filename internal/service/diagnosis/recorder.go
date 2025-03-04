package diagnosis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

type recorder struct {
	logger log.Logger
}

func newRecorder(logger log.Logger) *recorder {
	return &recorder{
		logger: logger,
	}
}

type liveDebuggingDataCounter struct {
	Count  uint64 // Count is the number of spans, metrics, logs that the data represent.
	Events uint64 // Event is the number of events received by the component for this data type.
}

type liveDebuggingData struct {
	ComponentID string
	Data        map[livedebugging.DataType]liveDebuggingDataCounter
}

// TODO: support modules
func (r *recorder) record(ctx context.Context, host service.Host, window time.Duration, graphs []*graph) ([]insight, error) {
	livedebugginService, exist := host.GetService(livedebugging.ServiceName)
	if !exist {
		return nil, fmt.Errorf("livedebugging service not found")
	}
	callbackManager, _ := livedebugginService.Data().(livedebugging.CallbackManager)
	id := livedebugging.CallbackID(uuid.New().String())

	dataCh := make(chan livedebugging.Data, 1000)
	dataMap := make(map[string]liveDebuggingData)
	droppedData := false
	for _, g := range graphs {
		err := callbackManager.AddCallbackMulti(id, livedebugging.ModuleID(g.module), func(data livedebugging.Data) {
			// Scope the data to the module
			data.ComponentID = livedebugging.ComponentID(g.module) + "/" + data.ComponentID
			select {
			case <-ctx.Done():
				return
			default:
				select {
				case dataCh <- data:
				default:
					if !droppedData {
						level.Warn(r.logger).Log("msg", "data throughput is very high, not all debugging data can be sent to the graph")
						droppedData = true
					}
				}
			}
		})
		if err != nil {
			// The reason may just be that the livedebugging service is not enabled, which is fine.
			level.Info(r.logger).Log("msg", "not recording diagnosis data", "reason", err)
			return nil, err
		}
	}

	defer func() {
		close(dataCh)
		for _, g := range graphs {
			callbackManager.DeleteCallbackMulti(id, livedebugging.ModuleID(g.module))
		}
	}()

	ticker := time.NewTicker(window)
	defer ticker.Stop()

	for {
		select {
		case data := <-dataCh:
			// Aggregate incoming data
			if existing, exists := dataMap[string(data.ComponentID)]; exists {
				if counter, ok := existing.Data[data.Type]; !ok {
					existing.Data[data.Type] = liveDebuggingDataCounter{
						Count:  data.Count,
						Events: 1,
					}
				} else {
					counter.Count += data.Count
					counter.Events++
					existing.Data[data.Type] = counter
				}
			} else {
				dataMap[string(data.ComponentID)] = liveDebuggingData{
					ComponentID: string(data.ComponentID),
					Data: map[livedebugging.DataType]liveDebuggingDataCounter{
						data.Type: {
							Count:  data.Count,
							Events: 1,
						},
					},
				}
			}

		case <-ticker.C:
			insights := make([]insight, 0)
			for _, rule := range dataRules {
				for _, g := range graphs {
					insights = rule(g, dataMap, insights)
				}
			}
			return insights, nil
		case <-ctx.Done():
			level.Info(r.logger).Log("msg", "the diagnosis was interrupted")
			return nil, nil
		}
	}
}
