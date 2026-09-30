# Remote configuration tunnel URL

## Context

Fleet Management currently serves the remote configuration API and tunnel API from separate
binaries. Alloy therefore needs a tunnel endpoint that can differ from the existing remote
configuration endpoint.

## Configuration

Add an optional `tunnel_url` argument to the `remotecfg` block.

- `url` remains the endpoint for collector registration and configuration polling.
- `tunnel_url`, when set, is the endpoint for the tunnel connection.
- When `tunnel_url` is omitted, the tunnel uses `url` for backward compatibility.
- Setting `tunnel_url` without setting `url` is invalid. The tunnel isn't a standalone replacement
  for remote configuration.
- Both endpoints use the existing HTTP client, proxy, TLS, and authentication configuration.

## Runtime behavior

The tunnel runner resolves an effective tunnel URL from `tunnel_url`, falling back to `url`. It
uses that URL to construct the tunnel client and includes it in connection logs.

Changes to either `url` or `tunnel_url` restart the active tunnel connection. Existing behavior for
collector ID changes and HTTP client configuration changes remains unchanged.

## Validation and errors

Configuration validation returns an error when `tunnel_url` is non-empty and `url` is empty.
Malformed or unsupported tunnel URLs continue to be reported by tunnel client construction and
retried by the existing reconnect loop.

## Testing and documentation

Extend the existing end-to-end-style tunnel test to configure distinct remote configuration and
tunnel URLs and verify that the tunnel connects to the explicit tunnel endpoint. Don't add
lower-level unit tests for this temporary integration. The public `remotecfg` reference lists
`tunnel_url`, its fallback behavior, and its dependency on `url`.
