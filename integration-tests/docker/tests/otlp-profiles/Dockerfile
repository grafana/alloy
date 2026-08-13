FROM golang:1.26.5@sha256:63f132d58c1f589f0dcda584933a9bb44bfda1150f1506377f5a902f34d86033 AS build

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /otlp-profiles-sink ./integration-tests/docker/tests/otlp-profiles/sink

FROM scratch

COPY --from=build /otlp-profiles-sink /otlp-profiles-sink
ENTRYPOINT ["/otlp-profiles-sink"]
