#!/usr/bin/env bash
# =============================================================================
# run_payloads.sh — Fire JSON payloads from ./payloads/ against /run endpoint
#
# Usage:
#   ./scripts/run_payloads.sh [OPTIONS]
#
# Options:
#   -u, --url   <base_url>   Base URL (default: http://localhost:8080)
#   -l, --lang  <id>         Only run payloads for this language folder
#   -v, --verbose            Show full JSON response body
#   -h, --help               Show this help
#
# Examples:
#   ./scripts/run_payloads.sh
#   ./scripts/run_payloads.sh -l scala -v
#   ./scripts/run_payloads.sh -u http://localhost:8080 -l typescript
# =============================================================================

set -euo pipefail

# ── Colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; RESET='\033[0m'

# ── Defaults ─────────────────────────────────────────────────────────────────
BASE_URL="http://localhost:8080"
FILTER_LANG=""
VERBOSE=false
PASS=0; FAIL=0; SKIP=0

# The payloads directory is relative to the project root (where this script lives in scripts/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PAYLOADS_DIR="$(dirname "$SCRIPT_DIR")/payloads"

# ── Arg parsing ───────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -u|--url)     BASE_URL="$2"; shift 2 ;;
    -l|--lang)    FILTER_LANG="$2"; shift 2 ;;
    -v|--verbose) VERBOSE=true; shift ;;
    -h|--help)
      grep '^#' "$0" | head -20 | sed 's/^# \?//'
      exit 0 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

RUN_URL="${BASE_URL}/run"

# ── Helper: derive the expected top-level status from the filename ─────────────
expected_from_filename() {
  local fname
  fname="$(basename "$1" .json)"
  # Normalise Zone.Identifier artefacts and any non-json files
  echo "$fname"
}

# ── Helper: run a single payload file ─────────────────────────────────────────
run_file() {
  local payload_file="$1"
  local expected_status="$2"
  local label="$3"

  printf "  ${BOLD}%-52s${RESET} " "$label"

  local response http_code body actual_status
  response=$(curl -s -w '\n%{http_code}' -X POST "$RUN_URL" \
    -H "Content-Type: application/json" \
    --data-binary "@${payload_file}" 2>&1) || {
      printf "${RED}✗ CURL ERROR${RESET}\n"
      ((FAIL++)) || true; return
  }

  http_code=$(echo "$response" | tail -1)
  body=$(echo "$response" | sed '$d')

  actual_status=$(echo "$body" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4 2>/dev/null || echo "unknown")

  if [[ "$actual_status" == "$expected_status" ]]; then
    printf "${GREEN}✓ PASS${RESET} ${DIM}[HTTP %s | %s]${RESET}\n" "$http_code" "$actual_status"
    ((PASS++)) || true
  else
    printf "${RED}✗ FAIL${RESET} ${DIM}[HTTP %s | expected: %s | got: %s]${RESET}\n" \
      "$http_code" "$expected_status" "$actual_status"
    ((FAIL++)) || true
    printf "${RED}Response body:${RESET}\n"
    echo "$body" | python3 -m json.tool 2>/dev/null || echo "$body"
    echo ""
  fi

  if $VERBOSE; then
    echo "$body" | python3 -m json.tool 2>/dev/null || echo "$body"
    echo ""
  fi
}

# ── Connectivity check ────────────────────────────────────────────────────────
echo ""
printf "${BOLD}GoBoxD Payload Runner${RESET} → ${CYAN}${RUN_URL}${RESET}\n"
printf "${DIM}Payloads dir: %s${RESET}\n" "$PAYLOADS_DIR"

if ! curl -sf "${BASE_URL}/healthz" > /dev/null 2>&1; then
  printf "\n${RED}✗ Server not reachable at %s${RESET}\n" "$BASE_URL"
  echo "  Make sure the container is running: make run"
  exit 1
fi
printf "${GREEN}✓ Server is up${RESET}\n"

if [[ ! -d "$PAYLOADS_DIR" ]]; then
  printf "${RED}✗ Payloads directory not found: %s${RESET}\n" "$PAYLOADS_DIR"
  exit 1
fi

# ── Main loop — iterate each language folder ──────────────────────────────────
found_any=false

for lang_dir in "$PAYLOADS_DIR"/*/; do
  lang="$(basename "$lang_dir")"

  # Apply language filter if set
  if [[ -n "$FILTER_LANG" && "$lang" != "$FILTER_LANG" ]]; then
    continue
  fi

  # Collect only real .json files (skip Zone.Identifier streams)
  mapfile -t json_files < <(find "$lang_dir" -maxdepth 1 -name "*.json" ! -name "*:*" | sort)

  if [[ ${#json_files[@]} -eq 0 ]]; then
    continue
  fi

  found_any=true

  echo ""
  printf "${CYAN}${BOLD}══════════════════════════════════════════════════════${RESET}\n"
  printf "${CYAN}${BOLD}  %-52s${RESET}\n" "$(echo "$lang" | tr '[:lower:]' '[:upper:]')"
  printf "${CYAN}${BOLD}══════════════════════════════════════════════════════${RESET}\n"

  for payload_file in "${json_files[@]}"; do
    expected="$(expected_from_filename "$payload_file")"
    label="${lang} / $(basename "$payload_file")"
    run_file "$payload_file" "$expected" "$label"
  done
done

if ! $found_any; then
  printf "\n${YELLOW}No payload files found%s${RESET}\n" \
    "${FILTER_LANG:+ for language '$FILTER_LANG'}"
  exit 1
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
printf "${CYAN}${BOLD}══════════════════════════════════════════════════════${RESET}\n"
printf "${BOLD}  Results: ${GREEN}%d passed${RESET} | ${RED}%d failed${RESET} | ${YELLOW}%d skipped${RESET}\n" \
  "$PASS" "$FAIL" "$SKIP"
printf "${CYAN}${BOLD}══════════════════════════════════════════════════════${RESET}\n"
echo ""

[[ "$FAIL" -eq 0 ]]
