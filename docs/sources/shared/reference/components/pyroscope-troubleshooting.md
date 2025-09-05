---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/pyroscope-troubleshooting/
description: Shared troubleshooting guide for pyroscope components
headless: true
---

### Connection limit errors

When using `pyroscope.write` to push profiles to a `pyroscope.receive_http` component, you may encounter errors like:

```
"failed to push to endpoint" err="deadline_exceeded: context deadline exceeded"
```

This typically indicates that the receiving component has reached its TCP connection limit.

#### Diagnosing connection limit issues

1. Check the connection metrics on the `pyroscope.receive_http` component:
   - `pyroscope_receive_http_tcp_connections`: Current number of accepted TCP connections
   - `pyroscope_receive_http_tcp_connections_limit`: Maximum number of TCP connections allowed

2. If the current connections are approaching or at the limit, you need to take action.

#### Solutions

**Option A: Increase the connection limit**

Increase the `conn_limit` parameter in the `pyroscope.receive_http` configuration:

```alloy
pyroscope.receive_http "example" {
  http {
    conn_limit = 32768  // Increase from default 16384
    // ... other settings
  }
  // ... rest of configuration
}
```

**Option B: Horizontal scaling**

Deploy multiple instances of `pyroscope.receive_http` behind a load balancer to distribute the connection load across multiple receivers.