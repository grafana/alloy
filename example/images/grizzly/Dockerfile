FROM golang:1.22-alpine

# NOTE(rfratto): Versions of grr newer than v0.2.1 don't work with our Grizzly
# jsonnet files, reporting that the the Alloy dashboard folder "cannot be
# found" after creating it.
RUN go install github.com/grafana/grizzly/cmd/grr@v0.2.1
