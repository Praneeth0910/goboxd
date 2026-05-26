# goboxd Security Audit

## Hole 2: Shell-style Directory Commands

**Description:**
The original Python reference implementation dynamically creates and deletes per-request execution directories by string-formatting shell commands (e.g., executing `"rm -rf " + path` or `"mkdir -p " + path` directly in a shell). This is highly dangerous as it exposes the system to path-based command injection if unescaped variables reach the shell.

**Fix:**
In the Go implementation, we completely avoid using shell commands or invoking `exec.Command("sh", ...)` or `exec.Command("rm", ...)` for any filesystem operations. All directory and file manipulations are securely handled via Go's native standard library (which invoke the respective OS syscalls directly without any shell parsing):
- `os.MkdirTemp` and `os.MkdirAll` replaces `mkdir -p`.
- `os.RemoveAll` replaces `rm -rf`.
- `os.WriteFile` replaces shell output redirects.
- Paths are never strings formatted into systemic execution lines.

**File References:**
- `sandbox.NewJailDir` safely utilizes `os.MkdirTemp` and returns an `os.RemoveAll` closure -> `FILE:LINE`
- `runner.SweepOrphanedDirectories` safely trims stale directories purely with `os.RemoveAll` -> `FILE:LINE`
- `runner.RunSandbox` safely deposits code into the jail via `os.WriteFile` -> `FILE:LINE`
