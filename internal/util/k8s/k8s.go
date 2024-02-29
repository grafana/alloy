// Package k8s spins up a Kubernetes cluster for testing.
package k8s

import (
	"context"
	"fmt"
	"log"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	promop_v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	apiextensions_v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Cluster is a Kubernetes cluster that runs inside of a k3s Docker container.
// Call GetConfig to retrieve a Kubernetes *rest.Config to use to connect to
// the cluster.
//
// Note that k3s uses containerd as its runtime, which means local Docker
// images are not immediately available for use. To push local images to a
// container, call PushImages. It's recommended that tests use image names that
// are not available on Docker Hub to avoid accidentally testing against the
// wrong image.
//
// Cluster should be stopped by calling Stop, otherwise running Docker
// containers will leak.
type Cluster struct {
	k3sContainer *k3s.K3sContainer
	restConfig   *rest.Config
	kubeClient   client.Client
}

// Options control creation of a cluster.
type Options struct {
	// Scheme is the Kubernetes scheme used for the generated Kubernetes client.
	// If nil, a generated scheme that contains all known Kubernetes API types
	// will be generated.
	Scheme *runtime.Scheme
}

func (o *Options) applyDefaults() error {
	if o.Scheme == nil {
		o.Scheme = runtime.NewScheme()

		for _, add := range []func(*runtime.Scheme) error{
			scheme.AddToScheme,
			apiextensions_v1.AddToScheme,
			gragent.AddToScheme,
			promop_v1.AddToScheme,
		} {
			if err := add(o.Scheme); err != nil {
				return fmt.Errorf("unable to register scheme: %w", err)
			}
		}
	}
	return nil
}

// NewCluster creates a new Cluster. NewCluster won't return with success until
// the cluster is running, but things like the ingress controller might not be
// running right away. You should never assume that any resource in the cluster
// is running and utilize exponential backoffs to allow time for things to spin
// up.
func NewCluster(ctx context.Context, o Options) (cluster *Cluster, err error) {
	if err := o.applyDefaults(); err != nil {
		return nil, fmt.Errorf("failed to apply defaults to options: %w", err)
	}

	container, err := k3s.RunContainer(ctx)
	defer func() {
		// We don't want to leak the cluster here, and we can't really be sure how
		// many resources exist, even if ClusterRun fails. If we never set our
		// cluster return argument, we'll delete the k3s cluster. This also
		// gracefully handles panics.
		if cluster == nil && container != nil {
			_ = container.Terminate(ctx)
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to run cluster: %w", err)
	}

	rawConfig, err := container.GetKubeConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	kubeClient, err := client.New(restCfg, client.Options{
		Scheme: o.Scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate client: %w", err)
	}

	return &Cluster{
		k3sContainer: container,
		restConfig:   restCfg,
		kubeClient:   kubeClient,
	}, nil
}

// Client returns the Kubernetes client for this Cluster. Client is handling
// objects registered to the Scheme passed to Options when creating the
// cluster.
func (c *Cluster) Client() client.Client {
	return c.kubeClient
}

// GetConfig returns a *rest.Config that can be used to connect to the
// Kubernetes cluster. The returned Config is a copy and is safe for
// modification.
func (c *Cluster) GetConfig() *rest.Config {
	return rest.CopyConfig(c.restConfig)
}

// Stop shuts down and deletes the cluster. Stop must be called to clean up
// created Docker resources.
func (c *Cluster) Stop() {
	err := c.k3sContainer.Terminate(context.Background())
	if err != nil {
		log.Printf("failed to shut down cluster, docker containers may have leaked: %s", err)
	}
}
