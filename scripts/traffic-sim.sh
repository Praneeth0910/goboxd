#!/usr/bin/env bash
# ============================================================================
# traffic-sim.sh — Simulate 20 concurrent users hitting the goboxd API
#
# Each "user" is a background subshell that sends a burst of requests using
# different languages and test cases. The script collects all results and
# prints a summary table at the end.
# ============================================================================

set -euo pipefail

API="http://localhost:8080"
NUM_USERS=20
REQUESTS_PER_USER=3       # each user sends 3 requests (different languages)
RESULTS_DIR=$(mktemp -d /tmp/goboxd-traffic-XXXXXX)
START_TIME=$(date +%s%N)

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo ""
echo -e "${BOLD}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║       goboxd Traffic Simulator — 20 Concurrent Users      ║${NC}"
echo -e "${BOLD}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# ── Verify server is alive ──────────────────────────────────────────────────
echo -ne "${CYAN}[PRE-CHECK]${NC} Checking server health... "
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${API}/healthz" 2>/dev/null || echo "000")
if [[ "$HTTP_CODE" != "200" ]]; then
    echo -e "${RED}FAILED (HTTP $HTTP_CODE)${NC}"
    echo "Server is not responding at ${API}. Start it first: make run"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# ── Fetch server info ──────────────────────────────────────────────────────
echo -ne "${CYAN}[PRE-CHECK]${NC} Fetching server info... "
INFO=$(curl -s "${API}/info" 2>/dev/null)
LANG_COUNT=$(echo "$INFO" | grep -o '"id"' | wc -l)
echo -e "${GREEN}${LANG_COUNT} languages configured${NC}"
echo ""

# ── Define payloads for different languages ─────────────────────────────────
# Each user will cycle through these

py_payload() {
    local n=$1
    cat <<EOF
{"language":"py3","source":"x = int(input())\\nprint(x * ${n})","tests":[{"stdin":"${n}\\n","expected_stdout":"$((n * n))\\n"}]}
EOF
}

cpp_payload() {
    local n=$1
    cat <<EOF
{"language":"cpp","source":"#include <iostream>\\nint main() { int x; std::cin >> x; std::cout << x * ${n} << std::endl; return 0; }","tests":[{"stdin":"${n}\\n","expected_stdout":"$((n * n))\\n"}]}
EOF
}

node_payload() {
    local n=$1
    cat <<EOF
{"language":"node","source":"const readline = require('readline');\\nconst rl = readline.createInterface({ input: process.stdin });\\nrl.on('line', (line) => { console.log(parseInt(line) * ${n}); rl.close(); });","tests":[{"stdin":"${n}\\n","expected_stdout":"$((n * n))\\n"}]}
EOF
}

bash_payload() {
    local n=$1
    cat <<EOF
{"language":"bash","source":"read x\\necho \$((x * ${n}))","tests":[{"stdin":"${n}\\n","expected_stdout":"$((n * n))\\n"}]}
EOF
}

go_payload() {
    local n=$1
    cat <<EOF
{"language":"go","source":"package main\\nimport \\"fmt\\"\\nfunc main() { var x int; fmt.Scan(&x); fmt.Println(x * ${n}) }","tests":[{"stdin":"${n}\\n","expected_stdout":"$((n * n))\\n"}]}
EOF
}

ruby_payload() {
    local n=$1
    cat <<EOF
{"language":"ruby","source":"x = gets.to_i\\nputs x * ${n}","tests":[{"stdin":"${n}\\n","expected_stdout":"$((n * n))\\n"}]}
EOF
}

# Pool of payload generators (interpreted languages are faster, mix them in)
GENERATORS=(py_payload node_payload bash_payload ruby_payload go_payload cpp_payload)

# ── Simulate a single user ──────────────────────────────────────────────────
simulate_user() {
    local user_id=$1
    local result_file="${RESULTS_DIR}/user_${user_id}.log"

    for req in $(seq 1 $REQUESTS_PER_USER); do
        local gen_idx=$(( (user_id * REQUESTS_PER_USER + req) % ${#GENERATORS[@]} ))
        local gen="${GENERATORS[$gen_idx]}"
        local n=$(( user_id + req ))
        local payload
        payload=$($gen $n)
        local lang
        lang=$(echo "$payload" | grep -o '"language":"[^"]*"' | head -1 | cut -d'"' -f4)

        local req_start
        req_start=$(date +%s%N)

        local response
        local http_code
        response=$(curl -s -w "\n%{http_code}" -X POST \
            -H "Content-Type: application/json" \
            -d "$payload" \
            "${API}/run" 2>/dev/null)

        local req_end
        req_end=$(date +%s%N)
        local duration_ms=$(( (req_end - req_start) / 1000000 ))

        http_code=$(echo "$response" | tail -1)
        local body
        body=$(echo "$response" | sed '$d')
        local status
        status=$(echo "$body" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)

        echo "user=${user_id} req=${req} lang=${lang} http=${http_code} status=${status} duration_ms=${duration_ms}" >> "$result_file"
    done
}

# ── Launch all users in parallel ────────────────────────────────────────────
echo -e "${BOLD}Launching ${NUM_USERS} concurrent users (${REQUESTS_PER_USER} requests each = $((NUM_USERS * REQUESTS_PER_USER)) total requests)...${NC}"
echo -e "Languages: Python, C++, Node.js, Bash, Go, Ruby"
echo ""

PIDS=()
for user_id in $(seq 1 $NUM_USERS); do
    simulate_user "$user_id" &
    PIDS+=($!)
done

# ── Progress bar ────────────────────────────────────────────────────────────
total=${#PIDS[@]}
echo -ne "${CYAN}[RUNNING]${NC} Waiting for all users to finish... "

# Wait for all background jobs
for pid in "${PIDS[@]}"; do
    wait "$pid" 2>/dev/null || true
done

END_TIME=$(date +%s%N)
TOTAL_DURATION_MS=$(( (END_TIME - START_TIME) / 1000000 ))

echo -e "${GREEN}DONE${NC}"
echo ""

# ── Aggregate results ──────────────────────────────────────────────────────
TOTAL_REQUESTS=0
ACCEPTED=0
WRONG=0
ERRORS=0
HTTP_ERRORS=0
TOTAL_LATENCY=0
MIN_LATENCY=999999
MAX_LATENCY=0

declare -A LANG_COUNT
declare -A LANG_ACCEPTED
declare -A LANG_TOTAL_MS

while IFS= read -r line; do
    TOTAL_REQUESTS=$((TOTAL_REQUESTS + 1))

    lang=$(echo "$line" | grep -o 'lang=[^ ]*' | cut -d= -f2)
    http=$(echo "$line" | grep -o 'http=[^ ]*' | cut -d= -f2)
    status=$(echo "$line" | grep -o 'status=[^ ]*' | cut -d= -f2)
    duration=$(echo "$line" | grep -o 'duration_ms=[^ ]*' | cut -d= -f2)

    TOTAL_LATENCY=$((TOTAL_LATENCY + duration))
    [[ $duration -lt $MIN_LATENCY ]] && MIN_LATENCY=$duration
    [[ $duration -gt $MAX_LATENCY ]] && MAX_LATENCY=$duration

    LANG_COUNT[$lang]=$(( ${LANG_COUNT[$lang]:-0} + 1 ))
    LANG_TOTAL_MS[$lang]=$(( ${LANG_TOTAL_MS[$lang]:-0} + duration ))

    if [[ "$http" != "200" ]]; then
        HTTP_ERRORS=$((HTTP_ERRORS + 1))
    elif [[ "$status" == "accepted" ]]; then
        ACCEPTED=$((ACCEPTED + 1))
        LANG_ACCEPTED[$lang]=$(( ${LANG_ACCEPTED[$lang]:-0} + 1 ))
    elif [[ "$status" == "wrong_output" ]]; then
        WRONG=$((WRONG + 1))
    else
        ERRORS=$((ERRORS + 1))
    fi
done < <(cat "${RESULTS_DIR}"/user_*.log 2>/dev/null)

AVG_LATENCY=0
if [[ $TOTAL_REQUESTS -gt 0 ]]; then
    AVG_LATENCY=$((TOTAL_LATENCY / TOTAL_REQUESTS))
fi

RPS=0
if [[ $TOTAL_DURATION_MS -gt 0 ]]; then
    RPS=$(echo "scale=1; $TOTAL_REQUESTS * 1000 / $TOTAL_DURATION_MS" | bc 2>/dev/null || echo "N/A")
fi

# ── Print results ───────────────────────────────────────────────────────────
echo -e "${BOLD}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║                    TRAFFIC SIMULATION RESULTS             ║${NC}"
echo -e "${BOLD}╠════════════════════════════════════════════════════════════╣${NC}"
echo -e "${BOLD}║${NC}  Concurrent Users:    ${CYAN}${NUM_USERS}${NC}"
echo -e "${BOLD}║${NC}  Requests per User:   ${CYAN}${REQUESTS_PER_USER}${NC}"
echo -e "${BOLD}║${NC}  Total Requests:      ${CYAN}${TOTAL_REQUESTS}${NC}"
echo -e "${BOLD}║${NC}  Total Duration:      ${CYAN}${TOTAL_DURATION_MS} ms${NC}"
echo -e "${BOLD}║${NC}  Throughput (RPS):    ${CYAN}${RPS} req/s${NC}"
echo -e "${BOLD}╠════════════════════════════════════════════════════════════╣${NC}"
echo -e "${BOLD}║${NC}                      ${BOLD}STATUS BREAKDOWN${NC}"
echo -e "${BOLD}║${NC}  ✅ Accepted:         ${GREEN}${ACCEPTED}${NC}"
echo -e "${BOLD}║${NC}  ❌ Wrong Output:     ${YELLOW}${WRONG}${NC}"
echo -e "${BOLD}║${NC}  💥 Other Errors:     ${RED}${ERRORS}${NC}"
echo -e "${BOLD}║${NC}  🚫 HTTP Errors:      ${RED}${HTTP_ERRORS}${NC}"
echo -e "${BOLD}╠════════════════════════════════════════════════════════════╣${NC}"
echo -e "${BOLD}║${NC}                      ${BOLD}LATENCY (ms)${NC}"
echo -e "${BOLD}║${NC}  Min:                 ${CYAN}${MIN_LATENCY} ms${NC}"
echo -e "${BOLD}║${NC}  Avg:                 ${CYAN}${AVG_LATENCY} ms${NC}"
echo -e "${BOLD}║${NC}  Max:                 ${CYAN}${MAX_LATENCY} ms${NC}"
echo -e "${BOLD}╠════════════════════════════════════════════════════════════╣${NC}"
echo -e "${BOLD}║${NC}                   ${BOLD}PER-LANGUAGE BREAKDOWN${NC}"
echo -e "${BOLD}║${NC}"
printf "${BOLD}║${NC}  %-10s  %6s  %8s  %10s\n" "Language" "Total" "Accepted" "Avg (ms)"
printf "${BOLD}║${NC}  %-10s  %6s  %8s  %10s\n" "--------" "-----" "--------" "--------"
for lang in $(echo "${!LANG_COUNT[@]}" | tr ' ' '\n' | sort); do
    total=${LANG_COUNT[$lang]:-0}
    accepted=${LANG_ACCEPTED[$lang]:-0}
    lang_ms=${LANG_TOTAL_MS[$lang]:-0}
    avg_ms=$((lang_ms / (total > 0 ? total : 1)))
    if [[ $accepted -eq $total ]]; then
        color=$GREEN
    else
        color=$YELLOW
    fi
    printf "${BOLD}║${NC}  %-10s  %6s  ${color}%8s${NC}  %10s\n" "$lang" "$total" "$accepted" "${avg_ms}"
done
echo -e "${BOLD}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""

# ── Show raw logs ───────────────────────────────────────────────────────────
echo -e "${CYAN}[RAW LOGS]${NC} Individual request logs saved to: ${RESULTS_DIR}/"
echo ""

# Show a sample of individual requests
echo -e "${BOLD}Sample requests (first 10):${NC}"
head -10 "${RESULTS_DIR}"/user_*.log 2>/dev/null | grep -v '^==>' | head -10
echo ""

# Cleanup
rm -rf "$RESULTS_DIR"

echo -e "${GREEN}✓ Traffic simulation complete.${NC}"
