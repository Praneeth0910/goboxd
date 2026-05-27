#!/usr/bin/env python3
"""
goboxd Phase 1 Submission Checker
==================================
Automated evaluator for the goboxd hackathon (SEEK × Paradox, IIT Madras).
Checks a submitted GitHub repo / local clone against the Phase 1 spec.

Usage:
    python goboxd_checker.py --repo <github-url-or-local-path> [--clone-dir /tmp/goboxd_eval]

What it checks (Phase 1 rubric):
    [A] Repo & tooling    — Dockerfile, docker-compose, Makefile, README, docs/, tests
    [B] Go project        — go.mod present, framework choice justified in README
    [C] nsjail            — pinned as submodule or fetched at image build time, tag 3.4
    [D] API contract      — POST /run, GET /healthz, GET /readyz, GET /info (live Docker test)
    [E] Languages         — at least 1 interpreted + 1 compiled end-to-end
    [F] YAML registry     — languages.yaml/config present and structurally valid
    [G] Security basics   — at least path-traversal and flag-injection mitigations visible
    [H] Commit hygiene    — not a single giant commit; meaningful messages
    [I] README quality    — no banned words, not AI-padded, short and direct

Scoring:
    Each check emits PASS / WARN / FAIL with a weight.
    Final score = weighted sum, capped at 100.
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional
import textwrap

# ──────────────────────────────────────────────
# Result primitives
# ──────────────────────────────────────────────

GREEN  = "\033[92m"
YELLOW = "\033[93m"
RED    = "\033[91m"
CYAN   = "\033[96m"
BOLD   = "\033[1m"
RESET  = "\033[0m"

@dataclass
class CheckResult:
    name: str
    status: str          # PASS | WARN | FAIL | SKIP
    detail: str
    weight: float = 1.0
    score_pct: float = 0.0   # 0–1, fraction of weight earned

    def display(self):
        icons = {"PASS": f"{GREEN}✔ PASS{RESET}",
                 "WARN": f"{YELLOW}⚠ WARN{RESET}",
                 "FAIL": f"{RED}✘ FAIL{RESET}",
                 "SKIP": f"  SKIP"}
        icon = icons.get(self.status, self.status)
        pts  = self.score_pct * self.weight
        print(f"  {icon}  [{pts:.1f}/{self.weight:.1f}]  {self.name}")
        if self.detail:
            for line in self.detail.strip().splitlines():
                print(f"           {line}")


results: list[CheckResult] = []

def record(name, status, detail="", weight=1.0, score_pct=None):
    if score_pct is None:
        score_pct = {"PASS": 1.0, "WARN": 0.5, "FAIL": 0.0, "SKIP": 0.0}[status]
    r = CheckResult(name, status, detail, weight, score_pct)
    results.append(r)
    return r


# ──────────────────────────────────────────────
# Utility helpers
# ──────────────────────────────────────────────

def run_cmd(cmd, cwd=None, timeout=60, capture=True):
    """Run a shell command, return (returncode, stdout, stderr)."""
    try:
        p = subprocess.run(
            cmd, shell=True, cwd=cwd, timeout=timeout,
            stdout=subprocess.PIPE if capture else None,
            stderr=subprocess.PIPE if capture else None,
            text=True,
        )
        return p.returncode, (p.stdout or ""), (p.stderr or "")
    except subprocess.TimeoutExpired:
        return -1, "", "TIMEOUT"
    except Exception as e:
        return -1, "", str(e)


def find_file(root: Path, *names) -> Optional[Path]:
    """Return first match of any name (case-insensitive) anywhere under root."""
    names_lower = [n.lower() for n in names]
    for p in root.rglob("*"):
        if p.name.lower() in names_lower:
            return p
    return None


def file_contains(path: Path, *patterns, case=False) -> list[str]:
    """Return which patterns appear in file."""
    if not path or not path.exists():
        return []
    text = path.read_text(errors="replace")
    if not case:
        text = text.lower()
    found = []
    for pat in patterns:
        needle = pat if case else pat.lower()
        if needle in text:
            found.append(pat)
    return found


def clone_repo(url: str, dest: Path) -> bool:
    print(f"\n{CYAN}→ Cloning {url} …{RESET}")
    rc, _, err = run_cmd(f"git clone --depth=50 {url} {dest}", timeout=120)
    if rc != 0:
        print(f"  {RED}git clone failed:{RESET} {err[:200]}")
        return False
    rc2, _, _ = run_cmd("git submodule update --init --recursive", cwd=dest, timeout=60)
    return True


def http_get(url, timeout=5):
    try:
        with urllib.request.urlopen(url, timeout=timeout) as r:
            return r.status, r.read().decode(errors="replace")
    except Exception as e:
        return 0, str(e)


# ──────────────────────────────────────────────
# Section A — Repo & Tooling
# ──────────────────────────────────────────────

def check_repo_structure(root: Path):
    print(f"\n{BOLD}[A] Repo & Tooling{RESET}")

    # Dockerfile
    df = find_file(root, "Dockerfile")
    if df:
        record("Dockerfile present", "PASS", str(df.relative_to(root)), weight=2)
    else:
        record("Dockerfile present", "FAIL", "No Dockerfile found anywhere in repo", weight=2)

    # docker-compose
    dc = find_file(root, "docker-compose.yml", "docker-compose.yaml",
                   "compose.yml", "compose.yaml")
    if dc:
        record("docker-compose present", "PASS", str(dc.relative_to(root)), weight=1)
    else:
        record("docker-compose present", "FAIL", "No docker-compose file found", weight=1)

    # Makefile with required targets
    mk = find_file(root, "Makefile")
    if not mk:
        record("Makefile present", "FAIL", "No Makefile found", weight=1.5)
    else:
        required_targets = ["build", "run", "test", "integration", "load", "lint"]
        mk_text = mk.read_text(errors="replace").lower()
        found   = [t for t in required_targets if re.search(rf"^{t}[:\s]", mk_text, re.M)]
        missing = [t for t in required_targets if t not in found]
        if not missing:
            record("Makefile targets", "PASS",
                   f"All required targets present: {', '.join(required_targets)}", weight=1.5)
        elif len(missing) <= 2:
            record("Makefile targets", "WARN",
                   f"Missing targets: {', '.join(missing)}", weight=1.5, score_pct=0.5)
        else:
            record("Makefile targets", "FAIL",
                   f"Missing targets: {', '.join(missing)}", weight=1.5, score_pct=0.2)

    # docs/ folder with expected files
    docs_dir = root / "docs"
    expected_docs = ["api.md", "languages.md", "security.md", "benchmarks.md", "architecture.md"]
    if docs_dir.exists():
        present = [f for f in expected_docs if (docs_dir / f).exists()]
        missing = [f for f in expected_docs if f not in present]
        if not missing:
            record("docs/ folder complete", "PASS",
                   f"All 5 expected docs present", weight=1)
        elif len(present) >= 3:
            record("docs/ folder complete", "WARN",
                   f"Missing: {', '.join(missing)}", weight=1, score_pct=0.5)
        else:
            record("docs/ folder complete", "FAIL",
                   f"Only {len(present)}/5 docs present. Missing: {', '.join(missing)}", weight=1, score_pct=0.2)
    else:
        record("docs/ folder complete", "FAIL", "No docs/ directory found", weight=1)

    # Unit tests
    test_files = list(root.rglob("*_test.go"))
    if len(test_files) >= 3:
        record("Unit tests present", "PASS",
               f"{len(test_files)} *_test.go files found", weight=1.5)
    elif len(test_files) >= 1:
        record("Unit tests present", "WARN",
               f"Only {len(test_files)} test file(s). Spec expects coverage of config, validation, status mapping, truncation.", weight=1.5, score_pct=0.5)
    else:
        record("Unit tests present", "FAIL", "No *_test.go files found", weight=1.5)


# ──────────────────────────────────────────────
# Section B — Go Project
# ──────────────────────────────────────────────

def check_go_project(root: Path):
    print(f"\n{BOLD}[B] Go Project{RESET}")

    gomod = find_file(root, "go.mod")
    if gomod:
        mod_text = gomod.read_text(errors="replace")
        # Extract go version
        m = re.search(r"^go\s+([\d.]+)", mod_text, re.M)
        go_ver = m.group(1) if m else "unknown"
        record("go.mod present", "PASS", f"Go version declared: {go_ver}", weight=1)
    else:
        record("go.mod present", "FAIL", "No go.mod — not a valid Go module", weight=2)
        return  # nothing else to check

    # Framework choice documented in README
    readme = find_file(root, "README.md", "readme.md")
    if readme:
        text = readme.read_text(errors="replace").lower()
        frameworks = ["net/http", "chi", "echo", "gin", "fiber", "gorilla"]
        found_fw = [f for f in frameworks if f in text]
        if found_fw:
            record("HTTP framework justified in README", "PASS",
                   f"Mentions: {', '.join(found_fw)}", weight=0.5)
        else:
            record("HTTP framework justified in README", "WARN",
                   "No HTTP framework named in README. Spec requires 2-sentence justification.", weight=0.5)
    else:
        record("README.md present", "FAIL", "No README.md found", weight=1)


# ──────────────────────────────────────────────
# Section C — nsjail
# ──────────────────────────────────────────────

def check_nsjail(root: Path):
    print(f"\n{BOLD}[C] nsjail Integration{RESET}")

    # Check for git submodule
    gitmodules = root / ".gitmodules"
    submodule_ok = False
    if gitmodules.exists():
        gm_text = gitmodules.read_text(errors="replace").lower()
        if "nsjail" in gm_text:
            submodule_ok = True
            record("nsjail as git submodule", "PASS",
                   ".gitmodules references nsjail", weight=1.5)

    # Check Dockerfile for nsjail build
    df = find_file(root, "Dockerfile")
    if df:
        df_text = df.read_text(errors="replace")
        df_lower = df_text.lower()
        has_nsjail_build = "nsjail" in df_lower and (
            "make" in df_lower or "cmake" in df_lower or "build" in df_lower
        )
        # Check for pinned tag 3.4
        tag_pinned = "3.4" in df_text or "v3.4" in df_text.lower()
        # Check they're NOT using apt/package manager
        apt_nsjail = bool(re.search(r"apt.*install.*nsjail|install.*nsjail.*apt", df_lower))

        if apt_nsjail:
            record("nsjail built from source (not apt)", "FAIL",
                   "Dockerfile installs nsjail via apt. Spec requires building from source at tag 3.4.", weight=2)
        elif has_nsjail_build:
            if tag_pinned:
                record("nsjail built from source, tag 3.4", "PASS",
                       "Dockerfile builds nsjail from source and references 3.4", weight=2)
            else:
                record("nsjail built from source, tag 3.4", "WARN",
                       "nsjail built from source but tag 3.4 not explicitly pinned", weight=2, score_pct=0.6)
        else:
            if not submodule_ok:
                record("nsjail in Dockerfile", "FAIL",
                       "No nsjail build step found in Dockerfile. Must build from source.", weight=2)
            else:
                record("nsjail referenced in Dockerfile", "WARN",
                       "nsjail submodule present but Dockerfile build step unclear", weight=2, score_pct=0.5)
    else:
        record("nsjail check", "SKIP", "No Dockerfile to inspect", weight=2)

    # Check for nsjail usage in Go code
    go_files = list(root.rglob("*.go"))
    nsjail_refs = []
    for gf in go_files:
        try:
            text = gf.read_text(errors="replace")
            if "nsjail" in text.lower():
                nsjail_refs.append(str(gf.relative_to(root)))
        except:
            pass
    if nsjail_refs:
        record("nsjail invoked in Go code", "PASS",
               f"References in: {', '.join(nsjail_refs[:3])}", weight=1)
    else:
        record("nsjail invoked in Go code", "FAIL",
               "No Go source file references nsjail. Core of the project is missing.", weight=1)


# ──────────────────────────────────────────────
# Section D — API Contract (static analysis)
# ──────────────────────────────────────────────

def check_api_static(root: Path):
    print(f"\n{BOLD}[D] API Contract (static){RESET}")

    go_files = list(root.rglob("*.go"))
    all_go_text = ""
    for gf in go_files:
        try:
            all_go_text += gf.read_text(errors="replace") + "\n"
        except:
            pass

    endpoints = {
        "POST /run":     ["/run", "POST"],
        "GET /healthz":  ["/healthz"],
        "GET /readyz":   ["/readyz"],
        "GET /info":     ["/info"],
    }
    for ep, needles in endpoints.items():
        if all(n.lower() in all_go_text.lower() for n in needles):
            record(f"Endpoint {ep}", "PASS", weight=1)
        else:
            record(f"Endpoint {ep}", "FAIL",
                   f"Could not find {needles} in Go source", weight=1)

    # Status vocabulary check
    statuses = ["accepted", "wrong_output", "output_whitespace_mismatch",
                "time_exceeded", "memory_exceeded", "runtime_error",
                "not_executed", "internal_error", "build_failed"]
    found_statuses   = [s for s in statuses if s in all_go_text.lower()]
    missing_statuses = [s for s in statuses if s not in all_go_text.lower()]

    if not missing_statuses:
        record("Full status vocabulary", "PASS",
               f"All {len(statuses)} status strings found", weight=2)
    elif len(found_statuses) >= 6:
        record("Full status vocabulary", "WARN",
               f"Missing: {', '.join(missing_statuses)}", weight=2, score_pct=0.6)
    else:
        record("Full status vocabulary", "FAIL",
               f"Only {len(found_statuses)}/{len(statuses)} statuses. Missing: {', '.join(missing_statuses)}", weight=2, score_pct=0.2)

    # Error response shape — code + message
    has_error_code = "error" in all_go_text.lower() and (
        '"code"' in all_go_text or '`json:"code"' in all_go_text
    )
    has_400 = "400" in all_go_text or "BadRequest" in all_go_text or "StatusBadRequest" in all_go_text
    if has_error_code and has_400:
        record("Error response shape (code+message, 400)", "PASS", weight=1)
    elif has_400:
        record("Error response shape (code+message, 400)", "WARN",
               "400 responses present but error code/message struct unclear", weight=1, score_pct=0.5)
    else:
        record("Error response shape (code+message, 400)", "FAIL",
               "No 400 / error response structure found", weight=1)

    # Request size limit
    has_max_bytes = "MaxBytesReader" in all_go_text or "maxBytes" in all_go_text or "max_source" in all_go_text.lower()
    if has_max_bytes:
        record("Request size limits", "PASS", weight=1)
    else:
        record("Request size limits", "WARN",
               "No MaxBytesReader or source-size cap found. Security hole #4.", weight=1, score_pct=0.3)


# ──────────────────────────────────────────────
# Section E — Languages
# ──────────────────────────────────────────────

def check_languages_static(root: Path):
    print(f"\n{BOLD}[E] Language Support{RESET}")

    # Look for YAML config files
    yaml_files = list(root.rglob("*.yaml")) + list(root.rglob("*.yml"))
    in_scope = {"c", "cpp", "c++", "java", "python", "py3", "bash",
                "sh", "javascript", "node", "js", "verilog"}
    interpreted = {"python", "py3", "bash", "sh", "javascript", "node", "js"}
    compiled    = {"c", "cpp", "c++", "java"}

    found_langs = set()
    lang_yaml_path = None
    for yf in yaml_files:
        try:
            text = yf.read_text(errors="replace").lower()
            for lang in in_scope:
                if lang in text:
                    found_langs.add(lang)
            if "language" in text and ("cmd" in text or "run" in text):
                lang_yaml_path = yf
        except:
            pass

    found_interpreted = found_langs & interpreted
    found_compiled    = found_langs & compiled

    if lang_yaml_path:
        record("Language YAML config found", "PASS",
               str(lang_yaml_path.relative_to(root)), weight=1)
    else:
        record("Language YAML config found", "FAIL",
               "No YAML file looks like a language registry", weight=1)

    if found_interpreted:
        record("Interpreted language in registry", "PASS",
               f"Found: {', '.join(found_interpreted)}", weight=1.5)
    else:
        record("Interpreted language in registry", "FAIL",
               "No interpreted language (Python/Bash/Node) found in YAML", weight=1.5)

    if found_compiled:
        record("Compiled language in registry", "PASS",
               f"Found: {', '.join(found_compiled)}", weight=1.5)
    else:
        record("Compiled language in registry", "FAIL",
               "No compiled language (C/C++/Java) found in YAML", weight=1.5)

    total_langs = len(found_langs)
    record("Language count", "PASS" if total_langs >= 5 else "WARN" if total_langs >= 2 else "FAIL",
           f"{total_langs} in-scope languages detected in YAML",
           weight=1, score_pct=min(1.0, total_langs / 7))

    # Check for flag allow-list in YAML
    for yf in yaml_files:
        try:
            text = yf.read_text(errors="replace").lower()
            if "flag_allowlist" in text or "allow_list" in text or "allowlist" in text:
                record("Flag allow-list in language YAML", "PASS", weight=1)
                return
        except:
            pass
    record("Flag allow-list in language YAML", "FAIL",
           "No flag_allowlist found in any YAML. Security hole #3 unaddressed.", weight=1)


# ──────────────────────────────────────────────
# Section F — Concurrency
# ──────────────────────────────────────────────

def check_concurrency(root: Path):
    print(f"\n{BOLD}[F] Concurrency{RESET}")

    go_files = list(root.rglob("*.go"))
    all_go_text = ""
    for gf in go_files:
        try:
            all_go_text += gf.read_text(errors="replace") + "\n"
        except:
            pass

    # Semaphore pattern: buffered channel or sync.Semaphore
    has_semaphore = (
        "make(chan struct{}" in all_go_text or
        "semaphore" in all_go_text.lower() or
        "workerpool" in all_go_text.lower() or
        "worker_pool" in all_go_text.lower() or
        ("chan" in all_go_text and "NumCPU" in all_go_text)
    )
    if has_semaphore:
        record("Bounded concurrency (semaphore/pool)", "PASS", weight=2)
    else:
        record("Bounded concurrency (semaphore/pool)", "FAIL",
               "No semaphore or worker-pool pattern found. Under load, goroutines will be unbounded.", weight=2)

    # runtime.NumCPU as default
    if "NumCPU" in all_go_text or "GOMAXPROCS" in all_go_text:
        record("NumCPU-based default concurrency limit", "PASS", weight=0.5)
    else:
        record("NumCPU-based default concurrency limit", "WARN",
               "Spec default is runtime.NumCPU(). Hardcoded limit is acceptable but note it.", weight=0.5, score_pct=0.5)

    # Benchmarks doc
    bench = root / "docs" / "benchmarks.md"
    if bench.exists():
        text = bench.read_text(errors="replace").lower()
        has_numbers = bool(re.search(r"\d+\s*(ms|req|rps|p50|p95|p99)", text))
        has_clients = any(c in text for c in ["100", "50", "10", "1 client"])
        if has_numbers and has_clients:
            record("benchmarks.md with numbers", "PASS",
                   "Contains latency/RPS data at multiple concurrency levels", weight=1.5)
        else:
            record("benchmarks.md with numbers", "WARN",
                   "File exists but may lack p50/p95/p99 at 1/10/50/100 clients", weight=1.5, score_pct=0.4)
    else:
        record("benchmarks.md present", "FAIL",
               "docs/benchmarks.md missing. Spec requires p50/p95/p99 at 1/10/50/100 clients.", weight=1.5)


# ──────────────────────────────────────────────
# Section G — Security
# ──────────────────────────────────────────────

def check_security(root: Path):
    print(f"\n{BOLD}[G] Security{RESET}")

    go_files  = list(root.rglob("*.go"))
    all_go_text = ""
    file_map = {}
    for gf in go_files:
        try:
            t = gf.read_text(errors="replace")
            all_go_text += t + "\n"
            file_map[gf] = t
        except:
            pass

    holes = {}

    # Hole 1: path traversal — filepath.Base or explicit validation
    h1 = (
        "filepath.Base" in all_go_text or
        "path.Base" in all_go_text or
        "traversal" in all_go_text.lower() or
        re.search(r'strings\.(Contains|HasPrefix).*["\']\.\.', all_go_text) or
        re.search(r'filepath\.Clean|filepath\.Abs', all_go_text)
    )
    holes[1] = ("Path traversal fix (#1)", h1, "filepath.Base/Clean validation")

    # Hole 3: flag injection — allow-list logic in Go
    h3_patterns = ["allowlist", "allow_list", "flagallow", "flag_allow",
                   "validflag", "permitted", "whitelist"]
    h3 = any(p in all_go_text.lower() for p in h3_patterns)
    holes[3] = ("Compiler flag injection fix (#3)", h3, "allow-list filter in Go code")

    # Hole 4: request size limits
    h4 = (
        "MaxBytesReader" in all_go_text or
        "maxBody" in all_go_text or
        "maxSource" in all_go_text or
        "rlimit" in all_go_text.lower()
    )
    holes[4] = ("Request size limits fix (#4)", h4, "MaxBytesReader or source size cap")

    # Hole 6: output truncation
    h6 = (
        "LimitReader" in all_go_text or
        "io.Limit" in all_go_text or
        "truncat" in all_go_text.lower() or
        "maxOutput" in all_go_text or
        "max_output" in all_go_text.lower()
    )
    holes[6] = ("Unbounded output fix (#6)", h6, "io.LimitReader or truncation")

    # Hole 7: defer cleanup / orphan sweep
    h7_defer = bool(re.search(r"defer.*clean|defer.*remov|defer.*os\.Remove", all_go_text, re.I))
    h7_sweep = "startup" in all_go_text.lower() or "orphan" in all_go_text.lower() or \
               bool(re.search(r"os\.ReadDir|filepath\.Walk.*jail", all_go_text))
    holes[7] = ("Stale jail dir cleanup fix (#7)", h7_defer or h7_sweep, "defer cleanup or startup sweep")

    fixed = [n for n, (_, ok, _) in holes.items() if ok]
    not_fixed = [n for n, (_, ok, _) in holes.items() if not ok]

    for n, (label, ok, hint) in holes.items():
        if ok:
            record(label, "PASS", weight=1)
        else:
            record(label, "FAIL", f"Not found. Hint: {hint}", weight=1)

    n_fixed = len(fixed)
    if n_fixed >= 5:
        record("≥5 security holes closed (required)", "PASS",
               f"{n_fixed}/7 holes closed (checked 5 key ones above)", weight=2)
    elif n_fixed >= 3:
        record("≥5 security holes closed (required)", "WARN",
               f"Only {n_fixed}/7 checked holes closed. Spec requires 5+.", weight=2, score_pct=0.4)
    else:
        record("≥5 security holes closed (required)", "FAIL",
               f"Only {n_fixed}/7 checked holes closed. Spec requires 5+.", weight=2, score_pct=0.0)

    # Bonus: security.md docs
    sec_doc = root / "docs" / "security.md"
    if sec_doc.exists():
        text = sec_doc.read_text(errors="replace").lower()
        has_file_line = bool(re.search(r"\.go:\d+", text))
        if has_file_line:
            record("security.md with file:line references", "PASS",
                   "(Bonus) file:line fix references found in security.md", weight=0.5)
        else:
            record("security.md exists but lacks file:line refs", "WARN",
                   "Spec asks for file:line links to each fix in PR/docs", weight=0.5, score_pct=0.5)


# ──────────────────────────────────────────────
# Section H — Commit hygiene
# ──────────────────────────────────────────────

def check_commits(root: Path):
    print(f"\n{BOLD}[H] Commit Hygiene{RESET}")

    rc, log, _ = run_cmd('git log --oneline', cwd=root)
    if rc != 0:
        record("Git history readable", "SKIP", "Could not read git log", weight=1)
        return

    commits = [l.strip() for l in log.strip().splitlines() if l.strip()]
    n = len(commits)

    if n == 0:
        record("Commit count", "FAIL", "No commits found", weight=1)
        return
    elif n == 1:
        record("Commit count (not a single giant commit)", "FAIL",
               "Only 1 commit. Spec explicitly says avoid a single giant commit.", weight=1.5)
    elif n <= 3:
        record("Commit count (not a single giant commit)", "WARN",
               f"Only {n} commits. Should show incremental progress.", weight=1.5, score_pct=0.4)
    else:
        record("Commit count (not a single giant commit)", "PASS",
               f"{n} commits found", weight=1.5)

    # Detect meaningless messages
    bad_patterns = ["initial commit", "wip", "fix", "update", "changes",
                    "asdf", "test commit", "temp", "misc", "stuff"]
    bad_msgs = [c for c in commits if any(c.lower().split(" ", 1)[-1].lower() in
                                           [b] for b in bad_patterns)]
    if len(bad_msgs) > n * 0.4:
        record("Commit message quality", "WARN",
               f"{len(bad_msgs)}/{n} commits have generic/lazy messages", weight=0.5, score_pct=0.3)
    else:
        record("Commit message quality", "PASS",
               f"Messages look descriptive enough", weight=0.5)


# ──────────────────────────────────────────────
# Section I — README quality
# ──────────────────────────────────────────────

def check_readme(root: Path):
    print(f"\n{BOLD}[I] README Quality{RESET}")

    readme = find_file(root, "README.md", "readme.md")
    if not readme:
        record("README.md present", "FAIL", "No README.md found", weight=1)
        return

    text = readme.read_text(errors="replace")
    text_lower = text.lower()

    # Banned words from spec
    banned = ["elegant", "robust", "seamlessly", "leverage", "cutting-edge",
              "state-of-the-art", "powerful", "intuitive", "comprehensive",
              "world-class", "innovative"]
    found_banned = [w for w in banned if w in text_lower]

    if not found_banned:
        record("README: no banned AI filler words", "PASS",
               "None of the spec-banned words found", weight=1)
    else:
        record("README: no banned AI filler words", "WARN",
               f"Banned words present: {', '.join(found_banned)}. Spec says remove them.", weight=1, score_pct=0.3)

    # No emoji decoration
    has_emoji = bool(re.search(r'[\U0001F300-\U0001FFFF]|[\U00002600-\U000027FF]', text))
    if has_emoji:
        record("README: no emoji decoration", "WARN",
               "Emoji found. Spec says no emoji.", weight=0.5, score_pct=0.3)
    else:
        record("README: no emoji decoration", "PASS", weight=0.5)

    # Length check — spec says "short"
    word_count = len(text.split())
    if word_count > 800:
        record("README: concise (spec says 'short')", "WARN",
               f"{word_count} words. Spec says short — what it is, how to run, where the docs are.", weight=0.5, score_pct=0.4)
    else:
        record("README: concise (spec says 'short')", "PASS",
               f"{word_count} words", weight=0.5)

    # Has Makefile reference
    if "make" in text_lower:
        record("README references Makefile", "PASS", weight=0.5)
    else:
        record("README references Makefile", "WARN",
               "Spec says 'how to run' should point at the Makefile", weight=0.5, score_pct=0.3)


# ──────────────────────────────────────────────
# Section J — Live Docker test (optional)
# ──────────────────────────────────────────────

def check_live_docker(root: Path, run_docker: bool):
    print(f"\n{BOLD}[J] Live Docker Test{RESET}")

    if not run_docker:
        record("Docker build & run", "SKIP",
               "Pass --docker to enable live Docker build+run tests", weight=5)
        return

    tag = "goboxd_eval_tmp"

    print(f"  {CYAN}Building Docker image (this may take several minutes)…{RESET}")
    rc, out, err = run_cmd(f"docker build -t {tag} .", cwd=root, timeout=600, capture=True)
    if rc != 0:
        record("Docker build", "FAIL",
               f"docker build failed:\n{err[-500:]}", weight=5)
        return
    record("Docker build", "PASS", weight=2)

    # Run container
    print(f"  {CYAN}Starting container…{RESET}")
    rc2, cid, _ = run_cmd(f"docker run --privileged -d -p 18888:8080 {tag}", timeout=20)
    if rc2 != 0:
        record("Docker run", "FAIL", "docker run failed", weight=3)
        return

    cid = cid.strip()
    time.sleep(3)  # wait for service to start

    base = "http://localhost:18888"

    # /healthz
    status, body = http_get(f"{base}/healthz")
    if status == 200:
        record("GET /healthz → 200", "PASS", body[:100], weight=1)
    else:
        record("GET /healthz → 200", "FAIL",
               f"Got HTTP {status}: {body[:100]}", weight=1)

    # /readyz
    status, body = http_get(f"{base}/readyz")
    if status in (200, 503):
        record("GET /readyz responds", "PASS",
               f"HTTP {status} — {'degraded' if status==503 else 'ready'}", weight=1)
    else:
        record("GET /readyz responds", "FAIL", f"HTTP {status}", weight=1)

    # /info
    status, body = http_get(f"{base}/info")
    if status == 200:
        try:
            data = json.loads(body)
            has_keys = all(k in data for k in ["build_info", "languages", "limits"])
            record("GET /info shape", "PASS" if has_keys else "WARN",
                   "All top-level keys present" if has_keys else f"Missing keys in: {list(data.keys())}",
                   weight=1)
        except:
            record("GET /info JSON valid", "FAIL", f"Not valid JSON: {body[:100]}", weight=1)
    else:
        record("GET /info → 200", "FAIL", f"HTTP {status}", weight=1)

    # POST /run — Python hello world
    payload = json.dumps({
        "language": "py3",
        "source": 'print("hello")',
        "tests": [{"stdin": "", "expected_stdout": "hello"}]
    }).encode()
    try:
        req = urllib.request.Request(
            f"{base}/run", data=payload,
            headers={"Content-Type": "application/json"}, method="POST"
        )
        with urllib.request.urlopen(req, timeout=15) as r:
            resp_body = r.read().decode()
            resp_status = r.status
        data = json.loads(resp_body)
        top_status = data.get("status")
        if top_status == "accepted":
            record("POST /run Python hello → accepted", "PASS", weight=2)
        else:
            record("POST /run Python hello → accepted", "FAIL",
                   f"top-level status={top_status}\n{resp_body}", weight=2)
    except Exception as e:
        record("POST /run Python hello", "FAIL", str(e), weight=2)

    # POST /run — unknown language → 400
    payload2 = json.dumps({
        "language": "nonexistent_lang_xyz",
        "source": "x",
        "tests": [{"stdin": "", "expected_stdout": "x"}]
    }).encode()
    try:
        req2 = urllib.request.Request(
            f"{base}/run", data=payload2,
            headers={"Content-Type": "application/json"}, method="POST"
        )
        try:
            with urllib.request.urlopen(req2, timeout=5) as r2:
                rc_status = r2.status
                body2 = r2.read().decode()
        except urllib.error.HTTPError as e:
            rc_status = e.code
            body2 = e.read().decode()
        if rc_status == 400:
            record("POST /run unknown lang → 400", "PASS", weight=1)
        else:
            record("POST /run unknown lang → 400", "FAIL",
                   f"Got HTTP {rc_status} instead of 400", weight=1)
    except Exception as e:
        record("POST /run unknown lang → 400", "FAIL", str(e), weight=1)

    # Cleanup
    run_cmd(f"docker stop {cid}", timeout=15)
    run_cmd(f"docker rm {cid}", timeout=10)
    run_cmd(f"docker rmi {tag}", timeout=30)


# ──────────────────────────────────────────────
# Final report
# ──────────────────────────────────────────────

def print_report(repo_path: str):
    print(f"\n{'═'*62}")
    print(f"{BOLD}  goboxd Phase 1 — Evaluation Report{RESET}")
    print(f"  Repo: {repo_path}")
    print(f"{'═'*62}\n")

    for r in results:
        r.display()

    total_weight = sum(r.weight for r in results if r.status != "SKIP")
    total_score  = sum(r.score_pct * r.weight for r in results)
    pct = (total_score / total_weight * 100) if total_weight else 0

    counts = {"PASS": 0, "WARN": 0, "FAIL": 0, "SKIP": 0}
    for r in results:
        counts[r.status] += 1

    print(f"\n{'─'*62}")
    print(f"  {GREEN}PASS: {counts['PASS']}{RESET}  "
          f"{YELLOW}WARN: {counts['WARN']}{RESET}  "
          f"{RED}FAIL: {counts['FAIL']}{RESET}  "
          f"SKIP: {counts['SKIP']}")
    print(f"\n  {BOLD}Score: {total_score:.1f} / {total_weight:.1f}  →  {pct:.0f}%{RESET}")

    if pct >= 80:
        grade = f"{GREEN}Strong submission — likely advances{RESET}"
    elif pct >= 60:
        grade = f"{YELLOW}Decent foundation — gaps to address{RESET}"
    elif pct >= 40:
        grade = f"{YELLOW}Partial work — significant issues remain{RESET}"
    else:
        grade = f"{RED}Incomplete — does not meet Phase 1 bar{RESET}"
    print(f"  {grade}")

    # Highlight critical failures
    hard_fails = [r for r in results if r.status == "FAIL" and r.weight >= 2]
    if hard_fails:
        print(f"\n  {BOLD}{RED}Critical failures (weight ≥ 2):{RESET}")
        for r in hard_fails:
            print(f"    • {r.name}")

    print(f"\n{'═'*62}\n")


# ──────────────────────────────────────────────
# Entry point
# ──────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="goboxd Phase 1 submission checker",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=textwrap.dedent("""\
        Examples:
          # Check a GitHub repo (clones it):
          python goboxd_checker.py --repo https://github.com/someuser/goboxd

          # Check a local clone:
          python goboxd_checker.py --repo /path/to/local/goboxd

          # Include live Docker build+run tests (slow, needs Docker):
          python goboxd_checker.py --repo https://github.com/someuser/goboxd --docker
        """)
    )
    parser.add_argument("--repo", required=True,
                        help="GitHub URL (https://github.com/…) or local path to repo")
    parser.add_argument("--clone-dir", default=None,
                        help="Where to clone (default: temp dir, auto-cleaned)")
    parser.add_argument("--docker", action="store_true",
                        help="Run live Docker build and API tests (requires Docker)")
    args = parser.parse_args()

    tmp_dir = None
    repo_input = args.repo.strip()

    # Resolve path
    if repo_input.startswith("http://") or repo_input.startswith("https://"):
        tmp_dir = args.clone_dir or tempfile.mkdtemp(prefix="goboxd_eval_")
        root = Path(tmp_dir)
        ok = clone_repo(repo_input, root)
        if not ok:
            print(f"{RED}Failed to clone repo. Exiting.{RESET}")
            sys.exit(1)
    else:
        root = Path(repo_input).expanduser().resolve()
        if not root.exists():
            print(f"{RED}Path does not exist: {root}{RESET}")
            sys.exit(1)

    print(f"\n{BOLD}Evaluating: {root}{RESET}")

    try:
        check_repo_structure(root)
        check_go_project(root)
        check_nsjail(root)
        check_api_static(root)
        check_languages_static(root)
        check_concurrency(root)
        check_security(root)
        check_commits(root)
        check_readme(root)
        check_live_docker(root, args.docker)
    finally:
        print_report(args.repo)
        if tmp_dir and not args.clone_dir:
            shutil.rmtree(tmp_dir, ignore_errors=True)


if __name__ == "__main__":
    main()