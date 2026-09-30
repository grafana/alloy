# Remote Configuration Tunnel URL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow the `remotecfg` block to direct tunnel traffic to a separate endpoint while retaining the existing URL as a fallback.

**Architecture:** Add `tunnel_url` to `remotecfg.Arguments` and resolve an effective tunnel endpoint at the tunnel boundary. The existing HTTP client configuration and collector identity remain shared. The tunnel runner reconnects when either endpoint changes.

**Tech Stack:** Go, Alloy configuration syntax, Connect RPC, `httptest`, Markdown documentation.

## Global Constraints

- Don't create commits.
- Add only one end-to-end-style test; don't add lower-level unit tests.
- `tunnel_url` falls back to `url` when omitted.
- `tunnel_url` is invalid unless `url` is also set.
- Reuse the existing HTTP client, proxy, TLS, and authentication configuration.

---

### Task 1: Add and exercise the separate tunnel endpoint

**Files:**
- Modify: `internal/service/remotecfg/tunnel_helper_test.go`
- Modify: `internal/service/remotecfg/arguments.go`
- Modify: `internal/service/remotecfg/tunnel_client.go`
- Modify: `internal/service/remotecfg/tunnel.go`
- Modify: `docs/sources/reference/config-blocks/remotecfg.md`

**Interfaces:**
- Produces: `Arguments.TunnelURL string`
- Produces: `Arguments.getTunnelURL() string`
- Consumes: the existing `tunnelClientFactory func(Arguments) (tunnelv1connect.TunnelServiceClient, error)`

- [ ] **Step 1: Make the existing end-to-end harness configure distinct endpoints**

Update the arguments in `tunnelTestHarness.Start`:

```go
args := Arguments{
	URL:              "https://remote-config.example.com",
	TunnelURL:        h.tunnelServer.URL,
	ID:               h.collectorID,
	HTTPClientConfig: httpConfig,
}
```

This keeps `TestTunnelCarriesGraphQLRequest` as the single end-to-end-style test and proves that the
tunnel uses `tunnel_url`, because `url` doesn't point to the test tunnel server.

- [ ] **Step 2: Run the end-to-end-style test and verify it fails**

Run:

```sh
go test -race -tags=nodocker ./internal/service/remotecfg -run TestTunnelCarriesGraphQLRequest
```

Expected: compilation fails because `Arguments` has no `TunnelURL` field.

- [ ] **Step 3: Add the configuration argument, fallback, and validation**

Add the field to `Arguments`:

```go
TunnelURL        string                   `alloy:"tunnel_url,attr,optional"`
```

Add the effective endpoint method:

```go
func (a Arguments) getTunnelURL() string {
	if a.TunnelURL != "" {
		return a.TunnelURL
	}
	return a.URL
}
```

At the start of `Validate`, reject a tunnel-only configuration:

```go
if a.URL == "" && a.TunnelURL != "" {
	return fmt.Errorf("tunnel_url requires url to be set")
}
```

- [ ] **Step 4: Use the effective tunnel endpoint**

In `newTunnelClient`, parse `args.getTunnelURL()` instead of `args.URL` and pass the same effective
URL to `tunnelv1connect.NewTunnelServiceClient`.

In `tunnelRunner.Update`, include this comparison in `changed`:

```go
r.desired.TunnelURL != args.TunnelURL
```

In `runReconnectLoop`, log the effective endpoint:

```go
"url", args.getTunnelURL(),
```

Keep the existing `args.URL == ""` guard in `Run`, because a normal remote configuration URL is
required even when `tunnel_url` is set.

- [ ] **Step 5: Document `tunnel_url`**

Add `tunnel_url` to the argument table in
`docs/sources/reference/config-blocks/remotecfg.md` with this description:

```text
The address of the API used for tunnel connections. Defaults to `url`.
```

Below the existing statement about an empty `url`, explain:

```text
You can't set `tunnel_url` unless you also set `url`.
```

- [ ] **Step 6: Format and run focused verification**

Run:

```sh
gofmt -w internal/service/remotecfg/arguments.go internal/service/remotecfg/tunnel_client.go internal/service/remotecfg/tunnel.go internal/service/remotecfg/tunnel_helper_test.go
go test -race -tags=nodocker ./internal/service/remotecfg -run TestTunnelCarriesGraphQLRequest
```

Expected: `TestTunnelCarriesGraphQLRequest` passes.

- [ ] **Step 7: Run package and repository verification**

Run:

```sh
go test -race -tags=nodocker ./internal/service/remotecfg/...
make lint
git diff --check
```

Expected: every command exits successfully.
