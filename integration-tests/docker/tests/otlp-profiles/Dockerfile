FROM golang:1.27.3@sha256:f24164268fc1e741bbdc0066dbadcf1c8426c5a8489f0729c02c3905a53032fd AS build

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /otlp-profiles-sink ./integration-tests/docker/tests/otlp-profiles/sink

FROM scratch

COPY --from=build /otlp-profiles-sink /otlp-profiles-sink
ENTRYPOINT ["/otlp-profiles-sink"]
