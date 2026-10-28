FROM golang:1.27.3@sha256:6f1acc898d064a4e9d5773ee574a9af6a9b786306939ac14d7fd806bf62046db AS build

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /otlp-profiles-sink ./integration-tests/docker/tests/otlp-profiles/sink
RUN CGO_ENABLED=0 go build -o /profile-workload ./integration-tests/docker/tests/pyroscope-otlp/workload

FROM scratch

COPY --from=build /otlp-profiles-sink /otlp-profiles-sink
COPY --from=build /profile-workload /profile-workload
ENTRYPOINT ["/otlp-profiles-sink"]
