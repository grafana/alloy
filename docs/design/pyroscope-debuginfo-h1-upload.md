# Pyroscope debuginfo upload: HTTP/2 streaming to HTTP/1.1 rewrite

* Author(s): Tolya Korniltsev
* Last updated: 2026-04-13

## Abstract

The Pyroscope debuginfo upload API was a single bidirectional streaming
connect-go RPC that required HTTP/2 (h2 or h2c). This document describes the
rewrite that replaces it with three HTTP/1.1-compatible endpoints, removing the
hard dependency on HTTP/2 for debug info uploads.

## Problem

The debuginfo upload service used a single bidirectional streaming RPC:

```protobuf
service DebuginfoService {
  rpc Upload(stream UploadRequest) returns (stream UploadResponse) {}
}
```

The client sent a `ShouldInitiateUploadRequest` as the first message, received
a `ShouldInitiateUploadResponse`, and then streamed `UploadChunk` messages
containing binary data. This required HTTP/2 because connect-go bidirectional
streaming relies on it.

Depending on HTTP/2 is problematic in environments where intermediaries (load
balancers, proxies, service meshes) do not support or are not configured for
h2/h2c. Unary RPCs and plain HTTP POST requests work over HTTP/1.1 without any
special infrastructure configuration.

## Solution

The single streaming RPC was replaced with three endpoints:

### 1. ShouldInitiateUpload (connect-go unary RPC)

```
POST /debuginfo.v1alpha1.DebuginfoService/ShouldInitiateUpload
```

The client sends file metadata (GNU build ID, file type, name). The server
checks whether an upload is needed by examining existing metadata in object
storage. Possible outcomes:

| Response | Reason |
|---|---|
| `should_initiate_upload: true` | First time seen, or previous upload is stale |
| `should_initiate_upload: false` | Already uploaded, upload in progress, or service disabled |

When the response is "yes", the server immediately writes `STATE_UPLOADING`
metadata to object storage. This ensures the upload slot is reserved before any
bytes are transferred over the wire.

### 2. Upload (plain HTTP POST)

```
POST /debuginfo.v1alpha1.DebuginfoService/Upload/{gnu_build_id}
```

The client sends the raw binary as the HTTP request body. The server reads
`r.Body` directly (it is already an `io.Reader`) and streams it into object
storage. Size enforcement uses `http.MaxBytesReader`, which returns an error
when the body exceeds the configured limit, causing `bucket.Upload` to fail
mid-transfer rather than storing a truncated file.

The GNU build ID is in the URL path. The tenant ID comes from the
`X-Scope-OrgID` header, injected into context by the standard auth middleware.
The server verifies that metadata exists in `STATE_UPLOADING` before accepting
the upload (HTTP 412 otherwise).

### 3. UploadFinished (connect-go unary RPC)

```
POST /debuginfo.v1alpha1.DebuginfoService/UploadFinished
```

The client sends the GNU build ID. The server verifies:

1. Metadata exists and is in `STATE_UPLOADING`.
2. The actual object exists in the bucket.

If both checks pass, the metadata is updated to `STATE_UPLOADED` with a
`finished_at` timestamp.

## Proto changes

The proto file (`api/debuginfo/v1alpha1/debuginfo.proto`) is `v1alpha1`, so the
wire-protocol break is acceptable.

Removed messages: `UploadRequest`, `UploadResponse`, `UploadChunk`,
`UploadStrategy`, `GrpcStrategy`.

Added messages: `UploadFinishedRequest` (contains `gnu_build_id`),
`UploadFinishedResponse` (empty).

Kept unchanged: `FileMetadata`, `ShouldInitiateUploadRequest`,
`ShouldInitiateUploadResponse`, `ObjectMetadata`.

```protobuf
service DebuginfoService {
  rpc ShouldInitiateUpload(ShouldInitiateUploadRequest) returns (ShouldInitiateUploadResponse) {}
  rpc UploadFinished(UploadFinishedRequest) returns (UploadFinishedResponse) {}
}
```

## Client flow

```
Client                              Server
  |                                    |
  |--- ShouldInitiateUpload --------->|  (unary RPC)
  |<-- ShouldInitiateUploadResponse --|
  |                                    |
  |    if should_initiate_upload:      |
  |                                    |
  |--- POST /upload/{build_id} ------>|  (plain HTTP, raw body)
  |<-- 200 OK ------------------------|
  |                                    |
  |--- UploadFinished --------------->|  (unary RPC)
  |<-- UploadFinishedResponse --------|
```

## Deleted code

The `pkg/debuginfo/reader` package contained an `UploadReader` type that
bridged streaming protobuf chunks into a standard `io.Reader`. With the plain
HTTP POST approach, `r.Body` is already an `io.Reader`, making this adapter
unnecessary.

## Error handling and edge cases

**Upload without prior ShouldInitiateUpload:** The HTTP POST handler checks
that metadata exists in `STATE_UPLOADING`. If not, it returns HTTP 412
(Precondition Failed).

**UploadFinished without actual upload:** The handler verifies the object exists
in the bucket via `bucket.Exists`. If not, it returns `FailedPrecondition`.

**Orphaned uploads:** If a client calls ShouldInitiateUpload and uploads the
file but never calls UploadFinished, the metadata stays as `STATE_UPLOADING`.
The existing stale-upload detection handles this: after `MaxUploadDuration + 2
minutes`, a subsequent client can retry.

**Size limits:** Enforced via `http.MaxBytesReader` with the configured
`MaxUploadSize` (default 100 MB).

**Concurrent uploads for the same build ID:** Same risk as the previous
implementation. The stale-upload recovery mechanism handles it.

## Files changed

| File | Change |
|---|---|
| `api/debuginfo/v1alpha1/debuginfo.proto` | New unary RPCs, removed streaming RPC and unused messages |
| `api/gen/proto/go/debuginfo/v1alpha1/*` | Regenerated |
| `pkg/debuginfo/store.go` | Three methods replace one: `ShouldInitiateUpload`, `UploadHTTPHandler`, `UploadFinished` |
| `pkg/debuginfo/reader/` | Deleted |
| `pkg/api/api.go` | `RegisterDebugInfo` now also registers the HTTP POST route |
| `pkg/pyroscope/modules.go` | Passes upload handler to registration |
| `pkg/debuginfo/store_test.go` | Rewritten for 3-step flow, uses HTTP/1.1 test server |

## PR

https://github.com/grafana/pyroscope/pull/5046
