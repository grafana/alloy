package kubernetes

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	client_go "k8s.io/client-go/kubernetes"
)

type ClientBuidler interface {
	GetKubernetesClient(log.Logger, *kubernetes.ClientArguments) (client_go.Interface, error)
}

type RestClientBuidler struct{}

func (b RestClientBuidler) GetKubernetesClient(log log.Logger,
	clientArgs *kubernetes.ClientArguments) (client_go.Interface, error) {

	restConfig, err := clientArgs.BuildRESTConfig(log)
	if err != nil {
		return nil, err
	}

	client, err := client_go.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	return client, nil
}
