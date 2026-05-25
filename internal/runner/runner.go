package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thesouldev/goboxd/internal/config"
	"github.com/thesouldev/goboxd/internal/sandbox"
	"github.com/thesouldev/goboxd/internal/status"
)

var jobCounter atomic.Int64

// GenerateJobID creates a unique job identifier.
// Format: {counter}-{randomHex} — never reuses an ID.
func GenerateJobID() string {
	count := jobCounter.Add(1)
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s-%d", count, hex.EncodeToString(b), os.Getpid())
}

// ---------------------------------------------------------------------------
// Request / Response types owned by the runner layer
// ---------------------------------------------------------------------------

// PhaseConfig holds resource limits and optional compiler flags for a phase.
type PhaseConfig struct {
	Limits config.ResourceLimits
	Flags  []string
}

// TestCase is a single stdin/expected-stdout pair.
type TestCase struct {
	Stdin          string
	ExpectedStdout string
}

// RunRequest is the execution request passed from the handler to RunSandbox.
type RunRequest struct {
	Language         string
	Source           string
	SourceFilename   string   // validated by handler before passing here
	ArtifactFilename string   // validated by handler before passing here
	Build            *PhaseConfig
	Run              PhaseConfig
	Tests            []TestCase
}

// BuildResult is the outcome of the compilation step.
type BuildResult struct {
	Status     string // "ok" | "failed" | "internal_error"
	Stdout     string
	Stderr     string
	DurationMS int64
}

// TestResult is the outcome of executing one test case.
type TestResult struct {
	Status       string
	Stdout       string
	Stderr       string
	DurationMS   int64
	MemoryPeakKB int64 // populated when nsjail reports it; 0 otherwise
}

// RunResult aggregates the full outcome of one POST /run request.
type RunResult struct {
	Status string
	Build  BuildResult
	Tests  []TestResult
}

// ---------------------------------------------------------------------------
// nsjail helpers
// ---------------------------------------------------------------------------

// nsjailPath returns the resolved path to the nsjail binary.
// It falls back to /usr/sbin/nsjail when NSJAIL_PATH is not set.
func nsjailPath() string {
	if p := os.Getenv("NSJAIL_PATH"); p != "" {
		return p
	}
	return "/usr/sbin/nsjail"
}

// nsjailAvailable reports whether the nsjail binary is executable.
func nsjailAvailable() bool {
	_, err := exec.LookPath(nsjailPath())
	return err == nil
}

// buildNsjailArgs constructs the nsjail argument list that wraps a child command.
// jailDir is the per-request chroot. limits drives time/memory flags.
// cmd + cmdArgs are the actual program to run inside the jail.
func buildNsjailArgs(jailDir string, limits config.ResourceLimits, cmd string, cmdArgs []string) []string {
	wallTimeS := limits.WallTimeS
	if wallTimeS <= 0 {
		wallTimeS = 10
	}
	memoryKB := limits.MemoryKB
	if memoryKB <= 0 {
		memoryKB = 256 * 1024 // 256 MiB default
	}

	args := []string{
		"--mode", "o", // one-shot: exit after child finishes
		"--time_limit", strconv.Itoa(wallTimeS),
		"--rlimit_as", strconv.Itoa(memoryKB),
		"--max_cpus", "1",
		"--log_fd", "3", // redirect nsjail internal logs to fd 3 (discarded)
		"--bindmount_ro", "/usr:/usr",
		"--bindmount_ro", "/lib:/lib",
		"--bindmount_ro", "/lib64:/lib64",
		"--bindmount_ro", "/bin:/bin",
		"--chroot", jailDir,
		"--",
		cmd,
	}
	return append(args, cmdArgs...)
}

// runCommand executes either a plain command or an nsjail-wrapped command
// depending on nsjail availability. It returns stdout, stderr, elapsed ms,
// and whether the context deadline was exceeded.
func runCommand(
	ctx context.Context,
	jailDir string,
	limits config.ResourceLimits,
	cmdStr string,
	cmdArgs []string,
	stdin io.Reader,
) (stdout string, stderr string, elapsedMS int64, timedOut bool, err error) {
	var cmd *exec.Cmd

	if nsjailAvailable() {
		njArgs := buildNsjailArgs(jailDir, limits, cmdStr, cmdArgs)
		cmd = exec.CommandContext(ctx, nsjailPath(), njArgs...)
		// Discard fd 3 (nsjail log) by pointing it at /dev/null
		devNull, _ := os.Open(os.DevNull)
		if devNull != nil {
			cmd.ExtraFiles = []*os.File{devNull} // becomes fd 3
			defer devNull.Close()
		}
	} else {
		// Fallback: direct execution (dev/CI environments without nsjail)
		cmd = exec.CommandContext(ctx, cmdStr, cmdArgs...)
	}

	cmd.Dir = jailDir
	if stdin != nil {
		cmd.Stdin = stdin
	}

	// Wrap pipes with CapReader to bound memory usage
	outPipe, outErr := cmd.StdoutPipe()
	errPipe, errErr := cmd.StderrPipe()
	if outErr != nil || errErr != nil {
		return "", "", 0, false, fmt.Errorf("failed to create output pipes: %v %v", outErr, errErr)
	}

	start := time.Now()
	if startErr := cmd.Start(); startErr != nil {
		return "", "", 0, false, startErr
	}

	// Read capped stdout and stderr concurrently
	capOut := sandbox.CapReader(outPipe, sandbox.DefaultMaxOutputBytes)
	capErr := sandbox.CapReader(errPipe, sandbox.DefaultMaxOutputBytes)

	var outBuf, errBuf bytes.Buffer
	var outReadErr, errReadErr error

	done := make(chan struct{})
	go func() {
		_, outReadErr = io.Copy(&outBuf, capOut)
		_, errReadErr = io.Copy(&errBuf, capErr)
		close(done)
	}()
	<-done

	runErr := cmd.Wait()
	elapsedMS = time.Since(start).Milliseconds()

	_ = outReadErr
	_ = errReadErr

	timedOut = ctx.Err() == context.DeadlineExceeded
	return outBuf.String(), errBuf.String(), elapsedMS, timedOut, runErr
}

// ---------------------------------------------------------------------------
// Template expansion (mirrors handler's expandArgs)
// ---------------------------------------------------------------------------

func expandArgs(args []string, sourcePath, artifactPath string, flags []string) []string {
	out := make([]string, 0, len(args)+len(flags))
	for _, arg := range args {
		switch arg {
		case config.TemplateFlags:
			out = append(out, flags...)
		case config.TemplateSource:
			out = append(out, sourcePath)
		case config.TemplateArtifact:
			out = append(out, artifactPath)
		default:
			arg = strings.ReplaceAll(arg, config.TemplateSource, sourcePath)
			arg = strings.ReplaceAll(arg, config.TemplateArtifact, artifactPath)
			out = append(out, arg)
		}
	}
	return out
}

// statusPrecedence maps each result status to a severity level (higher = worse).
var statusPrecedence = map[string]int{
	status.Accepted:                0,
	status.OutputWhitespaceMismatch: 1,
	status.WrongOutput:             2,
	status.TimeExceeded:            3,
	status.MemoryExceeded:          4,
	status.RuntimeError:            5,
	status.BuildFailed:             6,
	status.InternalError:           7,
}

// worstStatus returns whichever of a, b has the higher precedence.
func worstStatus(a, b string) string {
	if statusPrecedence[b] > statusPrecedence[a] {
		return b
	}
	return a
}

// ---------------------------------------------------------------------------
// RunSandbox — full lifecycle for one POST /run request
// ---------------------------------------------------------------------------

// RunSandbox handles the execution of one /run request end-to-end:
//  1. Creates an isolated jail directory (deferred cleanup)
//  2. Writes the source file into the jail dir
//  3. If the language has a build step: compiles via nsjail; aborts on failure
//  4. Runs each test case via nsjail, captures stdout/stderr, compares output
//  5. Returns a RunResult with correctly aggregated top-level status
//
// Concurrency is NOT managed here — the caller (handler) is responsible for
// semaphore acquisition before invoking RunSandbox.
func RunSandbox(lang config.Language, req RunRequest) (RunResult, error) {
	// ── 1. Create isolated jail directory ────────────────────────────────────
	jailDir, cleanup, err := sandbox.NewJailDir(os.TempDir())
	if err != nil {
		return RunResult{Status: status.InternalError}, fmt.Errorf("failed to create jail dir: %w", err)
	}
	defer cleanup()

	// ── 2. Write source file ──────────────────────────────────────────────────
	sourceFilename := lang.SourceFilename
	if req.SourceFilename != "" && lang.SourceFilenameStrategy == "from_request" {
		sourceFilename = req.SourceFilename
	}
	if sourceFilename == "" {
		sourceFilename = "solution"
	}
	sourcePath := filepath.Join(jailDir, sourceFilename)
	if err := os.WriteFile(sourcePath, []byte(req.Source), 0644); err != nil {
		return RunResult{Status: status.InternalError}, fmt.Errorf("failed to write source: %w", err)
	}

	// ── 3. Build step (compiled languages only) ───────────────────────────────
	var buildRes BuildResult

	if lang.Build != nil {
		artifactFilename := lang.ArtifactFilename
		if artifactFilename == "" {
			artifactFilename = "solution"
		}
		artifactPath := filepath.Join(jailDir, artifactFilename)

		var userFlags []string
		if req.Build != nil {
			userFlags = req.Build.Flags
		}
		buildArgs := expandArgs(lang.Build.Args, sourcePath, artifactPath, userFlags)

		// Determine build limits (request overrides config defaults)
		buildLimits := lang.Build.Limits
		if buildLimits.WallTimeS <= 0 {
			buildLimits.WallTimeS = 30 // generous default for compilation
		}
		if req.Build != nil && req.Build.Limits.WallTimeS > 0 {
			buildLimits.WallTimeS = req.Build.Limits.WallTimeS
		}

		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(buildLimits.WallTimeS)*time.Second)
		defer cancel()

		outStr, errStr, elapsedMS, _, buildErr := runCommand(ctx, jailDir, buildLimits,
			lang.Build.Cmd, buildArgs, nil)

		buildRes = BuildResult{
			Stdout:     outStr,
			Stderr:     errStr,
			DurationMS: elapsedMS,
		}

		if buildErr != nil {
			buildRes.Status = "failed"
			notExec := make([]TestResult, len(req.Tests))
			for i := range notExec {
				notExec[i] = TestResult{Status: status.NotExecuted}
			}
			return RunResult{
				Status: status.BuildFailed,
				Build:  buildRes,
				Tests:  notExec,
			}, nil
		}
		buildRes.Status = "ok"
	}

	// ── 4. Run each test case ─────────────────────────────────────────────────
	testResults := make([]TestResult, 0, len(req.Tests))
	overallStatus := status.Accepted

	for _, tc := range req.Tests {
		tr := runTestCase(lang, jailDir, sourceFilename, req, tc)
		testResults = append(testResults, tr)
		overallStatus = worstStatus(overallStatus, tr.Status)
	}

	return RunResult{
		Status: overallStatus,
		Build:  buildRes,
		Tests:  testResults,
	}, nil
}

// runTestCase executes a single test case inside the jail and returns its result.
func runTestCase(
	lang config.Language,
	jailDir string,
	sourceFilename string,
	req RunRequest,
	tc TestCase,
) TestResult {
	artifactFilename := lang.ArtifactFilename
	if artifactFilename == "" {
		artifactFilename = "solution"
	}
	artifactPath := filepath.Join(jailDir, artifactFilename)

	runArgs := expandArgs(lang.Run.Args, filepath.Join(jailDir, sourceFilename), artifactPath, nil)

	// Expand {{artifact}} and {{source}} in the run command using bare filenames
	// (relative to jailDir) so that "./{{artifact}}" → "./solution" resolves correctly.
	runCmd := lang.Run.Cmd
	runCmd = strings.ReplaceAll(runCmd, config.TemplateArtifact, artifactFilename)
	runCmd = strings.ReplaceAll(runCmd, config.TemplateSource, sourceFilename)

	runLimits := lang.Run.Limits
	if runLimits.WallTimeS <= 0 {
		runLimits.WallTimeS = 10
	}
	if req.Run.Limits.WallTimeS > 0 {
		runLimits.WallTimeS = req.Run.Limits.WallTimeS
	}

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(runLimits.WallTimeS)*time.Second)
	defer cancel()

	stdinReader := strings.NewReader(tc.Stdin)
	outStr, errStr, elapsedMS, timedOut, runErr := runCommand(
		ctx, jailDir, runLimits, runCmd, runArgs, stdinReader)

	var testStatus string
	switch {
	case timedOut:
		testStatus = status.TimeExceeded
	case runErr != nil:
		testStatus = status.RuntimeError
	case outStr == tc.ExpectedStdout:
		testStatus = status.Accepted
	case strings.TrimSpace(outStr) == strings.TrimSpace(tc.ExpectedStdout):
		testStatus = status.OutputWhitespaceMismatch
	default:
		testStatus = status.WrongOutput
	}

	return TestResult{
		Status:     testStatus,
		Stdout:     outStr,
		Stderr:     errStr,
		DurationMS: elapsedMS,
	}
}
