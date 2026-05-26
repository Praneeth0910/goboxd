# Stage 1: Build nsjail from source at tag 3.4
FROM debian:bookworm-slim AS nsjail-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    bison flex protobuf-compiler libprotobuf-dev \
    libnl-route-3-dev pkg-config g++ make git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Clone nsjail at tag 3.4 with kafel submodule
RUN git config --global http.sslVerify false && \
    git clone --depth=1 --branch 3.4 --recurse-submodules \
    https://github.com/google/nsjail /src/nsjail

WORKDIR /src/nsjail
RUN make && install -m 0755 nsjail /usr/sbin/nsjail

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
    -ldflags="-X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /usr/local/bin/goboxd \
    ./cmd/goboxd

# Stage 3: Runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates \
    python3 gcc g++ \
    default-jdk nodejs \
    iverilog \
    libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=nsjail-builder /usr/sbin/nsjail /usr/sbin/nsjail
COPY --from=go-builder /usr/local/bin/goboxd /usr/local/bin/goboxd

RUN mkdir -p /etc/goboxd
COPY languages.yaml /etc/goboxd/languages.yaml
RUN mkdir -p /tmp/goboxd && chmod 1777 /tmp/goboxd

ENV NSJAIL_PATH=/usr/sbin/nsjail
ENV LANGUAGES_CONFIG=/etc/goboxd/languages.yaml

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goboxd", "--config", "/etc/goboxd/languages.yaml"]
