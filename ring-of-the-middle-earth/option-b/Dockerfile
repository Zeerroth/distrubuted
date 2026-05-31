# Builds one Go engine instance WITH the Kafka client (-tags kafka).
# confluent-kafka-go needs cgo + librdkafka, hence the debian base + dev libs.
FROM golang:1.22-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libc6-dev librdkafka-dev pkg-config && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod ./
# go.sum will be created on first `go mod tidy`; copy if present.
COPY . .
RUN go mod tidy
ENV CGO_ENABLED=1
RUN go build -tags kafka -o /out/server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    librdkafka1 ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/server /usr/local/bin/server
ENTRYPOINT ["/usr/local/bin/server"]
