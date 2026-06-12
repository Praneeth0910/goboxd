#!/usr/bin/env bash
set -euo pipefail

# Swift has no Debian package — install the official Ubuntu 22.04 tarball.
# trixie's glibc 2.41 is backward-compatible with Ubuntu 22.04's glibc 2.35.

SWIFT_VERSION="5.9.2"
ARCH="$(dpkg --print-architecture)"

case "$ARCH" in
    amd64) SWIFT_ARCH="x86_64" ;;
    arm64) SWIFT_ARCH="aarch64" ;;
    *) echo "Unsupported architecture for Swift: $ARCH"; exit 1 ;;
esac

# Runtime + build deps Swift needs that trixie-slim doesn't have yet
apt-get install -y --no-install-recommends \
    curl gnupg2 \
    libcurl4-openssl-dev libxml2 libedit2 \
    libsqlite3-0 libncurses6 libz3-4 \
    libstdc++6 zlib1g binutils

SWIFT_URL="https://download.swift.org/swift-${SWIFT_VERSION}-release/ubuntu2204/swift-${SWIFT_VERSION}-RELEASE/swift-${SWIFT_VERSION}-RELEASE-ubuntu22.04${SWIFT_ARCH:+_$SWIFT_ARCH}.tar.gz"

# x86_64 tarball name omits the arch suffix; aarch64 includes it
if [ "$SWIFT_ARCH" = "x86_64" ]; then
    SWIFT_URL="https://download.swift.org/swift-${SWIFT_VERSION}-release/ubuntu2204/swift-${SWIFT_VERSION}-RELEASE/swift-${SWIFT_VERSION}-RELEASE-ubuntu22.04.tar.gz"
fi

curl -fsSL "$SWIFT_URL" -o /tmp/swift.tar.gz
mkdir -p /opt/swift
tar -xzf /tmp/swift.tar.gz -C /opt/swift --strip-components=1
rm /tmp/swift.tar.gz

ln -sf /opt/swift/usr/bin/swift   /usr/bin/swift
ln -sf /opt/swift/usr/bin/swiftc  /usr/bin/swiftc

# In-script smoke test — fails the whole build (and the for-loop) if broken
swift --version
swiftc --version