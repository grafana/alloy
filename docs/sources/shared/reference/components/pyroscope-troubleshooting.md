---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/pyroscope-troubleshooting/
description: Shared troubleshooting guide for pyroscope components
headless: true
---

### Connection limit errors

When using `pyroscope.write` to push profiles to a `pyroscope.receive_http` component, you may encounter errors like:

```text
"failed to push to endpoint" err="deadline_exceeded: context deadline exceeded"
```

This typically indicates that the receiving component has reached its TCP connection limit.

To resolve connection limit errors, first diagnose the issue, then apply one of the solutions.

#### Diagnose connection limit issues

1. Check the connection metrics on the `pyroscope.receive_http` component:
   - `pyroscope_receive_http_tcp_connections`: Current number of accepted TCP connections
   - `pyroscope_receive_http_tcp_connections_limit`: Maximum number of TCP connections allowed

1. If the current connections are approaching or at the limit, you need to take action.

#### Solution 1: Increase the connection limit

To increase the connection limit, increase the `conn_limit` parameter in the `pyroscope.receive_http` configuration:

```alloy
pyroscope.receive_http "example" {
  http {
    conn_limit = 32768  // Increase from default 16384
    // ... other settings
  }
  // ... rest of configuration
}
```

#### Solution 2: Horizontal scaling

To distribute the connection load across multiple receivers, deploy multiple instances of `pyroscope.receive_http` behind a load balancer.

### Timeout chain issues

When chaining multiple Pyroscope components such as `pyroscope.write` to `pyroscope.receive_http` to another `pyroscope.write`, you may encounter timeout issues that prevent retries:

```text
"failed to push to endpoint" err="deadline_exceeded: context deadline exceeded"
```

#### Understand the problem

This issue occurs when:

1. The first `pyroscope.write` component sends profiles to `pyroscope.receive_http`.
1. The `pyroscope.receive_http` component forwards profiles to another `pyroscope.write` component.
1. Both `pyroscope.write` components have the same default `remote_timeout` of 10 seconds.
1. The request context passes from the first `pyroscope.write` through `pyroscope.receive_http` to the second `pyroscope.write`, maintaining the original 10-second deadline.
1. If the second `pyroscope.write` component's downstream request takes the full 10 seconds due to a broken TCP idle connection, there's no time left for retries.

To resolve timeout chain issues, apply one of the following solutions.

#### Solution 1: Increase timeout on the first pyroscope.write

To provide buffer time for retries, increase the `remote_timeout` on the initial `pyroscope.write` component:

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

#### Solution 2: Decrease timeout on the downstream pyroscope.write

To ensure faster failures and allow time for retries, reduce the `remote_timeout` on the downstream `pyroscope.write` component:

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