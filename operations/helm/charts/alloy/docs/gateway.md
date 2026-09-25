# NGINX Gateway for Grafana Alloy

## NGINX Gateway for Grafana Alloy

The NGINX gateway provides a unified HTTP(S) entry point with path-based routing to Grafana Alloy's multiple protocol endpoints.

## Use case

When operating a centralized Grafana Alloy deployment, it listens on multiple ports for different telemetry protocols (OpenTelemetry on 4317/4318, Loki on 3100, Prometheus on 9009, etc.). In production deployments, this creates several challenges:

- **Multiple endpoints to manage**: Clients need to know and configure different endpoints for each protocol
- **TLS termination**: Need to configure TLS certificates for each port/service separately
- **Authentication**: When using multi-user basic auth, auth must be configured per protocol endpoint
- **Firewall/security rules**: Opening multiple ports increases attack surface and complexity

The NGINX gateway solves these problems by providing:

- Single entry point for all telemetry data
- Centralized TLS termination
- Unified authentication layer
- Path-based routing to different protocol backends
- Fine-grained control over proxy behavior (buffering, timeouts, connection pooling)

## Architecture

```
                                    ┌─────────────────────┐
                                    │   NGINX Gateway     │
                                    │  (Port 8080/8443)   │
                                    └──────────┬──────────┘
                                               │
                     ┌─────────────────────────┼─────────────────────────┐
                     │                         │                         │
         ┌───────────▼────────────┐ ┌─────────▼───────────┐ ┌───────────▼────────────┐
         │  /opentelemetry → 4317 │ │ /otlp/v1/logs → 4318│ │ /loki/api/v1/push →    │
         │  (OTLP gRPC)           │ │ (OTLP HTTP)         │ │      3100 (Loki)       │
         └────────────────────────┘ └─────────────────────┘ └────────────────────────┘

```

## Installation

### Basic setup

To enable the NGINX gateway with default settings:

```yaml
gateway:
  enabled: true
```

This will deploy:
- NGINX deployment with 2 replicas
- ClusterIP service on port 8080
- Default routing for OpenTelemetry, Loki, and Prometheus endpoints

### With TLS

To enable TLS:

```bash
# Create TLS secret
kubectl create secret tls alloy-gateway-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key
```

```yaml
gateway:
  enabled: true
  tls:
    enabled: true
    secretName: alloy-gateway-tls
    port: 8443
```

### With authentication

To enable basic authentication:

```bash
# Create htpasswd file
htpasswd -c .htpasswd user1
htpasswd .htpasswd user2

# Create secret
kubectl create secret generic alloy-gateway-auth \
  --from-file=auth=.htpasswd
```

```yaml
gateway:
  enabled: true
  auth:
    enabled: true
    existingSecret: alloy-gateway-auth
    realm: "Alloy Gateway"
```

### With LoadBalancer

To expose the gateway externally:

```yaml
gateway:
  enabled: true
  service:
    type: LoadBalancer
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

### With Ingress

The gateway automatically integrates with the existing Ingress configuration. When both `gateway.enabled` and `ingress.enabled` are true, the Ingress will automatically route traffic to the gateway service instead of directly to Grafana Alloy:

```yaml
gateway:
  enabled: true

ingress:
  enabled: true
  ingressClassName: nginx
  hosts:
    - alloy.example.com
  tls:
    - secretName: alloy-tls
      hosts:
        - alloy.example.com
```

This configuration creates:
- NGINX gateway handling telemetry protocol routing
- Kubernetes Ingress routing external traffic to the gateway
- Single external hostname for all telemetry data

The Ingress will route to:
- `alloy-gateway` service on port 8080 (when gateway is enabled)
- `alloy` service on port 12347 (when gateway is disabled)

## Configuration

### Upstream protocols

Configure which protocols to enable and their routing:

```yaml
gateway:
  enabled: true
  upstreams:
    # OpenTelemetry gRPC endpoint for traces
    otlpGrpc:
      enabled: true
      path: /v1/traces
      targetPort: 4317

    # OpenTelemetry HTTP endpoint for logs
    otlpHttp:
      enabled: true
      path: /v1/logs
      targetPort: 4318

    # Loki endpoint for logs
    loki:
      enabled: true
      path: /loki/api/v1/push
      targetPort: 3100

    # Prometheus remote write endpoint
    prometheus:
      enabled: true
      path: /api/v1/push
      targetPort: 9009

    # Pyroscope endpoint for continuous profiling
    pyroscope:
      enabled: true
      path: /ingest
      targetPort: 4100
```

### NGINX tuning

Adjust NGINX configuration for your workload:

```yaml
gateway:
  enabled: true
  nginxConfig:
    workerProcesses: auto
    workerConnections: 2048
    clientMaxBodySize: "50m"
    proxyConnectTimeout: "60s"
    proxySendTimeout: "120s"
    proxyReadTimeout: "120s"
    # Disable access logging for better performance in high-traffic environments
    enableAccessLog: false
    # Custom log format (only used if enableAccessLog is true)
    logFormat: '$remote_addr - $remote_user [$time_local] "$request" $status'
```

### Autoscaling

Enable horizontal pod autoscaling:

```yaml
gateway:
  enabled: true
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilizationPercentage: 80
    targetMemoryUtilizationPercentage: 80
```

### High availability

Configure for production deployments:

```yaml
gateway:
  enabled: true
  replicas: 3

  podDisruptionBudget:
    enabled: true
    minAvailable: 2

  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 1Gi

  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
            - key: app.kubernetes.io/component
              operator: In
              values:
              - gateway
          topologyKey: kubernetes.io/hostname
```

### Monitoring

Enable metrics collection with nginx-prometheus-exporter:

```yaml
gateway:
  enabled: true

  extraContainers:
  - name: nginx-exporter
    image: nginx/nginx-prometheus-exporter:1.1.0
    args:
    - -nginx.scrape-uri=http://localhost:8080/stub_status
    ports:
    - name: metrics
      containerPort: 9113
      protocol: TCP

  serviceMonitor:
    enabled: true
    additionalLabels:
      prometheus: kube-prometheus
```

### Custom NGINX configuration

Add custom NGINX directives:

```yaml
gateway:
  enabled: true
  nginxConfig:
    extraHttpConfig: |
      # Add custom HTTP block configuration
      gzip on;
      gzip_types text/plain application/json;

    extraServerConfig: |
      # Add custom server block configuration
      location /custom {
        return 200 "Custom endpoint";
      }
```

Add custom configuration per upstream:

```yaml
gateway:
  enabled: true
  upstreams:
    loki:
      enabled: true
      path: /loki/api/v1/push
      targetPort: 3100
      protocol: http
      extraConfig: |
        proxy_pass http://alloy-loki;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        # Custom headers
        proxy_set_header X-Scope-OrgID "tenant-1";
```

Add custom upstreams dynamically:

```yaml
gateway:
  enabled: true
  upstreams:
    # Add any custom upstream by creating a new key
    myCustomBackend:
      enabled: true
      path: /custom/api
      targetPort: 9000
      protocol: http
      extraConfig: |
        proxy_pass http://alloy-myCustomBackend;
        proxy_set_header X-Custom-Header "value";
```

Override entire NGINX configuration:

```yaml
gateway:
  enabled: true
  nginxConfig:
    customConfig: |
      worker_processes 4;
      events {
        worker_connections 1024;
      }
      http {
        server {
          listen {{ .Values.gateway.port }};
          location / {
            proxy_pass http://{{ include "alloy.fullname" . }}.{{ include "alloy.namespace" . }}.svc.cluster.local:12345;
          }
        }
      }
```

The `customConfig` field is passed through Helm's `tpl` function, giving you access to all template variables and functions.

## Grafana Alloy configuration

Configure Grafana Alloy to listen on the required ports:

```yaml
alloy:
  configMap:
    content: |
      // OpenTelemetry receiver
      otelcol.receiver.otlp "default" {
        grpc {
          endpoint = "0.0.0.0:4317"
        }

        http {
          endpoint = "0.0.0.0:4318"
        }

        output {
          metrics = [otelcol.exporter.prometheus.default.input]
          logs    = [otelcol.exporter.loki.default.input]
          traces  = [otelcol.exporter.otlp.default.input]
        }
      }

      // Loki receiver
      loki.source.api "default" {
        http {
          listen_address = "0.0.0.0"
          listen_port    = 3100
        }

        forward_to = [loki.write.default.receiver]
      }

  # Expose additional ports
  extraPorts:
  - name: otlp-grpc
    port: 4317
    targetPort: 4317
    protocol: TCP
  - name: otlp-http
    port: 4318
    targetPort: 4318
    protocol: TCP
  - name: loki
    port: 3100
    targetPort: 3100
    protocol: TCP
```

## Client configuration

### OpenTelemetry

Configure OpenTelemetry clients to use the gateway:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://alloy-gateway.example.com"
export OTEL_EXPORTER_OTLP_HEADERS="authorization=Basic dXNlcjpwYXNz"
```

### Loki

Configure Loki clients:

```yaml
clients:
  - url: https://alloy-gateway.example.com/loki/api/v1/push
    basic_auth:
      username: user
      password: pass
```

### Prometheus

Configure Prometheus remote write:

```yaml
remote_write:
  - url: https://alloy-gateway.example.com/api/v1/push
    basic_auth:
      username: user
      password: pass
```

## Troubleshooting

### Check gateway logs

```bash
kubectl logs -l app.kubernetes.io/component=gateway -n <namespace>
```

### Test connectivity

```bash
# Test health endpoint
curl http://alloy-gateway:8080/health

# Test with authentication
curl -u user:pass https://alloy-gateway:8443/health

# Check NGINX stub status
kubectl exec -it <gateway-pod> -- curl http://localhost:8080/stub_status
```

### Debug routing

Add debug logging to NGINX:

```yaml
gateway:
  enabled: true
  nginxConfig:
    extraHttpConfig: |
      log_format debug '$remote_addr - $remote_user [$time_local] '
                       '"$request" $status $body_bytes_sent '
                       '"$http_referer" "$http_user_agent" '
                       'upstream: $upstream_addr '
                       'upstream_status: $upstream_status '
                       'request_time: $request_time '
                       'upstream_response_time: $upstream_response_time';

      access_log /var/log/nginx/access.log debug;
```

## Performance considerations

The NGINX gateway introduces an additional network hop, which adds minimal latency (typically <1ms). For high-throughput deployments:

1. Enable connection pooling (enabled by default with keepalive)
2. Disable proxy buffering for streaming protocols (configured by default)
3. Scale gateway replicas based on CPU/memory usage
4. Use LoadBalancer or NodePort for direct external access

## Migration from Ingress

If you currently use Kubernetes Ingress for routing, you can migrate to the NGINX gateway:

**Before (Ingress):**
```yaml
ingress:
  enabled: true
  annotations:
    alb.ingress.kubernetes.io/conditions.alloy-grpc: |
      [{"field":"http-header","httpHeaderConfig":{"httpHeaderName": "Content-Type", "values":["application/grpc"]}}]
```

**After (Gateway):**
```yaml
gateway:
  enabled: true
ingress:
  enabled: true
  ingressClassName: nginx
  hosts:
    - alloy.example.com
```

Benefits of using the gateway instead of Ingress:

- Fine-grained control over proxy behavior
- Consistent configuration across cloud providers
- Built-in health checks and metrics
- Simplified authentication setup

## Complete example

Refer to `ci/gateway-values.yaml` for a complete production-ready example.
