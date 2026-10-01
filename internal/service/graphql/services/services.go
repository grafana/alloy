package services

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	"github.com/grafana/ckit/peer"
)

const (
	clusterServiceName      = "cluster"
	httpServiceName         = "http"
	labelStoreServiceName   = "labelstore"
	liveDebugServiceName    = "livedebugging"
	otelServiceName         = "otel"
	remoteConfigServiceName = "remotecfg"
)

type clusterData interface {
	Peers() []peer.Peer
	Enabled() bool
}

type httpData interface {
	HTTPListenAddress() string
	MemoryListenAddress() string
	ComponentBaseHTTPPath() string
}

type HTTPService = model.HTTPService
type GenericService = model.GenericService

func Cluster(host service.Host) (*model.Cluster, error) {
	svc, found := host.GetService(clusterServiceName)
	if !found {
		return nil, fmt.Errorf("cluster service not running")
	}

	clusterData, ok := svc.Data().(clusterData)
	if !ok {
		return nil, fmt.Errorf("cluster service data unavailable")
	}

	peers := clusterData.Peers()
	result := &model.Cluster{
		Enabled: clusterData.Enabled(),
		Peers:   make([]model.ClusterPeer, 0, len(peers)),
	}
	for _, p := range peers {
		result.Peers = append(result.Peers, model.ClusterPeer{
			Name:   p.Name,
			Addr:   p.Addr,
			State:  strings.ToLower(fmt.Sprint(p.State)),
			IsSelf: p.Self,
		})
	}
	return result, nil
}

func Services(host service.Host, serviceList []service.Service) []model.AlloyService {
	if len(serviceList) == 0 {
		serviceList = KnownServices(host)
	}
	result := make([]model.AlloyService, 0, len(serviceList))
	for _, svc := range serviceList {
		result = append(result, serviceModel(host, svc))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].GetName() < result[j].GetName()
	})
	return result
}

func Service(host service.Host, serviceList []service.Service, name string) model.AlloyService {
	if len(serviceList) == 0 {
		serviceList = KnownServices(host)
	}
	for _, svc := range serviceList {
		if svc.Definition().Name == name {
			return serviceModel(host, svc)
		}
	}
	return nil
}

func KnownServices(host service.Host) []service.Service {
	names := []string{
		clusterServiceName,
		httpServiceName,
		labelStoreServiceName,
		liveDebugServiceName,
		otelServiceName,
		remoteConfigServiceName,
		"ui",
	}

	result := make([]service.Service, 0, len(names))
	for _, name := range names {
		if svc, found := host.GetService(name); found {
			result = append(result, svc)
		}
	}
	return result
}

func ComponentHTTPPath(svc *model.HTTPService, componentID string) string {
	merged := path.Join(svc.BaseHTTPPath, componentID)
	if !strings.HasSuffix(merged, "/") {
		return merged + "/"
	}
	return merged
}

func serviceModel(host service.Host, svc service.Service) model.AlloyService {
	definition := svc.Definition()
	base := model.GenericService{
		Name:           definition.Name,
		Stability:      strings.Trim(definition.Stability.String(), `"`),
		DependsOn:      append([]string(nil), definition.DependsOn...),
		Consumers:      consumers(host.GetServiceConsumers(definition.Name)),
		HasRuntimeData: svc.Data() != nil,
	}

	if definition.Name == httpServiceName {
		if data, ok := svc.Data().(httpData); ok {
			return &model.HTTPService{
				Name:             base.Name,
				Stability:        base.Stability,
				DependsOn:        base.DependsOn,
				Consumers:        base.Consumers,
				HasRuntimeData:   true,
				HTTPListenAddr:   data.HTTPListenAddress(),
				MemoryListenAddr: data.MemoryListenAddress(),
				BaseHTTPPath:     data.ComponentBaseHTTPPath(),
			}
		}
	}

	return &base
}

func consumers(input []service.Consumer) []model.ServiceConsumer {
	result := make([]model.ServiceConsumer, 0, len(input))
	for _, consumer := range input {
		result = append(result, model.ServiceConsumer{
			Type: consumer.Type.String(),
			ID:   consumer.ID,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type == result[j].Type {
			return result[i].ID < result[j].ID
		}
		return result[i].Type < result[j].Type
	})
	return result
}
