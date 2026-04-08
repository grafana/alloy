package deps

import (
	"context"
	"fmt"
)

type lokiInstaller struct{}

func NewLokiInstaller() Installer {
	return lokiInstaller{}
}

func (l lokiInstaller) Name() string {
	return "loki"
}

func (l lokiInstaller) Install(ctx context.Context, kubeconfig string) error {
	if err := applyManifest(ctx, kubeconfig, lokiManifest); err != nil {
		return err
	}
	if err := waitForDeployment(ctx, kubeconfig, defaultNamespace, "loki"); err != nil {
		return err
	}
	if err := checkServiceReadyEndpoint(
		ctx,
		kubeconfig,
		defaultNamespace,
		"loki",
		33100,
		3100,
		"http://127.0.0.1:33100/ready",
	); err != nil {
		return fmt.Errorf("dependency=%s: %w", l.Name(), err)
	}
	return nil
}

func (l lokiInstaller) Uninstall(ctx context.Context, kubeconfig string) error {
	return deleteManifest(ctx, kubeconfig, lokiManifest)
}

const lokiManifest = `
apiVersion: v1
kind: Namespace
metadata:
  name: k8s-v2-observability
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: loki-config
  namespace: k8s-v2-observability
data:
  config.yaml: |
    auth_enabled: false
    server:
      http_listen_port: 3100
      grpc_listen_port: 9096
    common:
      path_prefix: /loki
      replication_factor: 1
      ring:
        kvstore:
          store: inmemory
      storage:
        filesystem:
          chunks_directory: /loki/chunks
          rules_directory: /loki/rules
    schema_config:
      configs:
        - from: 2024-01-01
          store: tsdb
          object_store: filesystem
          schema: v13
          index:
            prefix: loki_index_
            period: 24h
    storage_config:
      tsdb_shipper:
        active_index_directory: /loki/index
        cache_location: /loki/index_cache
      filesystem:
        directory: /loki/chunks
    limits_config:
      allow_structured_metadata: false
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki
  namespace: k8s-v2-observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: loki
  template:
    metadata:
      labels:
        app: loki
    spec:
      containers:
        - name: loki
          image: grafana/loki:3.2.0
          args:
            - "-config.file=/etc/loki/config.yaml"
          ports:
            - containerPort: 3100
              name: http
          volumeMounts:
            - name: config
              mountPath: /etc/loki
      volumes:
        - name: config
          configMap:
            name: loki-config
---
apiVersion: v1
kind: Service
metadata:
  name: loki
  namespace: k8s-v2-observability
spec:
  selector:
    app: loki
  ports:
    - name: http
      port: 3100
      targetPort: 3100
`
