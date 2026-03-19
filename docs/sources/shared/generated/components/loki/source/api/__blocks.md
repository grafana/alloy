| Block | Description | Required |
| ----- | ----------- | -------- |
| [`grpc`][net--grpc] | Configures the gRPC server that receives requests. | no |
| [`http`][net--http] | Configures the HTTP server that receives requests. | no |
| [`tls`][net--tls] | Configures TLS for the server. | no |
| grpc > [`tls`][net--tls] | Configures TLS for the server. | no |
| http > [`tls`][net--tls] | Configures TLS for the server. | no |

[net--grpc]: #net--grpc
[net--http]: #net--http
[net--tls]: #net--tls
