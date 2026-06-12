#!/bin/bash
set -e

ENDPOINT="http://localhost:8080/run"
PAYLOAD="$(dirname $0)/run-request.json"
DURATION=30
TIMEOUT=10s
RATES="5 10 25 50 75 100 150 200 300 400"
VEGETA="/home/praneeth_0910/go/bin/vegeta"

mkdir -p "$(dirname $0)/raw"

# Build vegeta target file inside workspace
TARGET_FILE="$(dirname $0)/raw/target.txt"
cat > "${TARGET_FILE}" << EOF
POST ${ENDPOINT}
Content-Type: application/json
@${PAYLOAD}
EOF

echo "target_rps,throughput_rps,duration_s,requests,success,failed,error_pct,p50_ms,p95_ms,p99_ms,max_ms" > "$(dirname $0)/results.csv"

for rate in $RATES; do
  echo "Running at ${rate} RPS for ${DURATION}s..."
  $VEGETA attack \
    -rate="${rate}/1s" \
    -duration="${DURATION}s" \
    -timeout="${TIMEOUT}" \
    -targets="${TARGET_FILE}" \
    > "$(dirname $0)/raw/report-${rate}.bin"

  $VEGETA report -type=json "$(dirname $0)/raw/report-${rate}.bin" \
    > "$(dirname $0)/raw/report-${rate}.json"

  # Print live summary
  $VEGETA report "$(dirname $0)/raw/report-${rate}.bin"

  jq -r --arg r "$rate" '
    [ $r,
      (.throughput | tostring),
      ((.duration/1e9) | tostring),
      (.requests | tostring),
      ((.status_codes["200"] // 0) | tostring),
      ((.requests - (.status_codes["200"] // 0)) | tostring),
      (((1 - .success) * 100) | tostring),
      ((.latencies["50th"]/1e6) | tostring),
      ((.latencies["95th"]/1e6) | tostring),
      ((.latencies["99th"]/1e6) | tostring),
      ((.latencies.max/1e6) | tostring)
    ] | @csv' \
    "$(dirname $0)/raw/report-${rate}.json" >> "$(dirname $0)/results.csv"

  echo "Done at ${rate} RPS. Sleeping 15s for recovery..."
  sleep 15
done

echo "Load test complete. Results in docs/loadtest/results.csv"
