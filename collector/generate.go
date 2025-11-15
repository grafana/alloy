package main

//go:generate builder --config ./builder-config.yaml --skip-compilation
//go:generate go mod tidy
//go:generate go run ./generator/generator.go -- ./main.go ./main_alloy.go
