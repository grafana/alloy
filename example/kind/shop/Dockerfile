FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /shop .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /shop /usr/local/bin/shop
ENTRYPOINT ["shop"]
