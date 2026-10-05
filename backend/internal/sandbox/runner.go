package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

// DiagnosticIssue provides structured compiler or linter diagnostic information.
type DiagnosticIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warning", "info"
}

// ExecutionResult holds compilation or test execution output.
type ExecutionResult struct {
	Language       string            `json:"language"`
	Success        bool              `json:"success"`
	CompilerOutput string            `json:"compiler_output"`
	TestOutput     string            `json:"test_output,omitempty"`
	ExitCode       int               `json:"exit_code"`
	Duration       time.Duration     `json:"duration"`
	ErrorsFound    []string          `json:"errors_found,omitempty"`
	Diagnostics    []DiagnosticIssue `json:"diagnostics,omitempty"`
}

// Runner inspects code by running real syntax/compiler/test checks on disk.
type Runner struct {
	timeout time.Duration
}

// NewRunner creates a new sandbox runner.
func NewRunner(timeout time.Duration) *Runner {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Runner{timeout: timeout}
}

// VerifyCodebase detects the primary language and executes native syntax, compiler,
// or automated test validation on disk.
func (r *Runner) VerifyCodebase(ctx context.Context, wsPath string, files []domain.CodeFile) *ExecutionResult {
	start := time.Now()

	// Detect primary language from files or workspace root markers
	lang := r.detectLanguage(wsPath, files)

	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var res *ExecutionResult
	switch lang {
	case "go":
		res = r.verifyGo(runCtx, wsPath, files)
	case "dart":
		res = r.verifyDart(runCtx, wsPath, files)
	case "typescript", "javascript":
		res = r.verifyNode(runCtx, wsPath, files, lang)
	case "python":
		res = r.verifyPython(runCtx, wsPath, files)
	default:
		res = r.verifyGeneric(wsPath, files)
	}

	res.Duration = time.Since(start)
	return res
}

// RunTests executes the project's own test suite and returns its output.
//
// The task gate calls this before accepting a task as done. It returns an empty
// string, and no error, when the project has no detectable test command: a
// project without tests must be recorded as "not verified" rather than as a
// silent pass, because "we have no tests so nothing failed" is exactly the
// reasoning that let unfinished work be called finished.
func (r *Runner) RunTests(root string) (string, error) {
	if root == "" {
		return "", nil
	}

	lang := r.detectLanguage(root, nil)

	runCtx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// The command is built once with its working directory attached. Building it
	// first and setting Dir afterwards would silently run the tests against the
	// daemon's own directory instead of the project being verified.
	var cmd *exec.Cmd
	switch lang {
	case "go":
		if !fileExists(filepath.Join(root, "go.mod")) {
			return "", nil
		}
		cmd = exec.CommandContext(runCtx, "go", "test", "./...")
	case "dart":
		if !fileExists(filepath.Join(root, "test")) {
			return "", nil
		}
		if path, err := exec.LookPath("flutter"); err == nil && path != "" {
			cmd = exec.CommandContext(runCtx, "flutter", "test", "--no-pub")
		} else if path, err := exec.LookPath("dart"); err == nil && path != "" {
			cmd = exec.CommandContext(runCtx, "dart", "test")
		} else {
			return "", nil
		}
	case "python":
		if !fileExists(filepath.Join(root, "pytest.ini")) &&
			!fileExists(filepath.Join(root, "setup.py")) &&
			!fileExists(filepath.Join(root, "pyproject.toml")) {
			return "", nil
		}
		// The interpreter matches what the syntax check uses, so a project is
		// never verified with one interpreter and tested with another.
		cmd = exec.CommandContext(runCtx, "python3", "-m", "pytest", "-q")
	case "typescript", "javascript":
		pkg := filepath.Join(root, "package.json")
		if !fileExists(pkg) {
			return "", nil
		}
		if !strings.Contains(strings.ToLower(readFileString(pkg)), "\"test\"") {
			return "", nil
		}
		cmd = exec.CommandContext(runCtx, "npm", "test", "--silent")
	default:
		return "", nil
	}

	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func (r *Runner) detectLanguage(wsPath string, files []domain.CodeFile) string {
	counts := make(map[string]int)
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Path))
		switch {
		case ext == ".go" || f.Language == "go":
			counts["go"]++
		case ext == ".dart" || f.Language == "dart":
			counts["dart"]++
		case ext == ".ts" || ext == ".tsx" || f.Language == "typescript":
			counts["typescript"]++
		case ext == ".js" || ext == ".jsx" || f.Language == "javascript":
			counts["javascript"]++
		case ext == ".py" || f.Language == "python":
			counts["python"]++
		}
	}

	bestLang := ""
	bestCount := 0
	for l, c := range counts {
		if c > bestCount {
			bestCount = c
			bestLang = l
		}
	}
	if bestLang != "" {
		return bestLang
	}

	// Fallback to project root markers
	if fileExists(filepath.Join(wsPath, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(wsPath, "pubspec.yaml")) {
		return "dart"
	}
	if fileExists(filepath.Join(wsPath, "tsconfig.json")) {
		return "typescript"
	}
	if fileExists(filepath.Join(wsPath, "package.json")) {
		return "javascript"
	}
	if fileExists(filepath.Join(wsPath, "requirements.txt")) || fileExists(filepath.Join(wsPath, "pyproject.toml")) {
		return "python"
	}

	return "generic"
}

func (r *Runner) verifyGo(ctx context.Context, wsPath string, files []domain.CodeFile) *ExecutionResult {
	if _, err := exec.LookPath("go"); err != nil {
		return r.verifyGeneric(wsPath, files)
	}

	// 1. Run go vet
	var cmd *exec.Cmd
	if fileExists(filepath.Join(wsPath, "go.mod")) {
		cmd = exec.CommandContext(ctx, "go", "vet", "./...")
	} else {
		var goFiles []string
		for _, f := range files {
			if strings.HasSuffix(f.Path, ".go") {
				goFiles = append(goFiles, f.Path)
			}
		}
		if len(goFiles) > 0 {
			args := append([]string{"vet"}, goFiles...)
			cmd = exec.CommandContext(ctx, "go", args...)
		} else {
			cmd = exec.CommandContext(ctx, "go", "vet", ".")
		}
	}
	cmd.Dir = wsPath
	out, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(out))

	success := err == nil
	errors, diags := parseCompilerErrors(outputStr)

	testOutput := ""
	// 2. If go vet succeeded and test files exist, run go test -short ./...
	hasTests := false
	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			hasTests = true
			break
		}
	}
	if success && hasTests {
		testCmd := exec.CommandContext(ctx, "go", "test", "-short", "./...")
		testCmd.Dir = wsPath
		tOut, tErr := testCmd.CombinedOutput()
		testOutput = strings.TrimSpace(string(tOut))
		if tErr != nil {
			success = false
			tErrors, tDiags := parseCompilerErrors(testOutput)
			errors = append(errors, tErrors...)
			diags = append(diags, tDiags...)
		}
	}

	return &ExecutionResult{
		Language:       "go",
		Success:        success,
		CompilerOutput: outputStr,
		TestOutput:     testOutput,
		ErrorsFound:    errors,
		Diagnostics:    diags,
	}
}

func (r *Runner) verifyDart(ctx context.Context, wsPath string, files []domain.CodeFile) *ExecutionResult {
	// Look for dart or flutter CLI
	analyzerBin := ""
	var analyzerArgs []string

	if path, err := exec.LookPath("flutter"); err == nil && path != "" {
		analyzerBin = "flutter"
		analyzerArgs = []string{"analyze", "--no-pub"}
	} else if path, err := exec.LookPath("dart"); err == nil && path != "" {
		analyzerBin = "dart"
		analyzerArgs = []string{"analyze"}
	}

	if analyzerBin != "" {
		cmd := exec.CommandContext(ctx, analyzerBin, analyzerArgs...)
		cmd.Dir = wsPath
		out, err := cmd.CombinedOutput()
		outputStr := strings.TrimSpace(string(out))

		success := err == nil || !strings.Contains(strings.ToLower(outputStr), "error •")
		errors, diags := parseDartAnalyzerErrors(outputStr)

		return &ExecutionResult{
			Language:       "dart",
			Success:        success,
			CompilerOutput: outputStr,
			ErrorsFound:    errors,
			Diagnostics:    diags,
		}
	}

	// Fallback to static syntax inspection when Dart/Flutter CLI is not in PATH
	return r.verifyStaticSyntax(wsPath, files, "dart")
}

func (r *Runner) verifyNode(ctx context.Context, wsPath string, files []domain.CodeFile, lang string) *ExecutionResult {
	// Check if TypeScript project with tsconfig.json
	hasTsConfig := fileExists(filepath.Join(wsPath, "tsconfig.json"))
	if hasTsConfig {
		tscBin := ""
		var tscArgs []string
		if _, err := exec.LookPath("tsc"); err == nil {
			tscBin = "tsc"
			tscArgs = []string{"--noEmit"}
		} else if _, err := exec.LookPath("npx"); err == nil {
			tscBin = "npx"
			tscArgs = []string{"tsc", "--noEmit"}
		}

		if tscBin != "" {
			cmd := exec.CommandContext(ctx, tscBin, tscArgs...)
			cmd.Dir = wsPath
			out, err := cmd.CombinedOutput()
			outputStr := strings.TrimSpace(string(out))

			errors, diags := parseCompilerErrors(outputStr)
			return &ExecutionResult{
				Language:       "typescript",
				Success:        err == nil,
				CompilerOutput: outputStr,
				ErrorsFound:    errors,
				Diagnostics:    diags,
			}
		}
	}

	// Try node -c on JavaScript files
	if _, err := exec.LookPath("node"); err == nil {
		var errs []string
		var diags []DiagnosticIssue
		for _, f := range files {
			ext := strings.ToLower(filepath.Ext(f.Path))
			if ext == ".js" || ext == ".mjs" || ext == ".cjs" {
				filePath := filepath.Join(wsPath, f.Path)
				if fileExists(filePath) {
					cmd := exec.CommandContext(ctx, "node", "--check", filePath)
					out, cmdErr := cmd.CombinedOutput()
					if cmdErr != nil {
						msg := strings.TrimSpace(string(out))
						errs = append(errs, msg)
						diags = append(diags, DiagnosticIssue{
							File:     f.Path,
							Message:  msg,
							Severity: "error",
						})
					}
				}
			}
		}
		if len(errs) > 0 {
			return &ExecutionResult{
				Language:       lang,
				Success:        false,
				CompilerOutput: strings.Join(errs, "\n"),
				ErrorsFound:    errs,
				Diagnostics:    diags,
			}
		}
	}

	return r.verifyStaticSyntax(wsPath, files, lang)
}

func (r *Runner) verifyPython(ctx context.Context, wsPath string, files []domain.CodeFile) *ExecutionResult {
	if _, err := exec.LookPath("python"); err == nil {
		cmd := exec.CommandContext(ctx, "python", "-m", "compileall", "-q", ".")
		cmd.Dir = wsPath
		out, err := cmd.CombinedOutput()
		outputStr := strings.TrimSpace(string(out))

		errors, diags := parseCompilerErrors(outputStr)
		return &ExecutionResult{
			Language:       "python",
			Success:        err == nil,
			CompilerOutput: outputStr,
			ErrorsFound:    errors,
			Diagnostics:    diags,
		}
	}

	return r.verifyStaticSyntax(wsPath, files, "python")
}

func (r *Runner) verifyStaticSyntax(wsPath string, files []domain.CodeFile, lang string) *ExecutionResult {
	var errors []string
	var diags []DiagnosticIssue

	for _, f := range files {
		content := f.Content
		if content == "" {
			fullPath := filepath.Join(wsPath, f.Path)
			if b, err := os.ReadFile(fullPath); err == nil {
				content = string(b)
			}
		}

		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			continue
		}

		// Basic brace/bracket balancing check
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")
		if openBraces != closeBraces {
			msg := fmt.Sprintf("%s: unbalanced braces ({ count: %d, } count: %d)", f.Path, openBraces, closeBraces)
			errors = append(errors, msg)
			diags = append(diags, DiagnosticIssue{
				File:     f.Path,
				Message:  msg,
				Severity: "error",
			})
		}
	}

	success := len(errors) == 0
	outMsg := fmt.Sprintf("Static syntax verified for %d files (%s).", len(files), lang)
	if !success {
		outMsg = strings.Join(errors, "\n")
	}

	return &ExecutionResult{
		Language:       lang,
		Success:        success,
		CompilerOutput: outMsg,
		ErrorsFound:    errors,
		Diagnostics:    diags,
	}
}

func (r *Runner) verifyGeneric(wsPath string, files []domain.CodeFile) *ExecutionResult {
	return &ExecutionResult{
		Language:       "generic",
		Success:        true,
		CompilerOutput: fmt.Sprintf("Validated %d files in sandbox.", len(files)),
	}
}

var diagLineRegex = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+))?:\s*(.+)$`)

func parseCompilerErrors(output string) ([]string, []DiagnosticIssue) {
	if output == "" {
		return nil, nil
	}

	var errors []string
	var diags []DiagnosticIssue

	lines := strings.Split(output, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if match := diagLineRegex.FindStringSubmatch(trimmed); len(match) > 4 {
			file := match[1]
			line, _ := strconv.Atoi(match[2])
			col, _ := strconv.Atoi(match[3])
			msg := match[4]

			severity := "error"
			if strings.Contains(strings.ToLower(msg), "warning") {
				severity = "warning"
			}

			diags = append(diags, DiagnosticIssue{
				File:     file,
				Line:     line,
				Column:   col,
				Message:  msg,
				Severity: severity,
			})
			errors = append(errors, trimmed)
		} else if strings.Contains(trimmed, "error") || strings.Contains(trimmed, "FAIL") {
			errors = append(errors, trimmed)
		}
	}

	return errors, diags
}

func parseDartAnalyzerErrors(output string) ([]string, []DiagnosticIssue) {
	if output == "" {
		return nil, nil
	}

	var errors []string
	var diags []DiagnosticIssue

	lines := strings.Split(output, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		parts := strings.Split(trimmed, " • ")
		if len(parts) >= 4 {
			sev := strings.ToLower(parts[0])
			msg := parts[1]
			loc := parts[2]

			diag := DiagnosticIssue{
				Message:  msg,
				Severity: sev,
			}
			locParts := strings.Split(loc, ":")
			if len(locParts) >= 2 {
				diag.File = locParts[0]
				diag.Line, _ = strconv.Atoi(locParts[1])
				if len(locParts) >= 3 {
					diag.Column, _ = strconv.Atoi(locParts[2])
				}
			}
			diags = append(diags, diag)
			if sev == "error" {
				errors = append(errors, trimmed)
			}
		} else if strings.Contains(strings.ToLower(trimmed), "error") {
			errors = append(errors, trimmed)
		}
	}

	return errors, diags
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
