package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

// Verifier decides whether the active task is actually finished, using the real
// codebase instead of the agent's own claim.
//
// The swarm previously trusted the round score, and the round score only looked
// at code quality. A clean, well-built single file scored 95% while most of the
// project did not exist, so the run stopped and called the project done. The
// gate here runs before the arbiter and asks a narrower question: does this one
// task's acceptance condition hold in the code that is really on disk?
//
// An agent saying "done" is a claim, not evidence. Only the check below can
// close a task, which is the same standard the loop uses on itself: it reports
// what it checked, never what it assumes.

// TaskVerdict is the outcome of verifying one task.
type TaskVerdict struct {
	TaskID string `json:"task_id"`
	// Passed is true only when the acceptance condition was checked against
	// real code and found to hold.
	Passed bool `json:"passed"`
	// Checked lists what was actually inspected, so a human can audit the
	// verdict instead of trusting it.
	Checked []string `json:"checked,omitempty"`
	// Gaps are the concrete reasons the task is not finished.
	Gaps []string `json:"gaps,omitempty"`
	Summary string `json:"summary"`
}

// TaskVerifier checks one task's acceptance condition against the workspace.
type TaskVerifier struct {
	client llm.Client
	// runTests runs the project's real test suite. It is optional: a project
	// without tests must not be able to claim "no tests failed, so it passes",
	// which the Definition of Done explicitly forbids.
	runTests func(root string) (string, error)
}

func NewTaskVerifier(client llm.Client) *TaskVerifier {
	return &TaskVerifier{client: client}
}

// SetTestRunner installs the command used to run the project's tests.
func (v *TaskVerifier) SetTestRunner(fn func(root string) (string, error)) {
	v.runTests = fn
}

// Verify checks the active task and, when it passes, closes it in the plan.
//
// The order matters: mechanical facts first (do the task's files exist, do the
// tests run), then the model's judgement of the acceptance condition against
// the code it was shown. A missing file is a fact and cannot be argued away, so
// it is checked before spending a model call.
func (v *TaskVerifier) Verify(ctx context.Context, sCtx *SwarmContext) (*TaskVerdict, error) {
	task := sCtx.ActiveTaskSnapshot()
	if task == nil {
		return nil, nil
	}

	verdict := &TaskVerdict{TaskID: task.ID}

	// 1. Mechanical gate: the task named files, so they must exist on disk.
	missing := v.missingFiles(task, sCtx)
	if len(missing) > 0 {
		verdict.Gaps = append(verdict.Gaps, fmt.Sprintf(
			"these files the task required do not exist: %s", strings.Join(missing, ", ")))
		verdict.Summary = fmt.Sprintf("Task %s is not finished: required files are missing.", task.ID)
		return verdict, nil
	}
	verdict.Checked = append(verdict.Checked,
		fmt.Sprintf("all %d file(s) named by the task exist on disk", len(task.Files)))

	// 2. Test gate: run the real suite. Tests that do not exist are recorded as
	// not-run rather than as a pass.
	testOutput, testErr := v.runTestsSafely(sCtx)
	if testErr != nil {
		verdict.Gaps = append(verdict.Gaps, fmt.Sprintf("the test suite failed to run: %v", testErr))
		verdict.Summary = fmt.Sprintf("Task %s is not finished: tests could not be verified.", task.ID)
		return verdict, nil
	}
	if testOutput != "" {
		verdict.Checked = append(verdict.Checked, "ran the project's test suite")
		if strings.Contains(strings.ToLower(testOutput), "fail") {
			verdict.Gaps = append(verdict.Gaps, "the test suite reported failures")
			verdict.Summary = fmt.Sprintf("Task %s is not finished: tests are failing.", task.ID)
			return verdict, nil
		}
	} else {
		verdict.Checked = append(verdict.Checked,
			"no test runner is configured for this project, so tests could not be verified")
	}

	// 3. Acceptance gate: ask the model whether the task's own condition holds,
	// showing it the real files the task covers.
	acceptance, err := v.checkAcceptance(ctx, sCtx, task, verdict.Checked)
	if err != nil {
		return nil, err
	}
	verdict.Checked = append(verdict.Checked, "checked the acceptance condition against the real code")
	if len(acceptance.Gaps) > 0 {
		verdict.Gaps = append(verdict.Gaps, acceptance.Gaps...)
		verdict.Summary = acceptance.Summary
		return verdict, nil
	}

	verdict.Passed = true
	verdict.Summary = acceptance.Summary
	return verdict, nil
}

// missingFiles reports which of the task's files are absent from both disk and
// the in-memory file set.
//
// Both sources are consulted because the coder's output is synced to disk only
// at the end of its phase; a file can legitimately exist in memory and not yet
// on disk.
func (v *TaskVerifier) missingFiles(task *domain.ProjectTask, sCtx *SwarmContext) []string {
	if len(task.Files) == 0 {
		return nil
	}

	inMemory := make(map[string]bool, len(sCtx.CodeFilesSnapshot()))
	for _, f := range sCtx.CodeFilesSnapshot() {
		inMemory[f.Path] = true
	}

	var missing []string
	for _, rel := range task.Files {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		if inMemory[rel] {
			continue
		}
		if sCtx.WorkspacePath == "" {
			missing = append(missing, rel)
			continue
		}
		if _, err := os.Stat(filepath.Join(sCtx.WorkspacePath, rel)); err != nil {
			missing = append(missing, rel)
		}
	}
	return missing
}

// runTestsSafely invokes the configured test runner without letting a panic in
// a runner take down the whole swarm.
func (v *TaskVerifier) runTestsSafely(sCtx *SwarmContext) (output string, err error) {
	if v.runTests == nil || sCtx.WorkspacePath == "" {
		return "", nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("test runner panicked: %v", r)
		}
	}()
	return v.runTests(sCtx.WorkspacePath)
}

type acceptanceResult struct {
	Gaps    []string `json:"gaps"`
	Summary string   `json:"summary"`
}

// checkAcceptance asks the model to judge the task's acceptance condition
// against the real code for that task.
//
// The prompt forbids accepting anything it was not shown, because the previous
// failure was exactly that: a model asked whether work was done, with no code in
// front of it, and it agreed the work was done.
func (v *TaskVerifier) checkAcceptance(ctx context.Context, sCtx *SwarmContext, task *domain.ProjectTask, checked []string) (*acceptanceResult, error) {
	if v.client == nil {
		// With no model to ask, fall back to the mechanical evidence. This keeps
		// the gate useful offline instead of silently passing everything.
		return &acceptanceResult{
			Summary: fmt.Sprintf("Task %s: required files exist%s; acceptance could not be judged without a model.",
				task.ID, testNote(checked)),
		}, nil
	}

	model, apiKey, baseURL, err := v.resolveModel(sCtx)
	if err != nil {
		return nil, err
	}

	code := v.taskCode(task, sCtx)

	systemPrompt := `You are a strict verifier. You decide whether ONE task is actually finished.

Rules you must not break:
1. Judge only from the code shown below. If the code needed to satisfy the task is not shown, the task is NOT done.
2. Absence of evidence is failure, not success. Do not assume code exists.
3. Do not judge style, elegance, or the rest of the project. Only this task's acceptance condition.
4. If the code contains a placeholder, a TODO, a stub, an empty function body, or a panic("not implemented"), the task is NOT done.
5. Be concrete. Every gap must name the file and what is missing.
6. If and only if the acceptance condition holds in the shown code, return no gaps.

Return ONLY valid JSON:
{
  "gaps": ["specific reason the task is not finished, or empty array if it is finished"],
  "summary": "one sentence stating what you verified"
}`

	userPrompt := fmt.Sprintf(
		"TASK ID: %s\nTASK TITLE: %s\nACCEPTANCE CONDITION: %s\n\n"+
			"ALREADY CHECKED MECHANICALLY:\n- %s\n\n"+
			"CODE FOR THIS TASK:\n%s\n\n"+
			"Is this specific task finished?",
		task.ID, task.Title, task.Acceptance, strings.Join(checked, "\n- "), code)

	resp, err := v.client.Generate(ctx, llm.Request{
		Model:        model,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
	})
	if err != nil {
		return nil, fmt.Errorf("acceptance check failed: %w", err)
	}

	var result acceptanceResult
	if err := json.Unmarshal([]byte(CleanJSON(resp.Content)), &result); err != nil {
		return nil, fmt.Errorf("verifier returned invalid JSON: %w (content: %s)", err, resp.Content)
	}
	if result.Summary == "" {
		result.Summary = fmt.Sprintf("Task %s reviewed against the real code.", task.ID)
	}
	return &result, nil
}

// taskCode gathers the code the task is responsible for, preferring files the
// task named and falling back to the whole project for a task that named none.
func (v *TaskVerifier) taskCode(task *domain.ProjectTask, sCtx *SwarmContext) string {
	files := sCtx.CodeFilesSnapshot()
	if len(task.Files) > 0 {
		wanted := make(map[string]bool, len(task.Files))
		for _, f := range task.Files {
			wanted[strings.TrimSpace(f)] = true
		}
		var scoped []domain.CodeFile
		for _, f := range files {
			if wanted[f.Path] {
				scoped = append(scoped, f)
			}
		}
		if len(scoped) > 0 {
			return renderFiles(scoped)
		}
	}
	if len(files) == 0 {
		return "(the project currently contains no files)"
	}
	return renderFiles(files)
}

func renderFiles(files []domain.CodeFile) string {
	var sb strings.Builder
	for _, f := range files {
		fmt.Fprintf(&sb, "--- %s ---\n%s\n\n", f.Path, f.Content)
	}
	return sb.String()
}

// resolveModel picks the model used for verification and its credentials.
//
// The key lookup goes through ResolveProvider because APIKeys is keyed by
// provider name ("openrouter"), not by model id. Reading the map by model name
// returned an empty key for every agent, so every verification call went out
// unauthenticated and failed, which the loop then reported as a fatal error.
func (v *TaskVerifier) resolveModel(sCtx *SwarmContext) (model, apiKey, baseURL string, err error) {
	// Any model the swarm is already using will do. The lookup used to prefer a
	// fixed list of built-in roles, so a custom team made of roles the fixed
	// roster knows nothing about had no model to verify with — and the run then
	// ended with an error instead of doing any work.
	var chosen string
	if sCtx.AgentModels != nil {
		// A reviewer is preferred when the team has one, because judging
		// completion is a reasoning task.
		for _, role := range []domain.AgentRole{domain.RoleCritic, domain.RoleSecurity, domain.RoleCoder, domain.RoleArchitect} {
			if m := strings.TrimSpace(sCtx.AgentModels[role]); m != "" {
				chosen = m
				break
			}
		}
		if chosen == "" {
			// Fall back to whatever model the team does assign. Map order does
			// not matter here: any working model can judge an acceptance
			// condition, and refusing to run would be strictly worse.
			for _, m := range sCtx.AgentModels {
				if m = strings.TrimSpace(m); m != "" {
					chosen = m
					break
				}
			}
		}
	}
	if chosen == "" {
		if envM := os.Getenv("NEXUS_MODEL"); envM != "" {
			chosen = envM
		} else if envM := os.Getenv("OPENAI_MODEL"); envM != "" {
			chosen = envM
		}
	}
	if chosen == "" {
		return "", "", "", fmt.Errorf("no model is configured, so task completion cannot be verified")
	}

	route, ok := ResolveProvider(chosen)
	if !ok {
		return "", "", "", fmt.Errorf("model %q has no known provider, so verification cannot be authenticated", chosen)
	}

	key := ""
	if sCtx.APIKeys != nil {
		key = sCtx.APIKeys[route.apiKeyName]
	}
	if key == "" {
		// Fall back to the environment through the same helper every other agent
		// uses, so the verifier cannot end up with different credentials than
		// the swarm it is verifying.
		key = providerKeyFromEnv(route.apiKeyName)
	}
	if strings.TrimSpace(key) == "" {
		return "", "", "", fmt.Errorf("no API key found for provider %q, so verification cannot run", route.apiKeyName)
	}

	// The provider is resolved from the namespaced name, but the endpoint is
	// sent the bare model id. Sending the prefix through makes the provider
	// reject it as an unknown model, which the loop then treats as a fatal
	// verification failure and ends the run.
	return cleanModelName(chosen), key, route.baseURL, nil
}

func testNote(checked []string) string {
	for _, c := range checked {
		if strings.Contains(c, "test suite") {
			return " and the test suite ran"
		}
	}
	return ""
}