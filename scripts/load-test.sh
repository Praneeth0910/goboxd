#!/bin/bash

# Load test script for goboxd using hey
# Usage: ./scripts/load-test.sh [target_url] [concurrent] [duration]

set -euo pipefail

TARGET="${1:-http://localhost:8080}"
CONCURRENT="${2:-50}"
DURATION="${3:-30s}"

echo "Starting load test..."
echo "Target: $TARGET"
echo "Concurrent: $CONCURRENT"
echo "Duration: $DURATION"

# Simple Go code to execute
READ_PAYLOAD='{
  "language": "go",
  "code": "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"hello world\") }",
  "args": ""
}'

# Run load test with hey
if command -v hey &> /dev/null; then
    echo "Using 'hey' for load testing..."
    hey -n 10000 -c "$CONCURRENT" -d "$READ_PAYLOAD" -m POST -H "Content-Type: application/json" "$TARGET/run"
elif command -v ab &> /dev/null; then
    echo "Using 'ab' (ApacheBench) for load testing..."
    ab -n 10000 -c "$CONCURRENT" -p payload.json "$TARGET/run"
else
    echo "Please install 'hey' or 'ab' for load testing"
    echo "For hey: go install github.com/rakyll/hey@latest"
    exit 1
fi

echo "Load test completed!"
