#!/usr/bin/env bash
set -euo pipefail

# Install dependencies needed for downloading and extracting Node.js
apt-get install -y --no-install-recommends curl xz-utils

NODE_VERSION="20.11.0"
ARCH="$(dpkg --print-architecture)"

case "$ARCH" in
    amd64) NODE_ARCH="x64" ;;
    arm64) NODE_ARCH="arm64" ;;
    *) echo "Unsupported architecture for Node: $ARCH"; exit 1 ;;
esac

NODE_URL="https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${NODE_ARCH}.tar.xz"

curl -fsSL "$NODE_URL" -o /tmp/node.tar.xz
tar -xJf /tmp/node.tar.xz -C /usr/local --strip-components=1 --no-same-owner
rm /tmp/node.tar.xz

# Ensure /usr/bin/node points to the official Node binary
ln -sf /usr/local/bin/node /usr/bin/node

# Install TypeScript globally using the newly installed npm
npm install -g typescript