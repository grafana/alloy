FROM golang:1.25.7 as build
WORKDIR /app/
COPY go.mod go.sum ./
COPY syntax/go.mod syntax/go.sum ./syntax/
RUN go mod download
COPY ./integration-tests/docker/configs/prom-gen/ ./
RUN CGO_ENABLED=0 go build -o main main.go
FROM alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659
COPY --from=build /app/main /app/main
CMD ["/app/main"]
