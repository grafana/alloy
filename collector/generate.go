package main

//go:generate go run go.opentelemetry.io/collector/cmd/builder@${BUILDER_VERSION} --config ./builder-config.yaml --skip-compilation
//go:generate go run ./generator/generator.go --path ./ --config ./builder-config.yaml
//go:generate go mod tidy
