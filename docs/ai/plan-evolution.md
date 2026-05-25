# The changes of plan wrt the time

## 24-05-26 - Project Architecture & Build Strategy

**What we thought we'd do:**
Simple flat structure with single main.go handling all concerns (HTTP, validation, execution, cleanup). Basic Docker build with everything in one stage. Use chi web framework for routing.

**What we actually did:**
Created 8-package layered architecture (config, handler, runner, sandbox, validate, status, stats, middleware) with clear separation of concerns. Implemented 3-stage Docker builds (nsjail-builder → go-builder → toolchains). Stuck with Go standard library (net/http) instead of chi.

**Why it changed:**
Realized early that monolithic structure would become unmaintainable as features scale. Multi-stage builds needed because nsjail submodule requires compilation with heavy build tools (protobuf, libnl-dev, gcc). Standard library proved sufficient for our needs—no chi overhead required. Clear package boundaries enable independent testing and future feature additions (Redis queue, metrics, auth) without refactoring.

## 25-05-26 - Security Validation Logic

**What we thought we'd do:**
Basic input validation with simple string checks for path traversal and flag injection. Minimal test coverage with a few unit tests for common cases.

**What we actually did:**Implemented comprehensive validation logic for filename and compiler flags, including edge cases and glob pattern support. Developed a robust test suite with 13+ tests covering various scenarios for both filename and flag validation.

**Why it changed:**
Security is a top priority for this project, and initial validation logic was too simplistic to cover all attack vectors. As we fleshed out the requirements, it became clear that we needed a more robust validation system to prevent common vulnerabilities. The test suite was expanded to ensure that all edge cases were covered and that the validation logic was thoroughly vetted against potential attacks. This iterative process of refining the validation logic and expanding test coverage was essential to building a secure and reliable system.
