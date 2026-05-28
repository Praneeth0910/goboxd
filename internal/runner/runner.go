package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
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
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate random bytes for job ID", "error", err)
	}
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
	SourceFilename   string // validated by handler before passing here
	ArtifactFilename string // validated by handler before passing here
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

// buildNsjailRunArgs constructs nsjail args for the RUN phase.
// --chroot is set to jailDir, so all paths inside the jail are jail-relative.
// E.g. source file at jailDir/solution.py is accessed as /solution.py inside the jail.
func buildNsjailRunArgs(jailDir string, limits config.ResourceLimits, cmd string, cmdArgs []string) []string {
	wallTimeS := limits.WallTimeS
	if wallTimeS <= 0 {
		wallTimeS = 10
	}
	memoryKB := limits.MemoryKB
	if memoryKB <= 0 {
		memoryKB = 256 * 1024
	}
	memoryMB := memoryKB / 1024
	if memoryMB <= 0 {
		memoryMB = 1
	}
	maxProcesses := limits.MaxProcesses
	if maxProcesses <= 0 {
		maxProcesses = 64
	}

	args := []string{
		"--mode", "o",
		"--time_limit", strconv.Itoa(wallTimeS),
		"--rlimit_as", strconv.Itoa(memoryMB),
		"--rlimit_nproc", strconv.Itoa(maxProcesses),
		"--max_cpus", "1",
		"--log_fd", "3",
		"--bindmount_ro", "/usr:/usr",
		"--bindmount_ro", "/lib:/lib",
		"--bindmount_ro", "/lib64:/lib64",
		"--bindmount_ro", "/bin:/bin",
		"--env", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--chroot", jailDir,
		"--",
		cmd,
	}
	return append(args, cmdArgs...)
}

// buildNsjailBuildArgs constructs nsjail args for the BUILD phase.
// The chroot is set to / (host root) with a writable bind-mount of jailDir
// so the compiler can write the artifact directly into jailDir using absolute paths.
// PATH is injected so collect2/ld can be found by g++.
func buildNsjailBuildArgs(jailDir string, limits config.ResourceLimits, cmd string, cmdArgs []string) []string {
	wallTimeS := limits.WallTimeS
	if wallTimeS <= 0 {
		wallTimeS = 30
	}
	memoryKB := limits.MemoryKB
	if memoryKB <= 0 {
		memoryKB = 1024 * 1024
	}
	memoryMB := memoryKB / 1024
	if memoryMB <= 0 {
		memoryMB = 1
	}
	maxProcesses := limits.MaxProcesses
	if maxProcesses <= 0 {
		maxProcesses = 100
	}

	args := []string{
		"--mode", "o",
		"--time_limit", strconv.Itoa(wallTimeS),
		"--rlimit_as", strconv.Itoa(memoryMB),
		"--rlimit_nproc", strconv.Itoa(maxProcesses),
		"--max_cpus", "1",
		"--log_fd", "3",
		"--bindmount_ro", "/usr:/usr",
		"--bindmount_ro", "/lib:/lib",
		"--bindmount_ro", "/lib64:/lib64",
		"--bindmount_ro", "/bin:/bin",
		// rw bind-mount so compiler can write the output artifact
		"--bindmount", jailDir + ":" + jailDir,
		"--env", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--chroot", "/",
		"--cwd", jailDir,
		"--",
		cmd,
	}
	return append(args, cmdArgs...)
}

// runPhase is the phase indicator passed to runCommand.
type runPhase int

const (
	phaseBuild runPhase = iota
	phaseRun
)

// runCommand executes either a plain command or an nsjail-wrapped command
// depending on nsjail availability. It returns stdout, stderr, elapsed ms,
// and whether the context deadline was exceeded.
// phase selects the nsjail argument set:
//   - phaseBuild: chroot=/, rw bindmount of jailDir, absolute paths (for compilers)
//   - phaseRun:   chroot=jailDir, jail-relative paths (for user programs)
//
// CommandResult holds the results of an executed command.
type CommandResult struct {
	Stdout         string
	Stderr         string
	ElapsedMS      int64
	TimedOut       bool
	Error          error
	MemoryExceeded bool
}

func runCommand(
	ctx context.Context,
	jailDir string,
	limits config.ResourceLimits,
	cmdStr string,
	cmdArgs []string,
	stdin io.Reader,
	phase runPhase,
) CommandResult {
	var cmd *exec.Cmd
	var nsjailLogReader *os.File
	var nsjailLogWriter *os.File
	var pipeErr error

	if nsjailAvailable() {
		var njArgs []string
		switch phase {
		case phaseBuild:
			njArgs = buildNsjailBuildArgs(jailDir, limits, cmdStr, cmdArgs)
		default:
			njArgs = buildNsjailRunArgs(jailDir, limits, cmdStr, cmdArgs)
		}
		cmd = exec.CommandContext(ctx, nsjailPath(), njArgs...)

		nsjailLogReader, nsjailLogWriter, pipeErr = os.Pipe()
		if pipeErr == nil {
			cmd.ExtraFiles = []*os.File{nsjailLogWriter} // becomes fd 3
		} else {
			slog.Error("failed to create pipe for nsjail log", "error", pipeErr)
		}
	} else {
		// Fallback: direct execution (dev/CI environments without nsjail)
		cmd = exec.CommandContext(ctx, cmdStr, cmdArgs...)
		cmd.Dir = jailDir
	}

	if stdin != nil {
		cmd.Stdin = stdin
	}

	// Wrap pipes with CapReader to bound memory usage
	outPipe, outErr := cmd.StdoutPipe()
	errPipe, errErr := cmd.StderrPipe()
	if outErr != nil || errErr != nil {
		if nsjailLogWriter != nil {
			nsjailLogWriter.Close()
		}
		if nsjailLogReader != nil {
			nsjailLogReader.Close()
		}
		return CommandResult{Error: fmt.Errorf("failed to create output pipes: stdout_err=%w stderr_err=%w", outErr, errErr)}
	}

	start := time.Now()
	if startErr := cmd.Start(); startErr != nil {
		if nsjailLogWriter != nil {
			nsjailLogWriter.Close()
		}
		if nsjailLogReader != nil {
			nsjailLogReader.Close()
		}
		return CommandResult{Error: startErr}
	}
	if nsjailLogWriter != nil {
		nsjailLogWriter.Close() // Parent no longer needs the write end
	}

	// Forcibly close pipes when context expires. This prevents io.Copy from hanging
	// forever if the process is killed but orphaned child processes hold the pipes open.
	go func() {
		<-ctx.Done()
		outPipe.Close()
		errPipe.Close()
		if nsjailLogReader != nil {
			nsjailLogReader.Close()
		}
	}()

	// Read capped stdout and stderr concurrently
	capOut := sandbox.CapReader(outPipe, sandbox.DefaultMaxOutputBytes)
	capErr := sandbox.CapReader(errPipe, sandbox.DefaultMaxOutputBytes)

	var outBuf, errBuf bytes.Buffer
	var outReadErr, errReadErr error
	var nsjailLogBuf bytes.Buffer

	done := make(chan struct{})
	go func() {
		_, outReadErr = io.Copy(&outBuf, capOut)
		_, errReadErr = io.Copy(&errBuf, capErr)
		if nsjailLogReader != nil {
			io.Copy(&nsjailLogBuf, nsjailLogReader)
		}
		close(done)
	}()
	<-done

	runErr := cmd.Wait()
	elapsedMS := time.Since(start).Milliseconds()

	if outReadErr != nil {
		slog.Error("stdout pipe read error", "error", outReadErr)
	}
	if errReadErr != nil {
		slog.Error("stderr pipe read error", "error", errReadErr)
	}

	timedOut := ctx.Err() == context.DeadlineExceeded

	var isKilled bool
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			if exitErr.ExitCode() == 137 {
				isKilled = true
			}
		}
	}

	logStr := nsjailLogBuf.String()
	if strings.Contains(logStr, "signal: 9") {
		isKilled = true
	}

	var memoryExceeded bool
	if strings.Contains(logStr, "rlimit") {
		memoryExceeded = true
	} else if isKilled {
		if strings.Contains(logStr, "memory") || strings.Contains(logStr, "OOM") || strings.Contains(logStr, "[STATS]") {
			memoryExceeded = true
		}
	}

	return CommandResult{
		Stdout:         outBuf.String(),
		Stderr:         errBuf.String(),
		ElapsedMS:      elapsedMS,
		TimedOut:       timedOut,
		Error:          runErr,
		MemoryExceeded: memoryExceeded,
	}
}

// ---------------------------------------------------------------------------
// Template expansion (mirrors handler's expandArgs)
// ---------------------------------------------------------------------------

func expandArgs(args []string, sourcePath, artifactPath string, flags []string) []string {
	out := make([]string, 0, len(args)+len(flags))
	class := strings.TrimSuffix(filepath.Base(artifactPath), filepath.Ext(artifactPath))
	for _, arg := range args {
		switch arg {
		case config.TemplateFlags:
			out = append(out, flags...)
		case config.TemplateSource:
			out = append(out, sourcePath)
		case config.TemplateArtifact:
			out = append(out, artifactPath)
		case config.TemplateClass:
			out = append(out, class)
		default:
			arg = strings.ReplaceAll(arg, config.TemplateSource, sourcePath)
			arg = strings.ReplaceAll(arg, config.TemplateArtifact, artifactPath)
			arg = strings.ReplaceAll(arg, config.TemplateClass, class)
			out = append(out, arg)
		}
	}
	return out
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
		return RunResult{Status: status.StatusInternalError}, fmt.Errorf("failed to create jail dir: %w", err)
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
	// Safely open the file to prevent TOCTOU symlink attacks
	f, err := os.OpenFile(sourcePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return RunResult{Status: status.StatusInternalError}, fmt.Errorf("failed to safely open source file: %w", err)
	}
	if _, err := f.WriteString(req.Source); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			slog.Error("failed to close source file after write error", "error", closeErr)
		}
		return RunResult{Status: status.StatusInternalError}, fmt.Errorf("failed to write source: %w", err)
	}
	if err := f.Close(); err != nil {
		return RunResult{Status: status.StatusInternalError}, fmt.Errorf("failed to close source file: %w", err)
	}

	// Verify the written path is a regular file
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return RunResult{Status: status.StatusInternalError}, fmt.Errorf("failed to lstat source file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return RunResult{Status: status.StatusInternalError}, fmt.Errorf("source file is not a regular file")
	}

	// ── 3. Build step (compiled languages only) ───────────────────────────────
	var buildRes BuildResult

	if lang.Build != nil {
		artifactFilename := lang.ArtifactFilename
		if lang.ArtifactFilenameStrategy == "from_request" {
			if req.ArtifactFilename != "" {
				artifactFilename = req.ArtifactFilename
			} else if sourceFilename != "" {
				artifactFilename = strings.TrimSuffix(sourceFilename, filepath.Ext(sourceFilename)) + filepath.Ext(lang.ArtifactFilename)
			}
		}
		if artifactFilename == "" {
			artifactFilename = "solution"
		}
		artifactPath := filepath.Join(jailDir, artifactFilename)

		var userFlags []string
		if req.Build != nil {
			userFlags = req.Build.Flags
		}
		// Build phase uses chroot=/ so absolute host paths are correct inside the jail.
		buildArgs := expandArgs(lang.Build.Args, sourcePath, artifactPath, userFlags)

		// Determine build limits (request overrides config defaults)
		buildLimits := lang.Build.Limits
		if buildLimits.WallTimeS <= 0 {
			buildLimits.WallTimeS = 30 // generous default for compilation
		}
		if req.Build != nil && req.Build.Limits.WallTimeS > 0 {
			buildLimits.WallTimeS = req.Build.Limits.WallTimeS
		}

		// Add 2s buffer so nsjail's internal --time_limit fires first and cleans up gracefully
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(buildLimits.WallTimeS+2)*time.Second)
		defer cancel()

		res := runCommand(ctx, jailDir, buildLimits,
			lang.Build.Cmd, buildArgs, nil, phaseBuild)

		buildRes = BuildResult{
			Stdout:     res.Stdout,
			Stderr:     res.Stderr,
			DurationMS: res.ElapsedMS,
		}

		if res.Error != nil {
			buildRes.Status = "failed"
			notExec := make([]TestResult, len(req.Tests))
			for i := range notExec {
				notExec[i] = TestResult{Status: status.StatusNotExecuted}
			}
			return RunResult{
				Status: status.StatusBuildFailed,
				Build:  buildRes,
				Tests:  notExec,
			}, nil
		}
		buildRes.Status = "ok"
	}

	// ── 4. Run each test case ─────────────────────────────────────────────────
	testResults := make([]TestResult, 0, len(req.Tests))
	overallStatus := status.StatusAccepted

	for _, tc := range req.Tests {
		tr := runTestCase(lang, jailDir, sourceFilename, req, tc)
		testResults = append(testResults, tr)
		if overallStatus == status.StatusAccepted && tr.Status != status.StatusAccepted {
			overallStatus = tr.Status
		}
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
	if lang.ArtifactFilenameStrategy == "from_request" {
		if req.ArtifactFilename != "" {
			artifactFilename = req.ArtifactFilename
		} else if sourceFilename != "" {
			artifactFilename = strings.TrimSuffix(sourceFilename, filepath.Ext(sourceFilename)) + filepath.Ext(lang.ArtifactFilename)
		}
	}
	if artifactFilename == "" {
		artifactFilename = "solution"
	}

	// Run phase: nsjail uses --chroot=jailDir, so paths inside the jail are
	// relative to jailDir root.  E.g. jailDir/solution.py → /solution.py inside jail.
	jailSourcePath := "/" + sourceFilename
	jailArtifactPath := "/" + artifactFilename

	runArgs := expandArgs(lang.Run.Args, jailSourcePath, jailArtifactPath, req.Run.Flags)

	// Expand {{artifact}}, {{source}}, {{class}} in the run command itself.
	runCmd := lang.Run.Cmd
	class := strings.TrimSuffix(artifactFilename, filepath.Ext(artifactFilename))
	runCmd = strings.ReplaceAll(runCmd, config.TemplateArtifact, artifactFilename)
	runCmd = strings.ReplaceAll(runCmd, config.TemplateSource, sourceFilename)
	runCmd = strings.ReplaceAll(runCmd, config.TemplateClass, class)

	runLimits := lang.Run.Limits
	if runLimits.WallTimeS <= 0 {
		runLimits.WallTimeS = 10
	}
	if req.Run.Limits.WallTimeS > 0 {
		runLimits.WallTimeS = req.Run.Limits.WallTimeS
	}

	// Add 2s buffer so nsjail's internal --time_limit fires first and cleans up gracefully
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(runLimits.WallTimeS+2)*time.Second)
	defer cancel()

	stdinReader := strings.NewReader(tc.Stdin)
	res := runCommand(
		ctx, jailDir, runLimits, runCmd, runArgs, stdinReader, phaseRun)

	var testStatus string
	switch {
	case res.TimedOut:
		testStatus = status.StatusTimeExceeded
	case res.MemoryExceeded:
		testStatus = status.StatusMemoryExceeded
	case res.Error != nil:
		testStatus = status.StatusRuntimeError
	case strings.TrimRight(res.Stdout, "\r\n") == strings.TrimRight(tc.ExpectedStdout, "\r\n"):
		testStatus = status.StatusAccepted
	case strings.TrimSpace(res.Stdout) == strings.TrimSpace(tc.ExpectedStdout):
		testStatus = status.StatusOutputWhitespaceMismatch
	default:
		testStatus = status.StatusWrongOutput
	}

	return TestResult{
		Status:     testStatus,
		Stdout:     res.Stdout,
		Stderr:     res.Stderr,
		DurationMS: res.ElapsedMS,
	}
}
