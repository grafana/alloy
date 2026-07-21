FROM golang:1.26.5-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o /logpullmock .

FROM alpine:3.21
COPY --from=build /logpullmock /usr/bin/logpullmock
ENTRYPOINT ["/usr/bin/logpullmock"]
