# Git Submodule Setup: nsjail 3.4

## Complete Command Sequence

### 1. Add the submodule
```bash
git submodule add -f https://github.com/google/nsjail external/nsjail
```

**Note**: The `-f` flag is needed because `external/nsjail/` is listed in `.gitignore`. This tells Git to add it as a submodule despite the ignore rule.

### 2. Checkout tag 3.4
```bash
cd external/nsjail && git checkout tags/3.4
cd ../..
```

### 3. Verify correct pinning
```bash
# Show submodule status
git submodule status

# Output should show commit hash and "3.4" tag
# Example: 079d70dda4aa1edd9512cfd25ff1e47e316dc355 external/nsjail (3.4)

# Verify the .gitmodules file
cat .gitmodules

# Verify submodule URL
git config -f .gitmodules --get submodule.external/nsjail.url
```

### 4. Commit the submodule
```bash
git add .gitmodules external/nsjail
git commit -m "feat: add nsjail 3.4 as git submodule"
```

---

## .gitignore Modification

Remove the old line if you want the submodule in version control. Update `.gitignore`:

```diff
- external/nsjail/
```

Keep `nsjail/` if it was a build artifact directory. The submodule at `external/nsjail` should be tracked.

---

## Dockerfile Setup for Building from Submodule

### Multi-stage build approach

Replace your Dockerfile with this version that builds nsjail from the local submodule:

```dockerfile
# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.23
ARG DEBIAN_VERSION=bookworm

# ---- Build nsjail from submodule ----
FROM debian:${DEBIAN_VERSION}-slim AS nsjail-builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
        autoconf bison ca-certificates flex g++ gcc git make \
        libnl-route-3-dev libprotobuf-dev libtool pkg-config \
        protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

# Copy the submodule source
COPY external/nsjail /src/nsjail

# Build nsjail
RUN make -C /src/nsjail && \
    install -m 0755 /src/nsjail/nsjail /usr/local/bin/nsjail && \
    install -m 0644 /src/nsjail/nsjail.1 /usr/local/share/man/man1/nsjail.1

# ---- Builder stage (Go app + linters) ----
FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder

# Install runtime dependencies for nsjail
RUN apt-get update && apt-get install -y --no-install-recommends \
        libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

# Copy nsjail binary
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail

# Setup Go build
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build goboxd with static linking where possible
RUN CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags="-s -w -X main.Version=$(git describe --tags --always)" \
        -o /out/goboxd ./cmd/goboxd

# ---- Runtime image ----
FROM debian:${DEBIAN_VERSION}-slim

# Install only runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libnl-route-3-200 libprotobuf32 \
    && rm -rf /var/lib/apt/lists/*

# Copy built artifacts
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
COPY --from=builder /out/goboxd /usr/local/bin/goboxd

# Create sandbox directory
RUN mkdir -p /tmp/goboxd && chmod 1777 /tmp/goboxd

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD /usr/local/bin/goboxd -healthz || exit 1

ENTRYPOINT ["/usr/local/bin/goboxd"]
CMD ["-addr", ":8080", "-config", "config.yaml"]
```

### Key points for building from submodule:

1. **`COPY external/nsjail`** — Uses the local submodule instead of cloning from GitHub
2. **Build dependencies** — Same as nsjail's own build (proto, libnl, etc.)
3. **Multi-stage** — Keeps final image small; nsjail builder stage runs all builds
4. **Runtime deps** — Only `libnl-route-3-200` and `libprotobuf32` needed at runtime
5. **Repeatable** — Binary is built deterministically from tagged source

### Docker build commands:

```bash
# Build the image
docker build -t goboxd:latest .

# Build with specific Go version
docker build --build-arg GO_VERSION=1.21 -t goboxd:latest .

# Build and inspect layers
docker build --progress=plain -t goboxd:latest .
```

---

## Cloning with submodule for first-time users

When someone clones your repo, they need to initialize the submodule:

```bash
git clone https://github.com/yourusername/goboxd.git
cd goboxd
git submodule update --init --recursive
```

Or in one step:
```bash
git clone --recursive https://github.com/yourusername/goboxd.git
```

---

## Updating nsjail version later

When you want to update to a newer version:

```bash
cd external/nsjail
git fetch origin
git checkout tags/3.5  # Update to 3.5
cd ../..
git add external/nsjail
git commit -m "chore: update nsjail to 3.5"
```

---

## Verification Checklist

- [x] Submodule added with `-f` flag (forced)
- [x] Tag 3.4 checked out: `git describe --tags` shows `3.4`
- [x] `.gitmodules` created and configured
- [x] `git submodule status` shows correct commit hash
- [x] Dockerfile builds from `external/nsjail` (not cloning)
- [x] Runtime image includes nsjail binary at `/usr/local/bin/nsjail`
