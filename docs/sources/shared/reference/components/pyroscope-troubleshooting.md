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

### Timeout chain issues

When chaining multiple pyroscope components (e.g., `pyroscope.write` → `pyroscope.receive_http` → another `pyroscope.write`), you may encounter timeout issues that prevent retries:

```
"failed to push to endpoint" err="deadline_exceeded: context deadline exceeded"
```

#### Understanding the problem

This issue occurs when:
1. `pyroscope.write` (w1) sends profiles to `pyroscope.receive_http` (r)
2. `pyroscope.receive_http` forwards profiles to another `pyroscope.write` (w2)
3. Both `pyroscope.write` components have the same default `remote_timeout` of 10 seconds
4. The request context is passed from w1 through r to w2, maintaining the original 10-second deadline
5. If w2's downstream request takes the full 10 seconds (e.g., due to a broken TCP idle connection), there's no time left for retries

#### Solutions

**Option A: Increase timeout on the first pyroscope.write**

Increase the `remote_timeout` on the initial `pyroscope.write` component to provide buffer time for retries:

```alloy
pyroscope.write "w1" {
  endpoint {
    url = "http://pyroscope-receiver:8080"
    remote_timeout = "30s"  // Increased from default 10s
    // ... other settings
  }
  // ... rest of configuration
}
```

**Option B: Decrease timeout on the downstream pyroscope.write**

Reduce the `remote_timeout` on the downstream `pyroscope.write` component to ensure faster failures and allow time for retries:

```alloy
pyroscope.write "w2" {
  endpoint {
    url = "http://pyroscope-backend:4040"
    remote_timeout = "3s"  // Reduced from default 10s
    // ... other settings
  }
  // ... rest of configuration
}
```

#### Important considerations

- **Normal latency**: Pyroscope servers with the new architecture typically have 500-1000ms average latency for requests
- **Timeout buffer**: Always leave sufficient buffer time for retries when chaining components
- **Retry configuration**: Consider adjusting `max_backoff_retries` and backoff periods alongside timeout values
- **Monitoring**: Monitor the `pyroscope_write_latency` metric to understand actual request latencies and adjust timeouts accordingly