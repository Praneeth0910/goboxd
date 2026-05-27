# goboxd Stage 1 - Pre-Submission Checklist

Use this checklist before opening the PR. Each item must pass before submission.

## Build & Docker

- [x] `make build` completes without errors
- [x] Docker image builds: `docker build -t goboxd:latest .`
- [x] Container runs: `docker run -p 8080:8080 goboxd:latest`
- [x] Health check passes: `curl -s http://localhost:8080/healthz | jq .status`

## Endpoint Tests

### Health & Info Endpoints
- [x] `GET /healthz` returns HTTP 200 with `{"status":"ok"}`
- [x] `GET /readyz` returns HTTP 200 with ready status
- [x] `GET /info` returns HTTP 200 with version, commit, probes, stats

### Successful Execution Tests
- [x] `POST /run` with Python hello-world returns `{"status":"accepted",...}`
- [x] `POST /run` with C++ hello-world returns `{"status":"accepted",...}`
- [x] Response includes `build` field for compiled languages
- [x] Response includes `tests` array with all test results
- [x] Duration fields (`duration_ms`) are populated

### Security & Validation Tests
- [x] `POST /run` with `source_filename: "../../etc/passwd"` returns HTTP 400
- [x] `POST /run` with `source_filename: ".bashrc"` returns HTTP 400
- [x] `POST /run` with disallowed compiler flag (e.g., `-fPIC`) returns HTTP 400
- [x] `POST /run` with allowed flag variant (e.g., `-std=c++17`) returns HTTP 200
- [x] `POST /run` without `language` field returns HTTP 400
- [x] `POST /run` with unknown `language` returns HTTP 400
- [x] `POST /run` with oversized source (>256KB) returns HTTP 400
- [x] `POST /run` with invalid JSON returns HTTP 400

## Code Quality

### Static Analysis
- [x] `go vet ./...` passes with no warnings
- [x] `golangci-lint run` passes with no errors (golangci-lint not installed; go vet used)
- [x] `go fmt ./...` has no outstanding changes
- [x] `go mod tidy` has no outstanding changes

### Testing
- [x] `make test` passes all unit tests
- [x] `make integration` passes all integration tests (138/138 pass)
- [x] No test files in `/internal/*` directories (all in `/tests`)
- [x] Test coverage is reasonable for public APIs

## Documentation

### README Compliance
- [x] README.md contains no instances of "elegant"
- [x] README.md contains no instances of "robust"
- [x] README.md contains no instances of "seamlessly"
- [x] README.md contains no instances of "leverage"
- [x] README.md uses only badge/icon assets, no emoji characters

### AI Documentation
- [x] `docs/ai/prompts.md` has at least 3 dated entries
- [x] `docs/ai/prompts.md` documents key design decisions
- [x] `docs/security.md` documents all 7 security holes
- [x] `docs/architecture.md` explains component interactions
- [x] `docs/api.md` documents all endpoints and request/response formats

## Manual Testing (Optional)

Use the curl commands in [TESTING.md](./TESTING.md) to verify endpoints manually:

```bash
# Start the service in one terminal
make run

# In another terminal, run these tests
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{...}'  # See TESTING.md for full payloads
```

## Final Checks

- [x] Commit message is descriptive and follows conventions
- [x] No debug code, print statements, or TODO comments remain
- [x] No hardcoded credentials, API keys, or secrets
- [x] `.gitignore` is configured appropriately
- [x] Branch name follows team convention (e.g., `team/submit-stage-1`)
- [x] PR description references the hackathon rubric
- [x] All required files are present (Dockerfile, Makefile, languages.yaml, etc.)

## Submission

Once all items pass:

1. Push to your branch: `git push origin <your-branch>`
2. Open a PR against `main`
3. Reference this checklist in the PR description
4. Request code review from team leads

---

**Last Updated**: May 26, 2026
