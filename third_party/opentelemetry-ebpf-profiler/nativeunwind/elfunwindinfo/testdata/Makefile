.PHONY: all

BINARIES=helloworld \
	helloworld.pie \
	helloworld.stripped.pie \
	helloworld.arm64

# Use the default go executable if it is not specified otherwise.
GO_BINARY ?= go

all: $(BINARIES)

clean:
	rm -f $(BINARIES)

helloworld:
	$(GO_BINARY) build -o $@ helloworld.go

helloworld.pie:
	$(GO_BINARY) build -buildmode=pie -o $@ helloworld.go

helloworld.stripped.pie:
	$(GO_BINARY) build -buildmode=pie -ldflags="-s -w" -o $@ helloworld.go

helloworld.arm64:
	GOARCH=arm64 $(GO_BINARY) build -o $@ helloworld.go
