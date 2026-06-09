#!/usr/bin/env bash
set -euo pipefail
# Go is copied from the go-builder stage directly via COPY --from=go-builder.
# No apt install needed. This script is a no-op marker that verifies Go is present.
go version
