FROM golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244 as build
WORKDIR /app/
COPY go.mod go.sum ./
COPY syntax/go.mod syntax/go.sum ./syntax/
RUN go mod download
COPY ./integration-tests/docker/configs/kafka/ ./
RUN CGO_ENABLED=0 go build -o main main.go
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
COPY --from=build /app/main /app/main
CMD ["/app/main"]
