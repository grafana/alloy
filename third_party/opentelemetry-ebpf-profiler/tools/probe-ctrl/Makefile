.PHONY: build clean lint test vuln-check

build:
	go build ./cmd/probe-ctrl

clean:
	go clean -cache -i
	rm probe-ctrl

lint:
	go tool staticcheck -checks=all ./...

test:
	go test ./...

vuln-check:
	go tool govulncheck ./...
