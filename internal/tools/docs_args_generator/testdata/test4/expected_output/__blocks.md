| Block | Description | Required |
| ----- | ----------- | -------- |
| [`grpc`][server--grpc] | Configures the gRPC server. | no |
| [`http`][server--http] | Configures the HTTP server. | no |
| grpc > [`tls`][server--grpc--tls] | Configures TLS for the gRPC server. | no |
| http > [`tls`][server--http--tls] | Configures TLS for the HTTP server. | no |

[server--grpc]: #server--grpc
[server--http]: #server--http
[server--grpc--tls]: #server--grpc--tls
[server--http--tls]: #server--http--tls
