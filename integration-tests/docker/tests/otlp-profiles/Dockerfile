FROM golang:1.27.2@sha256:5bc7f572bbaa98885a3a1fd9c0aa76b59e3e14e8628bfc316bbfd0c701e4818c AS build

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /otlp-profiles-sink ./integration-tests/docker/tests/otlp-profiles/sink

FROM scratch

COPY --from=build /otlp-profiles-sink /otlp-profiles-sink
ENTRYPOINT ["/otlp-profiles-sink"]
