# Stage 1: Build nsjail
FROM debian:trixie-slim AS nsjail-builder
RUN apt-get update && apt-get install -y \
    autoconf bison flex gcc g++ git libprotobuf-dev libnl-route-3-dev libtool make pkg-config protobuf-compiler
WORKDIR /nsjail
RUN git clone https://github.com/google/nsjail.git . && \
    git checkout 3.4 && \
    make -j$(nproc)

# Stage 2: Build Go binary
FROM golang:1.22-bookworm AS go-builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=0.1.0
ARG COMMIT=unknown
ENV GOTOOLCHAIN=local

RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /usr/local/bin/goboxd \
    ./cmd/goboxd

# Stage 3: Runtime — trixie provides GLIBC 2.41 (nsjail requires >= 2.38)
FROM debian:trixie-slim

# Install only runtime dependencies actually needed by languages.yaml:
#   - python3: for py3 language
#   - g++: for cpp language (compile step)
#   - libnl-route-3-200, libprotobuf32t64: nsjail runtime deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates \
    python3 g++ gcc nodejs default-jdk iverilog \
    rustc kotlin \
    libnl-route-3-200 libprotobuf32t64 \
    && rm -rf /var/lib/apt/lists/*

# nsjail pinned at tag 3.4 — see .gitmodules
COPY --from=nsjail-builder /nsjail/nsjail /usr/sbin/nsjail
RUN chmod 0755 /usr/sbin/nsjail
COPY --from=go-builder /usr/local/bin/goboxd /usr/local/bin/goboxd

# Copy Go from builder stage and symlink to /usr/bin/go
COPY --from=go-builder /usr/local/go /usr/local/go
RUN ln -s /usr/local/go/bin/go /usr/bin/go && \
    ln -s /usr/local/go/bin/gofmt /usr/bin/gofmt


RUN mkdir -p /etc/goboxd
COPY languages.yaml /etc/goboxd/languages.yaml
RUN mkdir -p /tmp/goboxd && chmod 1777 /tmp/goboxd

ENV NSJAIL_PATH=/usr/sbin/nsjail
ENV LANGUAGES_CONFIG=/etc/goboxd/languages.yaml

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goboxd", "--config", "/etc/goboxd/languages.yaml"]
