FROM golang:1.25.6@sha256:ce63a16e0f7063787ebb4eb28e72d477b00b4726f79874b3205a965ffd797ab2 as build
WORKDIR /app/
COPY go.mod go.sum ./
COPY syntax/go.mod syntax/go.sum ./syntax/
RUN go mod download
COPY ./integration-tests/docker/configs/kafka/ ./
RUN CGO_ENABLED=0 go build -o main main.go
FROM alpine:3.19@sha256:6baf43584bcb78f2e5847d1de515f23499913ac9f12bdf834811a3145eb11ca1
COPY --from=build /app/main /app/main
CMD ["/app/main"]
