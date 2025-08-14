FROM golang:1.24.6 as build
WORKDIR /app/
COPY go.mod go.sum ./
COPY syntax/go.mod syntax/go.sum ./syntax/
RUN go mod download
COPY ./internal/cmd/integration-tests/configs/kafka/ ./
RUN CGO_ENABLED=0 go build -o main main.go
FROM alpine:3.18
COPY --from=build /app/main /app/main
CMD ["/app/main"]
