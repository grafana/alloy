FROM golang:1.22.5 as build
WORKDIR /app/
COPY go.mod go.sum ./
COPY syntax/go.mod syntax/go.sum ./syntax/
COPY internal/etc/datadogreceiver/go.mod internal/etc/datadogreceiver/go.sum ./internal/etc/datadogreceiver/
RUN go mod download
COPY ./internal/cmd/integration-tests/configs/prom-gen/ ./
RUN CGO_ENABLED=0 go build -o main main.go
FROM alpine:3.18
COPY --from=build /app/main /app/main
CMD ["/app/main"]
