#!/bin/bash
set -e

BASE_URL="http://localhost:8080"

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass() { echo -e "${GREEN}✅ PASS${NC}: $1"; }
fail() { echo -e "${RED}❌ FAIL${NC}: $1"; }
info() { echo -e "${YELLOW}ℹ️  INFO${NC}: $1"; }

# Check Docker
info "Checking if Docker container is running..."
if ! docker ps --filter "name=goboxd" --filter "status=running" | grep -q goboxd; then
  fail "goboxd container is NOT running"
  exit 1
else
  pass "goboxd container is running"
fi

# Health check
info "Checking /healthz..."
HEALTH=$(curl -sf "$BASE_URL/healthz" || echo '{"status":"error"}')
echo "   $HEALTH"
if echo "$HEALTH" | grep -q '"ok"'; then
  pass "/healthz is healthy"
else
  fail "/healthz returned unexpected response"
fi

# Info / language list
info "Fetching /info to verify registered languages..."
INFO=$(curl -sf "$BASE_URL/info")
echo "$INFO" | python3 -c "import sys,json; d=json.load(sys.stdin); [print('   -', l) for l in d.get('languages',[])]" 2>/dev/null || echo "   $INFO"

run_test() {
  local lang="$1"
  local source="$2"
  local description="$3"

  RESULT=$(curl -sf -X POST "$BASE_URL/run" \
    -H "Content-Type: application/json" \
    -d "{
      \"language\": \"$lang\",
      \"source\": $source,
      \"tests\": [
        {\"stdin\": \"5\n\", \"expected_stdout\": \"10\n\"},
        {\"stdin\": \"21\n\", \"expected_stdout\": \"42\n\"}
      ]
    }")

  STATUS=$(echo "$RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','unknown'))" 2>/dev/null || echo "parse_error")
  echo "   Response: $RESULT"
  if [ "$STATUS" = "accepted" ]; then
    pass "$description [$lang]: $STATUS"
  else
    fail "$description [$lang]: $STATUS"
  fi
}

echo ""
echo "=============================="
echo " Running language tests"
echo "=============================="

# OCaml
run_test "ocaml" \
  '"let () =\n  let n = int_of_string (input_line stdin) in\n  Printf.printf \"%d\\n\" (n * 2)"' \
  "OCaml"

# TypeScript
run_test "typescript" \
  '"const lines: string[] = [];\nprocess.stdin.on(\"data\", (d: Buffer) => lines.push(d.toString()));\nprocess.stdin.on(\"end\", () => {\n  const n = parseInt(lines.join(\"\").trim(), 10);\n  console.log(n * 2);\n});"' \
  "TypeScript"

# Scala
run_test "scala" \
  '"object Solution {\n  def main(args: Array[String]): Unit = {\n    val n = scala.io.StdIn.readLine().trim.toInt\n    println(n * 2)\n  }\n}"' \
  "Scala"

echo ""
echo "=============================="
echo " Done"
echo "=============================="
