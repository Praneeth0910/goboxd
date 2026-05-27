#!/bin/bash

# Load test script for goboxd using hey
set -euo pipefail

TARGET="${1:-http://localhost:8080/run}"

# Payload as defined in the spec
PAYLOAD='{"language":"py3","source":"print(\"hi\")","tests":[{"stdin":"","expected_stdout":"hi\n"}]}'

if ! command -v hey &> /dev/null; then
    echo "hey is not installed. Installing..."
    export PATH=$PATH:$(go env GOPATH)/bin
    go install github.com/rakyll/hey@latest
fi

export PATH=$PATH:$(go env GOPATH)/bin

TEMP_FILE=$(mktemp /tmp/goboxd-bench-XXXXXX.txt)
echo "Running benchmarks into $TEMP_FILE"

echo "# Benchmark Results" > "$TEMP_FILE"
echo "" >> "$TEMP_FILE"

for CONCURRENCY in 1 10 50 100; do
    echo "Running 200 requests with concurrency $CONCURRENCY..."
    echo "## Concurrency: $CONCURRENCY" >> "$TEMP_FILE"
    hey -n 200 -c "$CONCURRENCY" -m POST -H "Content-Type: application/json" -d "$PAYLOAD" "$TARGET" >> "$TEMP_FILE" 2>&1
    echo "" >> "$TEMP_FILE"
done

echo "Benchmarks completed. Output written to $TEMP_FILE."
cat "$TEMP_FILE"
