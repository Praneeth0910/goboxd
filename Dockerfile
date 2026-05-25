# syntax=docker/dockerfile:1.7
# Stage 1: Build nsjail from source (tag 3.4)
# Stage 2: Build Go binary
# Stage 3: Runtime with all language toolchains

# ============================================================================
# Stage 1: nsjail-builder
# ============================================================================
FROM debian:bookworm-slim AS nsjail-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    bison flex protobuf-compiler libprotobuf-dev \
    libnl-route-3-dev pkg-config g++ make git \
    && rm -rf /var/lib/apt/lists/*

COPY external/nsjail /src/nsjail
WORKDIR /src/nsjail
RUN make && install -m 0755 nsjail /usr/sbin/nsjail

# ============================================================================
# Stage 2: go-builder
# ============================================================================
FROM golang:1.22-bookworm AS go-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# version/commit passed in as build args
ARG VERSION=0.1.0
ARG COMMIT=unknown
ENV GOTOOLCHAIN=local

RUN CGO_ENABLED=0 go build \
    -ldflags="-X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /usr/local/bin/goboxd \
    ./cmd/goboxd

# ============================================================================
# Stage 3: final runtime image with language toolchains
# ============================================================================
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates \
    python3 \
    gcc g++ \
    default-jdk \
    nodejs \
    iverilog \
    libnl-route-3-200 \
    libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

# Copy binaries from build stages
COPY --from=nsjail-builder /usr/sbin/nsjail /usr/sbin/nsjail
COPY --from=go-builder /usr/local/bin/goboxd /usr/local/bin/goboxd

# Copy language config
RUN mkdir -p /etc/goboxd
COPY languages.yaml /etc/goboxd/languages.yaml

# Sandbox temp dir
RUN mkdir -p /tmp/goboxd && chmod 1777 /tmp/goboxd

ENV NSJAIL_PATH=/usr/sbin/nsjail
ENV LANGUAGES_CONFIG=/etc/goboxd/languages.yaml

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goboxd", "--config", "/etc/goboxd/languages.yaml"]
