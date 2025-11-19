symbfile format
===============

`symbfile` is our custom file format for efficiently storing large amounts of
symbol information. A symbfile is a concatenation of length- and message-type
prefixed protobuf messages. The purpose of a symbfile is to provide a mapping
from each address in an executable to the corresponding function name, file name
and line number, including support for inline functions. One or more symbfiles
always describe just one executable. Recording associations of one or more
symbfiles with an executable (e.g. via a file hash) is left to other components.

We currently use two different symbol information representations:

- **Range based records ([`RangeV1`])**\
  These map an ELF virtual address range to symbol information and a `depth` integer that
  determines the depth within an inline chain. Inline chains are flattened into
  multiple overlapping range records. To determine the inline trace for any
  given address, the user would sweep though the whole symbfile and collect all
  ranges that contain the desired address and then order the resulting range
  records by their `depth` field. This presents the ground truth for symbol
  information.
- **Return pad records ([`ReturnPadV1`])**\
  These map a single address to the symbols of a full inline trace. We generate
  such records for each instruction following a `call`. The idea here is that
  when building stack traces all but the last frame will have addresses that
  point to such return addresses. Special casing these allows the symbolization
  service to proactively insert all symbols for all non-leaf frames which then
  massively reduces the amount of frames that need to be symbolized lazily.

While the symbfile format would generally also allow mixing both record types
into a single file, we currently always generate a separate symbfile per record
kind.

More details about the format itself can be found in the documentation comments
of the [protobuf definition][symbfile-proto].

[`RangeV1`]: ./symbfile.proto#L120
[`ReturnPadV1`]: ./symbfile.proto#L212
[symbfile-proto]: ./symbfile.proto

## REST API

Symbfiles are uploaded via a REST API. The `symbtool push-symbols` command
extracts and uploads at least two symbol files ("ranges" and "returnpads") to
the symbolization service via HTTP(S). The "ranges" files that contain non-leaf
frame symbol information are uploaded via `/api/symbols-ranges`. The "return
pads" files that contain leaf frame symbol information are uploaded via
`/api/symbols-returnpads`. Symbfiles may be split and uploaded in multiple
chunks (in separate HTTP requests) for improved load balancing in the presence
of multiple symbolizer services.

### File metadata (request)

While the binary file data forms the request body, the needed file metadata is
set as HTTP headers. We use the following HTTP headers:

| Header         | Description            | Example                                                                              |
| -------------- | ---------------------- | ------------------------------------------------------------------------------------ |
| FileID         | Base64 encoded FileID  | `FileID: d--nFqkSpJIXRFeHMp_Smg`                                                     |
| FilePart       | Part number 0..N-1     | `FilePart: 1`                                                                        |
| FileParts      | Number of parts N,     | `FileParts: 5`                                                                       |
| Content-Length | Length of body         | `Content-Length: 735912`                                                             |
| Authorization  | Contains an API key    | `Authorization: APIKey QzJqQ1Q0WUI1NlR0QVl4NTlZcXg6Y0xhcFN1S2tTSXlyTFlNTUloclJvdw==` |

### Response

The response to an upload request is JSON formatted.

A successful upload sets the HTTP status code to 200. The response body looks
like this

```json
{
  "success": true,
  "status": 200
}
```

In the failure case, the HTTP status code is 4xx or 5xx. The response body
explains the failure in greater detail, for example:

```json
{
  "success": false,
  "uuid": "f1ada52e-d705-423f-a1b0-fc054eb8900e",
  "error": {
    "Code": "1000",
    "Text": "Something went wrong on our side."
  },
  "status": 400
}
```

`uuid` allows logically connecting user reports and logs: error reports from
the user that contain the UUID allow finding the logs needed for
investigation and debugging.
