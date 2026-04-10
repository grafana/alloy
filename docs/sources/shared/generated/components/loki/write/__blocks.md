| Block | Description | Required |
| ----- | ----------- | -------- |
| [`endpoint`][endpoint] | Location to send logs to. Can be specified multiple times to fan-out logs to multiple destinations. | no |
| [`wal`][wal] | Configures the Write-Ahead Log (WAL) used by the remote write client. | no |
| `endpoint` > [`authorization`][config--endpoint--authorization] | Configures generic authorization credentials for HTTP clients. | no |
| `endpoint` > [`basic_auth`][config--endpoint--basic_auth] | Configures basic authentication credentials for HTTP clients. | no |
| `endpoint` > [`oauth2`][config--endpoint--oauth2] | Configures OAuth 2.0 authentication for HTTP clients. | no |
| `endpoint` > [`queue_config`][endpoint--queue_config] | Configures the queue used to buffer batches before sending to the endpoint. | no |
| `endpoint` > [`tls_config`][config--endpoint--tls_config] | Configures TLS settings for HTTP clients. | no |
| `endpoint` > `oauth2` > [`tls_config`][config--tls_config] | Configures TLS settings for HTTP clients. | no |

[endpoint]: #endpoint
[wal]: #wal
[config--endpoint--authorization]: #config--endpoint--authorization
[config--endpoint--basic_auth]: #config--endpoint--basic_auth
[config--endpoint--oauth2]: #config--endpoint--oauth2
[endpoint--queue_config]: #endpoint--queue_config
[config--endpoint--tls_config]: #config--endpoint--tls_config
[config--tls_config]: #config--tls_config
