# The changes of plan wrt the time

## 24-05-26 - Project Architecture & Build Strategy

**What we thought we'd do:**
Simple flat structure with single main.go handling all concerns (HTTP, validation, execution, cleanup). Basic Docker build with everything in one stage. Use chi web framework for routing.

**What we actually did:**
Created 8-package layered architecture (config, handler, runner, sandbox, validate, status, stats, middleware) with clear separation of concerns. Implemented 3-stage Docker builds (nsjail-builder → go-builder → toolchains). Stuck with Go standard library (net/http) instead of chi.

**Why it changed:**
Realized early that monolithic structure would become unmaintainable as features scale. Multi-stage builds needed because nsjail submodule requires compilation with heavy build tools (protobuf, libnl-dev, gcc). Standard library proved sufficient for our needs—no chi overhead required. Clear package boundaries enable independent testing and future feature additions (Redis queue, metrics, auth) without refactoring.
