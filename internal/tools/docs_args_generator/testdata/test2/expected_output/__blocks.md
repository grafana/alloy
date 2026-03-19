| Block | Description | Required |
| ----- | ----------- | -------- |
| [`grpc`][grpc] | Configures the gRPC server. | no |
| [`http`][http] | Configures the gRPC server. | no |
| [`otlp`][otlp] | Configures the gRPC server. | no |
| `grpc` > [`tls`][grpc--tls] | Configures TLS for the server. | no |
| `http` > [`tls`][http--tls] | Configures TLS for the server. | no |
| `otlp` > [`tls`][otlp--tls] | Configures TLS for the server. | no |

[grpc]: #grpc
[http]: #http
[otlp]: #otlp
[grpc--tls]: #grpc--tls
[http--tls]: #http--tls
[otlp--tls]: #otlp--tls
