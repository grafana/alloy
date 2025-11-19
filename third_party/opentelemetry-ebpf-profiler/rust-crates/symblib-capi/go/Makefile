.PHONY: all clean

all:
	CGO_ENABLED=1 \
	go build \
	-mod=readonly \
	-ldflags='-linkmode external -extldflags=-static' \
	-trimpath \
	-tags 'static_build'


clean:
	go clean
