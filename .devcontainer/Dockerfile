FROM mcr.microsoft.com/vscode/devcontainers/base:ubuntu

# Install dependencies
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
    git \
    make \
    wget \
    ca-certificates \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.24.1
RUN wget -q https://go.dev/dl/go1.24.1.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz \
    && rm go1.24.1.linux-amd64.tar.gz

# Set Go environment variables
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH

# Create Go workspace directory with proper permissions
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" "$GOPATH/pkg/mod/cache/download" \
    && chmod -R 777 "$GOPATH"

# Install Grafana development tools
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest