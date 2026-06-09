#!/usr/bin/env bash
set -euo pipefail
# Go is copied from the go-builder stage directly via COPY --from=go-builder.
# No apt install needed. Go is verified at the end of the Dockerfile in the smoke test.
echo "Go installation is handled via COPY --from=go-builder later in the Dockerfile"
