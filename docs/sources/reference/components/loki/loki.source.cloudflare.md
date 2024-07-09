---

canonical: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.cloudflare/
aliases:
  - ../loki.source.cloudflare/ # /docs/alloy/latest/reference/components/loki.source.cloudflare/
description: Learn about loki.source.cloudflare
title: loki.source.cloudflare
---

# loki.source.cloudflare

`loki.source.cloudflare` pulls logs from the Cloudflare Logpull API and forwards them to other `loki.*` components.

These logs contain data related to the connecting client, the request path through the Cloudflare network, and the response from the origin web server and can be useful for enriching existing logs on an origin server.

You can specify multiple `loki.source.cloudflare` components by giving them different labels.

## Usage

```alloy
loki.source.cloudflare "LABEL" {
  zone_id   = "ZONE_ID"
  api_token = "API_TOKEN"

  forward_to = RECEIVER_LIST
}
```

## Arguments

`loki.source.cloudflare` supports the following arguments:

Name                | Type                 | Description                                                                   | Default     | Required
--------------------|----------------------|-------------------------------------------------------------------------------|-------------|---------
`forward_to`        | `list(LogsReceiver)` | List of receivers to send log entries to.                                     |             | yes
`api_token`         | `string`             | The API token to authenticate with.                                           |             | yes
`zone_id`           | `string`             | The Cloudflare zone ID to use.                                                |             | yes
`labels`            | `map(string)`        | The labels to associate with incoming log entries.                            | `{}`        | no
`workers`           | `int`                | The number of workers to use for parsing logs.                                | `3`         | no
`pull_range`        | `duration`           | The timeframe to fetch for each pull request.                                 | `"1m"`      | no
`fields_type`       | `string`             | The set of fields to fetch for log entries.                                   | `"default"` | no
`additional_fields` | `list(string)`       | The additional list of fields to supplement those provided via `fields_type`. |             | no


By default `loki.source.cloudflare` fetches logs with the `default` set of
fields. Here are the different sets of `fields_type` available for selection,
and the fields they include:

* `default` includes:
```
"ClientIP", "ClientRequestHost", "ClientRequestMethod", "ClientRequestURI", "EdgeEndTimestamp", "EdgeResponseBytes", "EdgeRequestHost", "EdgeResponseStatus", "EdgeStartTimestamp", "RayID"
```
plus any extra fields provided via `additional_fields` argument.

* `minimal` includes all `default` fields and adds:
```
"ZoneID", "ClientSSLProtocol", "ClientRequestProtocol", "ClientRequestPath", "ClientRequestUserAgent", "ClientRequestReferer", "EdgeColoCode", "ClientCountry", "CacheCacheStatus", "CacheResponseStatus", "EdgeResponseContentType"
```
plus any extra fields provided via `additional_fields` argument.

* `extended` includes all `minimal` fields and adds:
```
"ClientSSLCipher", "ClientASN", "ClientIPClass", "CacheResponseBytes", "EdgePathingOp", "EdgePathingSrc", "EdgePathingStatus", "ParentRayID", "WorkerCPUTime", "WorkerStatus", "WorkerSubrequest", "WorkerSubrequestCount", "OriginIP", "OriginResponseStatus", "OriginSSLProtocol", "OriginResponseHTTPExpires", "OriginResponseHTTPLastModified"
```
plus any extra fields provided via `additional_fields` argument.

* `all` includes all `extended` fields and adds:
```
 "BotScore", "BotScoreSrc", "BotTags", "ClientRequestBytes", "ClientSrcPort", "ClientXRequestedWith", "CacheTieredFill", "EdgeResponseCompressionRatio", "EdgeServerIP", "FirewallMatchesSources", "FirewallMatchesActions", "FirewallMatchesRuleIDs", "OriginResponseBytes", "OriginResponseTime", "ClientDeviceType", "WAFFlags", "WAFMatchedVar", "EdgeColoID", "RequestHeaders", "ResponseHeaders", "ClientRequestSource"`
```

plus any extra fields provided via `additional_fields` argument. This is still relevant in this case if new fields are made available via Cloudflare API but are not yet included in `all`.

* `custom` includes only the fields defined in `additional_fields`.

The component saves the last successfully fetched timestamp in its positions file.
If a position is found in the file for a given zone ID, the component restarts pulling logs from that timestamp.
When no position is found, the component starts pulling logs from the current time.

Logs are fetched using multiple `workers` which request the last available `pull_range` repeatedly.
It's possible to fall behind due to having too many log lines to process for each pull.
Adding more workers, decreasing the pull range, or decreasing the quantity of fields fetched can mitigate this performance issue.

The last timestamp fetched by the component is recorded in the `loki_source_cloudflare_target_last_requested_end_timestamp` debug metric.

All incoming Cloudflare log entries are in JSON format.
You can use the `loki.process` component and a JSON processing stage to extract more labels or change the log line format.
A sample log looks like this:

```json
{
    "CacheCacheStatus": "miss",
    "CacheResponseBytes": 8377,
    "CacheResponseStatus": 200,
    "CacheTieredFill": false,
    "ClientASN": 786,
    "ClientCountry": "gb",
    "ClientDeviceType": "desktop",
    "ClientIP": "100.100.5.5",
    "ClientIPClass": "noRecord",
    "ClientRequestBytes": 2691,
    "ClientRequestHost": "www.foo.com",
    "ClientRequestMethod": "GET",
    "ClientRequestPath": "/comments/foo/",
    "ClientRequestProtocol": "HTTP/1.0",
    "ClientRequestReferer": "https://www.foo.com/foo/168855/?offset=8625",
    "ClientRequestURI": "/foo/15248108/",
    "ClientRequestUserAgent": "some bot",
    "ClientRequestSource": "1"
    "ClientSSLCipher": "ECDHE-ECDSA-AES128-GCM-SHA256",
    "ClientSSLProtocol": "TLSv1.2",
    "ClientSrcPort": 39816,
    "ClientXRequestedWith": "",
    "EdgeColoCode": "MAN",
    "EdgeColoID": 341,
    "EdgeEndTimestamp": 1637336610671000000,
    "EdgePathingOp": "wl",
    "EdgePathingSrc": "macro",
    "EdgePathingStatus": "nr",
    "EdgeRateLimitAction": "",
    "EdgeRateLimitID": 0,
    "EdgeRequestHost": "www.foo.com",
    "EdgeResponseBytes": 14878,
    "EdgeResponseCompressionRatio": 1,
    "EdgeResponseContentType": "text/html",
    "EdgeResponseStatus": 200,
    "EdgeServerIP": "8.8.8.8",
    "EdgeStartTimestamp": 1637336610517000000,
    "FirewallMatchesActions": [],
    "FirewallMatchesRuleIDs": [],
    "FirewallMatchesSources": [],
    "OriginIP": "8.8.8.8",
    "OriginResponseBytes": 0,
    "OriginResponseHTTPExpires": "",
    "OriginResponseHTTPLastModified": "",
    "OriginResponseStatus": 200,
    "OriginResponseTime": 123000000,
    "OriginSSLProtocol": "TLSv1.2",
    "ParentRayID": "00",
    "RayID": "6b0a...",
    "RequestHeaders": [],
    "ResponseHeaders": [
      "x-foo": "bar"
    ],
    "SecurityLevel": "med",
    "WAFAction": "unknown",
    "WAFFlags": "0",
    "WAFMatchedVar": "",
    "WAFProfile": "unknown",
    "WAFRuleID": "",
    "WAFRuleMessage": "",
    "WorkerCPUTime": 0,
    "WorkerStatus": "unknown",
    "WorkerSubrequest": false,
    "WorkerSubrequestCount": 0,
    "ZoneID": 1234
}
```

## Exported fields

`loki.source.cloudflare` doesn't export any fields.

## Component health

`loki.source.cloudflare` is only reported as unhealthy if given an invalid configuration.

## Debug information

`loki.source.cloudflare` exposes the following debug information:
* Whether the target is ready and reading logs from the API.
* The Cloudflare zone ID.
* The last error reported, if any.
* The stored positions file entry, as the combination of `zone_id`, labels and last fetched timestamp.
* The last timestamp fetched.
* The set of fields being fetched.

## Debug metrics
* `loki_source_cloudflare_target_entries_total` (counter): Total number of successful entries sent via the cloudflare target.
* `loki_source_cloudflare_target_last_requested_end_timestamp` (gauge): The last cloudflare request end timestamp fetched, for calculating how far behind the target is.

## Example

This example pulls logs from Cloudflare's API and forwards them to a
`loki.write` component.

```alloy
loki.source.cloudflare "dev" {
  zone_id   = env("CF_ZONE_ID")
  api_token = local.file.api.content

  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "loki:3100/api/v1/push"
  }
}
```
<!-- START GENERATED COMPATIBLE COMPONENTS -->

## Compatible components

`loki.source.cloudflare` can accept arguments from the following components:

- Components that export [Loki `LogsReceiver`](../../../compatibility/#loki-logsreceiver-exporters)


{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}

<!-- END GENERATED COMPATIBLE COMPONENTS -->
