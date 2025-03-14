package cluster_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/advertise"
	"github.com/grafana/ckit/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/alloycli"
	"github.com/grafana/alloy/internal/featuregate"
	runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/internal/service"
	httpservice "github.com/grafana/alloy/internal/service/http"
	remotecfgservice "github.com/grafana/alloy/internal/service/remotecfg"
)

func TestClusterE2E(t *testing.T) {
	tests := []struct {
		name      string
		nodeCount int
	}{
		{
			name:      "single node",
			nodeCount: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Create a list of available ports for our test nodes
			ports := getFreePorts(t, tc.nodeCount)

			_ = ctx

			// Track all peer addresses to be used for discovery
			var peerAddresses []string
			for i := 0; i < tc.nodeCount; i++ {
				peerAddresses = append(peerAddresses, fmt.Sprintf("127.0.0.1:%d", ports[i]))
			}

			wg := sync.WaitGroup{}

			for i, peerAddress := range peerAddresses {
				nodeName := fmt.Sprintf("node-%d", i)
				nodeDirPath := path.Join(t.TempDir(), nodeName)

				logger, err := logging.New(os.Stdout, logging.Options{
					Level:  logging.LevelDebug,
					Format: logging.FormatDefault,
				})
				require.NoError(t, err)
				tracer, err := tracing.New(tracing.DefaultOptions)
				require.NoError(t, err)

				reg := prometheus.NewRegistry()
				clusterService, err := alloycli.BuildClusterService(alloycli.ClusterOptions{
					Log:     log.With(logger, "service", "cluster"),
					Tracer:  tracer,
					Metrics: reg,

					EnableClustering:    true,
					NodeName:            nodeName,
					AdvertiseAddress:    peerAddress,
					ListenAddress:       peerAddress,
					JoinPeers:           peerAddresses,
					RejoinInterval:      5 * time.Second,
					AdvertiseInterfaces: advertise.DefaultInterfaces,
					ClusterMaxJoinPeers: 5,
					ClusterName:         "cluster_e2e_test",
					EnableTLS:           false,
				})
				require.NoError(t, err)

				httpService := httpservice.New(httpservice.Options{
					Logger:   logger,
					Tracer:   tracer,
					Gatherer: prometheus.DefaultGatherer,

					ReadyFunc:  func() bool { return true },
					ReloadFunc: func() (*runtime.Source, error) { return nil, nil },

					HTTPListenAddr: peerAddress,
				})

				configFilePath := path.Join(nodeDirPath, "config.alloy")
				remoteCfgService, err := remotecfgservice.New(remotecfgservice.Options{
					Logger:      log.With(logger, "service", "remotecfg"),
					ConfigPath:  configFilePath,
					StoragePath: nodeDirPath,
					Metrics:     reg,
				})
				require.NoError(t, err)

				f := runtime.New(runtime.Options{
					Logger:               logger,
					Tracer:               tracer,
					DataPath:             nodeDirPath,
					Reg:                  reg,
					MinStability:         featuregate.StabilityExperimental,
					EnableCommunityComps: false,
					Services: []service.Service{
						clusterService,
						httpService,
						remoteCfgService,
					},
				})

				wg.Add(1)
				go func() {
					defer wg.Done()
					f.Run(ctx)
				}()

				src, err := runtime.ParseSource(t.Name(), []byte(""))
				require.NoError(t, err)
				err = f.LoadSource(src, nil, configFilePath)
				require.NoError(t, err)

				err = clusterService.ChangeState(ctx, peer.StateParticipant)
				require.NoError(t, err)
			}

			wg.Wait()
		})
	}
}

// getFreePorts returns a slice of free ports that can be used for testing
func getFreePorts(t *testing.T, count int) []int {
	ports := make([]int, count)

	for i := 0; i < count; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err, "Failed to find free port")

		addr := listener.Addr().(*net.TCPAddr)
		ports[i] = addr.Port

		err = listener.Close()
		require.NoError(t, err, "Failed to close listener")
	}

	return ports
}
