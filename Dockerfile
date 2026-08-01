# Build stage
FROM golang:1.25-alpine AS builder

# Build deps
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache module downloads
COPY go.mod go.sum ./
RUN go mod download

# Build featload (Linux only)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /out/featload ./cmd/featload

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
    && addgroup -S featcache && adduser -S -G featcache featcache

COPY --from=builder /out/featload /usr/local/bin/featload

# featload runs as the featcache user; the shared memory segment is
# created in /dev/shm which is world-writable on most systems.
USER featcache

EXPOSE 0

ENTRYPOINT ["/usr/local/bin/featload"]
