FROM debian:testing-20250721-slim@sha256:aaa28744f5b892a7ccc3e97c0e9b9cdd0fcc447227efaf9e54080801b990f973

WORKDIR /agent

RUN dpkg --add-architecture amd64 && dpkg --add-architecture arm64

# cross_debian_arch: amd64 or arm64
# cross_pkg_arch: x86-64 or aarch64
RUN cross_debian_arch=$(uname -m | sed -e 's/aarch64/amd64/'  -e 's/x86_64/arm64/'); \
    cross_pkg_arch=$(uname -m | sed -e 's/aarch64/x86-64/' -e 's/x86_64/aarch64/'); \
    apt-get update -y && \
    apt-get dist-upgrade -y && \
    apt-get install -y --no-install-recommends --no-install-suggests \
        curl wget make git cmake unzip libc6-dev g++ gcc pkgconf \
        llvm-17 clang-17 clang-format-17 ca-certificates \
        gcc-${cross_pkg_arch}-linux-gnu libc6-${cross_debian_arch}-cross \
        musl-dev:amd64 musl-dev:arm64 && \
    apt-get clean autoclean && \
    apt-get autoremove --yes

COPY go.mod /tmp/go.mod
# Extract Go version from go.mod
RUN GO_VERSION=$(grep -oPm1 '^go \K([[:digit:].]+)' /tmp/go.mod) && \
    GOARCH=$(uname -m) && if [ "$GOARCH" = "x86_64" ]; then GOARCH=amd64; elif [ "$GOARCH" = "aarch64" ]; then GOARCH=arm64; fi && \
    wget -qO- https://golang.org/dl/go${GO_VERSION}.linux-${GOARCH}.tar.gz | tar -C /usr/local -xz

# Set Go environment variables
ENV GOPATH="/agent/go"
ENV GOCACHE="/agent/.cache"
ENV PATH="/usr/local/go/bin:$PATH"

# gRPC dependencies
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

RUN                                                                                \
  PB_URL="https://github.com/protocolbuffers/protobuf/releases/download/v24.4/";   \
  PB_FILE="protoc-24.4-linux-x86_64.zip";                                      \
  INSTALL_DIR="/usr/local";                                                        \
                                                                                   \
  wget -nv "$PB_URL/$PB_FILE"                                                       \
    && unzip "$PB_FILE" -d "$INSTALL_DIR" 'bin/*' 'include/*'                      \
    && chmod +xr "$INSTALL_DIR/bin/protoc"                                         \
    && find "$INSTALL_DIR/include" -type d -exec chmod +x {} \;                    \
    && find "$INSTALL_DIR/include" -type f -exec chmod +r {} \;                    \
    && rm "$PB_FILE"

# Append to /etc/profile for login shells
RUN echo 'export PATH="/usr/local/go/bin:$PATH"' >> /etc/profile
RUN echo 'export PATH="/agent/go/bin:$PATH"' >> /etc/profile

# Create rust related directories in /usr/local
RUN mkdir -p /usr/local/cargo /usr/local/rustup

# Set environment variable before rustup installation
ENV CARGO_HOME=/usr/local/cargo
ENV RUSTUP_HOME=/usr/local/rustup

# Install rustup and cargo
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal --default-toolchain 1.77

# Add rust related environment variables
RUN echo 'export PATH="/usr/local/cargo/bin:$PATH"' >> /etc/profile     \
    && echo 'export CARGO_HOME="/usr/local/cargo"' >> /etc/profile      \
    && echo 'export RUSTUP_HOME="/usr/local/rustup"' >> /etc/profile

# Set mode bits
RUN chmod -R a+w /usr/local/rustup      \
    && chmod -R a+w /usr/local/cargo

ENTRYPOINT ["/bin/bash", "-l", "-c"]
