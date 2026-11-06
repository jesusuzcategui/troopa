//go:build unix

package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// waitRun waits for the detached agent of the current run and feeds the
// result to the model, like the watcher does.
func waitRun(t *testing.T, m Model) Model {
	t.Helper()
	if m.screen != screenLog || m.run.ID == 0 {
		t.Fatalf("no agent running: screen %v err %q status %q", m.screen, m.err, m.status)
	}
	next, _ := m.Update(runAgent(&exec.Cmd{Dir: m.runSess.Worktree}, "", m.run.ID)())
	return next.(Model)
}

func TestPlanRunsStepsInOrderWithAccountAndProfile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	hookOut := filepath.Join(t.TempDir(), "hooks.txt")
	toml := "checks = [\"true\"]\n\n[hooks]\napprove = \"echo \\\"$TROOPA_TASK\\\" >> " + hookOut + "\"\n"
	if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.Git(repo, "add", "troopa.toml"); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.Git(repo, "commit", "-m", "troopa"); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	_ = os.MkdirAll(bin, 0o755)
	script := "#!/bin/sh\necho \"$CLAUDE_CONFIG_DIR\" > account.txt\necho step >> steps.txt\necho done > SUMMARY.md\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	m := newTestModel(t)
	m.cfg.Launch = config.DefaultLaunch(t.TempDir())
	m.cfg.Launch.Brain = m.cfg.BrainRaw
	m.cfg.Launch.Notify = false
	m.cfg.Launch.Merge = "squash"
	m.cfg.Launch.Accounts = map[string]config.Account{"work": {}}
	prof, err := m.store.CreateProfile(store.Profile{Name: "Ana", Profession: "Chef"})
	if err != nil {
		t.Fatal(err)
	}
	m.cfg.DefaultProfile = prof.ID
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: repo, Account: "work"})
	if err != nil {
		t.Fatal(err)
	}
	m.openBoard(p)
	plan, err := m.store.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Menu", Agent: store.AgentClaude, Auto: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Starters", "Desserts"} {
		if _, err := m.store.AddPlanStep(plan.ID, store.Task{Title: title, Commits: store.CommitsNone}); err != nil {
			t.Fatal(err)
		}
	}

	next, _ := m.startPlan(plan.ID)
	got := waitRun(t, next.(Model))
	if got.screen != screenReview || got.run.State != store.RunReview {
		t.Fatalf("step 1: screen %v state %s err %q", got.screen, got.run.State, got.err)
	}
	first := got.run
	acc, _ := os.ReadFile(filepath.Join(first.Worktree, "account.txt"))
	wantAcc := filepath.Join(filepath.Dir(got.configPath), "accounts", "work", "claude")
	if strings.TrimSpace(string(acc)) != wantAcc {
		t.Fatalf("account dir %q, want %q", acc, wantAcc)
	}
	task, _ := os.ReadFile(filepath.Join(first.Worktree, "TASK.md"))
	if !strings.Contains(string(task), "Ana (Chef)") {
		t.Fatalf("TASK.md without profile:\n%s", task)
	}

	// Approving step 1 commits it and starts step 2 from its branch.
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	got = waitRun(t, next.(Model))
	second := got.run
	if second.TaskID == first.TaskID || second.Base != first.Branch {
		t.Fatalf("step 2 base %q, want %q", second.Base, first.Branch)
	}
	steps, _ := os.ReadFile(filepath.Join(second.Worktree, "steps.txt"))
	if strings.Count(string(steps), "step") != 2 {
		t.Fatalf("step 2 does not build on step 1: %q", steps)
	}
	if reason := got.mergeBlockReason(); !strings.Contains(reason, "plan") {
		t.Fatalf("a step must not merge alone: %q", reason)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	got = next.(Model)
	saved, _ := got.store.GetPlan(plan.ID)
	if saved.State != store.PlanDone {
		t.Fatalf("plan state %s, status %q", saved.State, got.status)
	}

	next, _ = got.mergePlan(plan.ID)
	got = next.(Model)
	if got.err != "" {
		t.Fatal(got.err)
	}
	merged, _ := os.ReadFile(filepath.Join(repo, "steps.txt"))
	if strings.Count(string(merged), "step") != 2 {
		t.Fatalf("plan not merged into base: %q", merged)
	}
	if saved, _ := got.store.GetPlan(plan.ID); saved.State != store.PlanMerged {
		t.Fatalf("plan state after merge %s", saved.State)
	}
	if _, err := os.Stat(first.Worktree); !os.IsNotExist(err) {
		t.Fatal("step worktrees should be removed after the merge")
	}
	launch.WaitHooks()
	deadline := time.Now().Add(3 * time.Second)
	var hooks []byte
	for time.Now().Before(deadline) {
		hooks, _ = os.ReadFile(hookOut)
		if strings.Count(string(hooks), "\n") >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(string(hooks), "Starters") || !strings.Contains(string(hooks), "Desserts") {
		t.Fatalf("approve hooks: %q", hooks)
	}
}

func TestPlansScreenCreatesPlanAndSteps(t *testing.T) {
	m := newTestModel(t)
	p, _ := m.store.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	m.openBoard(p)
	m = press(t, m, "w")
	if m.screen != screenPlans {
		t.Fatalf("w should open plans, screen %v", m.screen)
	}
	m = press(t, m, "n")
	for _, r := range "Launch" {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if m.plans.view != planAddStep {
		t.Fatalf("after saving a new plan, add steps: view %v err %q", m.plans.view, m.err)
	}
	for _, title := range []string{"One", "Two"} {
		for _, r := range title {
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			m = next.(Model)
		}
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(Model)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if len(m.plans.steps) != 2 || m.plans.steps[1].Title != "Two" {
		t.Fatalf("steps %+v", m.plans.steps)
	}
	m = press(t, m, "K")
	if m.plans.steps[0].Title != "Two" {
		t.Fatalf("K should move the step up: %+v", m.plans.steps)
	}
	if !strings.Contains(m.viewPlans(), "Launch") {
		t.Fatal("plan title missing from the view")
	}
}

func TestProfilesScreen(t *testing.T) {
	m := newTestModel(t)
	m.openProfiles(screenList)
	m = press(t, m, "n")
	for _, r := range "Ana" {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	for _, r := range "Chef" {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if len(m.profiles.list) != 1 || m.profiles.list[0].Label() != "Ana · Chef" {
		t.Fatalf("profiles %+v err %q", m.profiles.list, m.err)
	}
	if m.cfg.DefaultProfile != m.profiles.list[0].ID {
		t.Fatal("the first profile should become the default")
	}
	m = press(t, m, "d")
	m = press(t, m, "d")
	if len(m.profiles.list) != 0 || m.cfg.DefaultProfile != 0 {
		t.Fatalf("delete: %+v default %d", m.profiles.list, m.cfg.DefaultProfile)
	}
}

func TestLaunchingAStepFromTheBoardStartsThePlan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "bin")
	_ = os.MkdirAll(bin, 0o755)
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho x >> steps.txt\necho done > SUMMARY.md\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	m := newTestModel(t)
	m.cfg.Launch = config.DefaultLaunch(t.TempDir())
	m.cfg.Launch.Brain = m.cfg.BrainRaw
	m.cfg.Launch.Notify = false
	p, _ := m.store.Create(store.Project{Name: "App", RepoPath: repo})
	m.openBoard(p)
	plan, _ := m.store.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Two", Agent: store.AgentClaude, Auto: true})
	first, _ := m.store.AddPlanStep(plan.ID, store.Task{Title: "One", Commits: store.CommitsNone})
	_, _ = m.store.AddPlanStep(plan.ID, store.Task{Title: "Two", Commits: store.CommitsNone})

	// Launched from the board (ready), not with s in the plan.
	next, _ := m.launchAgent(first)
	got := waitRun(t, next.(Model))
	if saved, _ := got.store.GetPlan(plan.ID); saved.State != store.PlanRunning || saved.Base == "" {
		t.Fatalf("plan should be running: %+v", saved)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	got = next.(Model)
	if got.screen != screenLog || got.runTask.PlanStep != 2 {
		t.Fatalf("approving step 1 should launch step 2: screen %v step %d err %q status %q", got.screen, got.runTask.PlanStep, got.err, got.status)
	}
	got = waitRun(t, got)
}

func TestAutoApprovePlanRunsAlone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"test ! -f FAIL\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = launch.Git(repo, "add", "troopa.toml")
	_, _ = launch.Git(repo, "commit", "-m", "checks")
	bin := filepath.Join(t.TempDir(), "bin")
	_ = os.MkdirAll(bin, 0o755)
	// The third step breaks the checks.
	script := "#!/bin/sh\necho x >> steps.txt\nif [ \"$(wc -l < steps.txt)\" -ge 3 ]; then touch FAIL; fi\necho done > SUMMARY.md\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	m := newTestModel(t)
	m.cfg.Launch = config.DefaultLaunch(t.TempDir())
	m.cfg.Launch.Brain = m.cfg.BrainRaw
	m.cfg.Launch.Notify = false
	p, _ := m.store.Create(store.Project{Name: "App", RepoPath: repo})
	m.openBoard(p)
	plan, _ := m.store.CreatePlan(store.Plan{ProjectID: p.ID, Title: "Alone", Agent: store.AgentClaude, AutoApprove: true})
	for _, title := range []string{"One", "Two", "Three", "Four"} {
		_, _ = m.store.AddPlanStep(plan.ID, store.Task{Title: title, Commits: store.CommitsNone})
	}
	next, _ := m.startPlan(plan.ID)
	got := next.(Model)
	for i := 0; i < 3; i++ {
		got = waitRun(t, got)
	}
	steps, _ := got.store.PlanSteps(plan.ID)
	want := []string{store.StatusDone, store.StatusDone, store.StatusBlocked, store.StatusBacklog}
	for i, s := range steps {
		if s.Status != want[i] {
			t.Fatalf("step %d status %s, want %s (status %q err %q)", i+1, s.Status, want[i], got.status, got.err)
		}
	}
	if saved, _ := got.store.GetPlan(plan.ID); saved.State != store.PlanRunning {
		t.Fatalf("a blocked step leaves the plan running for x: %s", saved.State)
	}
}
