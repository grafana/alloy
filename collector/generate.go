package main

//go:generate go run github.com/grafana/alloy/cmd/alloy-builder --ocb-version=${BUILDER_VERSION} --config ./builder-config.yaml --skip-compilation
//go:generate go run ./generator/generator.go --path ./
//go:generate go mod tidy
