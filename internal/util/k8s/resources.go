package k8s

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/backoff"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceSet deploys a set of temporary objects to a k8s test cluster and
// deletes them when Stop is called.
type ResourceSet struct {
	log        log.Logger
	kubeClient client.Client
	objects    []client.Object
}

// NewResourceSet returns a new resource set.
func NewResourceSet(l log.Logger, cluster *Cluster) *ResourceSet {
	return &ResourceSet{
		log:        l,
		kubeClient: cluster.Client(),
	}
}

// Add will read from r and deploy the resources into the cluster.
func (rs *ResourceSet) Add(ctx context.Context, r io.Reader) error {
	readObjects, err := ReadObjects(r, rs.kubeClient)
	if err != nil {
		return fmt.Errorf("error reading fixture: %w", err)
	}
	err = CreateObjects(ctx, rs.kubeClient, readObjects...)
	if err != nil {
		return err
	}

	rs.objects = append(rs.objects, readObjects...)
	return nil
}

// AddFile will open filename and deploy it into the cluster.
func (rs *ResourceSet) AddFile(ctx context.Context, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", filename, err)
	}
	defer f.Close()
	return rs.Add(ctx, f)
}

// Wait waits until all of the ResourceSet's objects can be found.
func (rs *ResourceSet) Wait(ctx context.Context) error {
	bo := backoff.New(ctx, backoff.Config{
		MinBackoff: 10 * time.Millisecond,
		MaxBackoff: 100 * time.Second,
	})

	check := func() error {
		for _, obj := range rs.objects {
			key := client.ObjectKeyFromObject(obj)

			clone := obj.DeepCopyObject().(client.Object)
			if err := rs.kubeClient.Get(ctx, key, clone); err != nil {
				return fmt.Errorf("failed to get %s: %w", key, err)
			}
		}

		return nil
	}

	for bo.Ongoing() {
		err := check()
		if err == nil {
			return nil
		}

		level.Debug(rs.log).Log("msg", "not all resources are available; waiting", "err", err)
		bo.Wait()
	}

	return bo.Err()
}

// Stop removes deployed resources from the cluster.
func (rs *ResourceSet) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for _, obj := range rs.objects {
		err := rs.kubeClient.Delete(ctx, obj)
		if err != nil {
			level.Error(rs.log).Log("msg", "failed to delete object", "obj", client.ObjectKeyFromObject(obj), "err", err)
		}
	}
}
