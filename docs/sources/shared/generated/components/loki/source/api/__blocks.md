| Block | Description | Required |
| ----- | ----------- | -------- |
| [`grpc`][grpc] | Configures the gRPC server that receives requests. | no |
| [`http`][http] | Configures the HTTP server that receives requests. | no |
| [`tls`][tls] | Configures TLS for the server. | no |
| grpc > [`tls`][tls] | Configures TLS for the gRPC server. | no |
| http > [`tls`][tls] | Configures TLS for the HTTP server. | no |

[grpc]: #grpc
[http]: #http
[tls]: #tls
[tls]: #tls
[tls]: #tls
