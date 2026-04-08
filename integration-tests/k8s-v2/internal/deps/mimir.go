package deps

import (
	"context"
	"fmt"
)

type mimirInstaller struct{}

func NewMimirInstaller() Installer {
	return mimirInstaller{}
}

func (m mimirInstaller) Name() string {
	return "mimir"
}

func (m mimirInstaller) Install(ctx context.Context, kubeconfig string) error {
	if err := applyManifest(ctx, kubeconfig, mimirManifest); err != nil {
		return err
	}
	if err := waitForDeployment(ctx, kubeconfig, defaultNamespace, "mimir"); err != nil {
		return err
	}
	if err := checkServiceReadyEndpoint(
		ctx,
		kubeconfig,
		defaultNamespace,
		"mimir",
		39009,
		9009,
		"http://127.0.0.1:39009/ready",
	); err != nil {
		return fmt.Errorf("dependency=%s: %w", m.Name(), err)
	}
	return nil
}

func (m mimirInstaller) Uninstall(ctx context.Context, kubeconfig string) error {
	return deleteManifest(ctx, kubeconfig, mimirManifest)
}

const mimirManifest = `
apiVersion: v1
kind: Namespace
metadata:
  name: k8s-v2-observability
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: mimir-config
  namespace: k8s-v2-observability
data:
  config.yaml: |
    multitenancy_enabled: false
    server:
      http_listen_port: 9009
      log_level: error
    distributor:
      ring:
        instance_addr: 127.0.0.1
        kvstore:
          store: memberlist
    ingester:
      ring:
        instance_addr: 127.0.0.1
        kvstore:
          store: memberlist
        replication_factor: 1
    blocks_storage:
      backend: filesystem
      bucket_store:
        sync_dir: /tmp/mimir/tsdb-sync
      filesystem:
        dir: /tmp/mimir/data/tsdb
      tsdb:
        dir: /tmp/mimir/tsdb
    ruler_storage:
      backend: filesystem
      filesystem:
        dir: /tmp/mimir/rules
    compactor:
      data_dir: /tmp/mimir/compactor
      sharding_ring:
        kvstore:
          store: memberlist
    store_gateway:
      sharding_ring:
        replication_factor: 1
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mimir
  namespace: k8s-v2-observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mimir
  template:
    metadata:
      labels:
        app: mimir
    spec:
      containers:
        - name: mimir
          image: grafana/mimir:2.14.2
          args:
            - "-config.file=/etc/mimir/config.yaml"
          ports:
            - containerPort: 9009
              name: http
          volumeMounts:
            - name: config
              mountPath: /etc/mimir
      volumes:
        - name: config
          configMap:
            name: mimir-config
---
apiVersion: v1
kind: Service
metadata:
  name: mimir
  namespace: k8s-v2-observability
spec:
  selector:
    app: mimir
  ports:
    - name: http
      port: 9009
      targetPort: 9009
`
