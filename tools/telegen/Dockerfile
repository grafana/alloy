FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.8 AS build
ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG RELEASE_BUILD=1
ARG VERSION
ARG GOEXPERIMENT

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o telegen ./

FROM gcr.io/distroless/base-debian10
WORKDIR /app
COPY --from=build /app/telegen .
CMD ["./telegen"]
