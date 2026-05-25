# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.23
ARG DEBIAN_VERSION=bookworm

# ---- Build nsjail from local submodule ----
FROM debian:${DEBIAN_VERSION}-slim AS nsjail-builder

# Install build dependencies for nsjail
RUN apt-get update && apt-get install -y --no-install-recommends \
        autoconf bison ca-certificates flex g++ gcc make \
        libnl-route-3-dev libprotobuf-dev libtool pkg-config \
        protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

# Copy the nsjail submodule source code
COPY external/nsjail /src/nsjail

# Build nsjail from source
RUN make -C /src/nsjail && \
    install -m 0755 /src/nsjail/nsjail /usr/local/bin/nsjail

# ---- Builder / dev image (Go + nsjail) ----
FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder

# Install runtime dependencies for nsjail
RUN apt-get update && apt-get install -y --no-install-recommends \
        libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

# Copy nsjail binary from builder stage
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail

# Setup Go environment
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build goboxd with static linking where possible
RUN CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/goboxd ./cmd/goboxd

# ---- Runtime image ----
FROM debian:${DEBIAN_VERSION}-slim AS runtime

# Install only runtime dependencies for nsjail
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

# Copy built artifacts from builder stages
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
COPY --from=builder        /out/goboxd          /usr/local/bin/goboxd

# Create sandbox directory with sticky bit
RUN mkdir -p /tmp/goboxd && chmod 1777 /tmp/goboxd

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD /usr/local/bin/goboxd -addr :8080 2>/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/goboxd"]
CMD ["-addr", ":8080", "-config", "config.yaml"]
