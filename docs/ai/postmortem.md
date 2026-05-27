# goboxd Postmortem

## What went well
- **Isolation Strategy**: Moving from a Python-based shell-execution model to a robust Go-based service utilizing `nsjail` vastly improved security and resource bounds. The containerization approach ensures no direct host execution.
- **Language Extensibility**: Refactoring language definitions out of the core code and into a plug-and-play `languages.yaml` file made adding new languages (like Java) trivial without recompiling the binary.
- **Security Enhancements**: 
  - Path traversal vulnerabilities in `filename.go` were eradicated by strictly rejecting dots, path separators, and absolute paths.
  - Shell-injection vulnerabilities were removed by using native Go directory functions (`os.MkdirTemp`, `os.RemoveAll`).
  - Flag injection was mitigated via a strict prefix/suffix allowlist matching algorithm.

## What broke / What we learned
- **Memory & Resource Exhaustion (OOMs)**: Early implementations read entire `stdin` and process output directly into memory, which would crash the server for infinite loops or massive print statements. **Lesson**: All untrusted pipes must be bounded. We introduced `CapReader` and `http.MaxBytesReader` to strictly bound memory consumption.
- **Stale Jail Directories**: Initially, jail directories were generated but not always cleaned up when context timeouts or process failures occurred. This caused disk space bloat and potential leakage. **Lesson**: Introduced a dedicated startup sweep loop to clear orphaned instances by age, and rigorously ensured deferred `cleanup()` calls are always executed, even on early returns.
- **UID Collisions Under Load**: We found that random small integer UIDs for jails collided under heavy concurrent benchmarking (e.g., `hey -c 100`). **Lesson**: Replaced basic randomness with a cryptographically secure, atomic counter + PID suffix scheme (`goboxd-PID-counter-randomHex`) guaranteeing zero collisions globally.
- **Connection Drops / WSL Stability**: We ran into 'Failed parsing install script output' errors inside VS Code running over WSL due to corrupted journal files. **Lesson**: Environmental instability (unclean Windows shutdowns) impacts the development server. Wrote a `fix-wsl.sh` and documented the need to gracefully `wsl --shutdown`.

## Future Action Items
- Monitor metrics continuously under extreme loads (concurrency > 100).
- Integrate an eBPF-based network filter to replace or supplement `nsjail` network restrictions.
- Consider moving to a purely stateless ephemeral container engine if warmup latency can be kept under 50ms.
