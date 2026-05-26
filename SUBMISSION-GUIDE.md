# goboxd Stage 1 - Submission Guide

This document summarizes the three key submission artifacts and how to use them.

## 📋 Three Submission Artifacts

### 1. `.golangci.yml` - Linter Configuration

**Purpose**: Ensures code quality and security checks pass.

**Setup**:
```bash
# Install golangci-lint if not already installed
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

**Configuration highlights**:
- Go 1.22+ compatible
- Enabled: govet, errcheck, staticcheck, gosimple, ineffassign, unused, gofmt, gocritic, gosec
- Disabled: wrapcheck, exhaustive (too strict for systems code)
- Timeout: 5 minutes

**In CI/CD**: Add to your GitHub Actions or GitLab CI pipeline:
```yaml
- name: Lint
  run: golangci-lint run
```

---

### 2. `STAGE1-CHECKLIST.md` - Pre-Submission Checklist

**Purpose**: Comprehensive checklist of all tests that must pass before opening the PR.

**How to use**:
1. Open the checklist
2. Run each section in order
3. Mark items as complete
4. Open PR only when all items ✓

**Key sections**:
- Build & Docker (4 items)
- Endpoint Tests (8 items for health, success, security)
- Code Quality (4 items)
- Documentation (5 items)
- Final Checks (6 items)

**Estimated time**: 15-20 minutes to complete all checks

---

### 3. `TESTING.md` - Manual Testing Commands

**Purpose**: Exact curl commands for testing every endpoint and security boundary.

**Quick start**:
```bash
# Terminal 1: Start the service
make run

# Terminal 2: Run health checks
curl -s http://localhost:8080/healthz | jq .

# Run Python test
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "language": "py3",
    "source": "print(\"Hello, World!\")",
    "source_filename": "solution.py",
    "tests": [{"stdin": "", "expected_stdout": "Hello, World!\n"}]
  }' | jq .

# Run C++ test
# (see TESTING.md for full payload)
```

**Test coverage**:
- ✓ 3 health endpoints (/healthz, /readyz, /info)
- ✓ 4 successful execution tests (Python, C++, multiple tests)
- ✓ 5 security/validation tests (path traversal, flag injection, oversized input)
- ✓ 4 error handling tests (missing fields, invalid JSON)
- ✓ Complete workflow script at the bottom

---

## 🚀 Submission Workflow

### Step 1: Code Review (5 min)
```bash
# Format code
go fmt ./...
go mod tidy

# Check for issues
go vet ./...

# Run linter
golangci-lint run
```

### Step 2: Automated Tests (5 min)
```bash
# Run unit tests
make test

# Run integration tests
make integration
```

### Step 3: Manual Testing (10 min)
Follow the checklist in `STAGE1-CHECKLIST.md`:
- Build Docker image
- Run the container
- Execute health checks
- Test Python and C++ execution
- Verify security boundaries

### Step 4: Documentation Verification (5 min)
```bash
# Check README compliance
grep -E "(elegant|robust|seamlessly|leverage)" README.md
echo $?  # Should be 1 (no matches)

# Verify prompts.md has entries
wc -l docs/ai/prompts.md
# Should be > 50 lines with multiple dated entries
```

### Step 5: Open PR
```bash
# Create feature branch
git checkout -b team/submit-stage-1

# Commit changes
git add .
git commit -m "Stage 1: Code execution sandbox with Python and C++ support"

# Push and open PR
git push origin team/submit-stage-1
```

In the PR description, include:
- Reference to this submission guide
- Checklist completion confirmation
- Any known limitations or deferred items
- Test coverage summary

---

## ✅ Pre-Submission Commands (Copy & Paste)

Run these commands in sequence before opening the PR:

```bash
# 1. Code quality
go fmt ./...
go mod tidy
go vet ./...
golangci-lint run

# 2. Testing
make test
make integration

# 3. Docker build
docker build -t goboxd:latest .

# 4. Health check
docker run -d -p 8080:8080 --name goboxd-test goboxd:latest
sleep 2
curl -s http://localhost:8080/healthz | jq .

# 5. Python test
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(\"Hello, World!\")","source_filename":"solution.py","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  | jq '.status'
# Should output: "accepted"

# 6. C++ test
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"#include <iostream>\nint main() { std::cout << \"Hello, World!\" << std::endl; return 0; }","source_filename":"solution.cpp","artifact_filename":"solution","tests":[{"stdin":"","expected_stdout":"Hello, World!\n"}]}' \
  | jq '.status'
# Should output: "accepted"

# 7. Security test - path traversal
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language":"py3","source":"print(1)","source_filename":"../../etc/passwd","tests":[]}' \
  | jq '.error.code'
# Should output: "invalid_filename"

# 8. Security test - disallowed flag
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"language":"cpp","source":"int main(){}","source_filename":"solution.cpp","artifact_filename":"solution","build":{"flags":["-fPIC"]},"tests":[]}' \
  | jq '.error.code'
# Should output: "invalid_flag"

# 9. Cleanup
docker stop goboxd-test && docker rm goboxd-test

# 10. Documentation check
grep -i -E "(elegant|robust|seamlessly|leverage)" README.md | wc -l
# Should output: 0

# 11. Final verification
echo "✓ All checks complete - ready to submit!"
```

---

## 📦 Files Created for Submission

```
goboxd/
├── .golangci.yml           ← Linter configuration (NEW)
├── STAGE1-CHECKLIST.md     ← Pre-submission checklist (NEW)
├── TESTING.md              ← Manual test guide (NEW)
├── SUBMISSION-GUIDE.md     ← This file (NEW)
├── Dockerfile              ← Multi-stage build
├── Makefile                ← Build, test, lint targets
├── languages.yaml          ← Python 3 + C++ configuration
├── README.md               ← Project overview
├── go.mod / go.sum         ← Dependency management
└── docs/
    ├── api.md              ← API documentation
    ├── security.md         ← Security holes (all 7)
    ├── architecture.md     ← Component design
    └── ai/
        ├── prompts.md      ← Design decisions (3+ entries)
        ├── issues.md       ← Issue tracker
        └── ...
```

---

## 🔍 What Happens During Review

The review team will likely:

1. **Run the checklist** - Verify all items pass
2. **Run linter** - `golangci-lint run` must be clean
3. **Run tests** - `make test && make integration`
4. **Manual testing** - Curl commands from TESTING.md
5. **Code review** - Security, performance, readability
6. **Documentation review** - Completeness and accuracy

All of these are covered by the three artifacts you're submitting.

---

## ⚠️ Common Issues & Fixes

| Issue | Solution |
|-------|----------|
| `golangci-lint: command not found` | Run: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |
| Docker build fails | Check `Dockerfile` and ensure all dependencies are installed |
| Python test fails | Verify Python 3 is installed in the container |
| C++ test fails | Verify g++ is installed in the container |
| Port 8080 already in use | Change port in `Makefile` or kill existing process |
| Tests fail with timeout | Increase `wall_time_s` in language configuration |
| High memory usage | Check if nsjail is properly configured for cgroups |

---

## 📞 Questions Before Submission?

1. **Linter errors**: See [.golangci.yml](.golangci.yml) for configuration
2. **Test failures**: See [TESTING.md](TESTING.md) for curl commands
3. **Checklist items**: See [STAGE1-CHECKLIST.md](STAGE1-CHECKLIST.md) for detailed steps
4. **Code quality**: Run `golangci-lint run --fix` to auto-fix many issues
5. **Test coverage**: Run `go test -cover ./...` to see coverage %

---

**Status**: Ready for Stage 1 Submission  
**Created**: May 26, 2026  
**Go Version**: 1.22+  
**Docker Required**: Yes
