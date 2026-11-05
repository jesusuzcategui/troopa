package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/gitinit"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestInteractiveArgsAreInteractive(t *testing.T) {
	prompt := PairPrompt()
	for _, agent := range []string{"claude", "codex", "opencode", "pi"} {
		args, err := InteractiveArgs(agent, "model-x", prompt)
		if err != nil {
			t.Fatal(err)
		}
		joined := "\n" + strings.Join(args, "\n") + "\n"
		if strings.Contains(joined, "\n-p\n") || strings.Contains(joined, "\nexec\n") || strings.Contains(joined, "\nrun\n") || strings.Contains(joined, "\n--auto\n") {
			t.Fatalf("%s non-interactive: %v", agent, args)
		}
		if !strings.Contains(strings.Join(args, " "), prompt) {
			t.Fatalf("%s missing prompt: %v", agent, args)
		}
		if !strings.Contains(strings.Join(args, " "), "model-x") {
			t.Fatalf("%s missing model: %v", agent, args)
		}
	}
	if _, err := InteractiveArgs("me", "", prompt); err == nil {
		t.Fatal("me should fail")
	}
}

func TestPairOpenArgsTmuxAndTerminal(t *testing.T) {
	script := "/wt/pair.sh"
	tmux := PairOpenArgs(true, "kitty -e", "/wt", script)
	if tmux[0] != "tmux" || !contains(tmux, "split-window") || !contains(tmux, "/wt") {
		t.Fatalf("tmux args %v", tmux)
	}
	term := PairOpenArgs(false, "kitty -e", "/wt", script)
	if strings.Join(term, " ") != "kitty -e sh "+script {
		t.Fatalf("term args %v", term)
	}
	wez := PairOpenArgs(false, "wezterm start --", "/wt", script)
	if wez[0] != "wezterm" || wez[len(wez)-1] != script {
		t.Fatalf("wez %v", wez)
	}
}

func TestPreparePairWritesPAIRAndReusesWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	project := store.Project{RepoPath: repo, Slug: "demo"}
	task := store.Task{ID: 3, Title: "Login", Type: store.TypeFeature, Commits: store.CommitsNone,
		Description: "Auth", AcceptanceCriteria: "Hay login"}
	sess, err := PreparePair(cfg, project, task, Session{})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := os.ReadFile(filepath.Join(sess.Worktree, "PAIR.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(pair)
	if !strings.Contains(body, "red") || !strings.Contains(body, "green") || !strings.Contains(body, "commits=none") {
		t.Fatalf("PAIR.md: %s", body)
	}
	taskMD, _ := os.ReadFile(filepath.Join(sess.Worktree, "TASK.md"))
	if !strings.Contains(string(taskMD), "Hay login") {
		t.Fatal("TASK.md missing")
	}
	again, err := PreparePair(cfg, project, task, sess)
	if err != nil {
		t.Fatal(err)
	}
	if again.Worktree != sess.Worktree {
		t.Fatalf("reuse %s vs %s", again.Worktree, sess.Worktree)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "keep.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := PreparePair(cfg, project, task, Session{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(third.Worktree, "keep.txt")); err != nil {
		t.Fatal("worktree was recreated")
	}
}

func TestFinishPairRunsChecks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	if err := gitinit.InitRepo(repo, "Demo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "troopa.toml"), []byte("checks = [\"true\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Git(repo, "add", "troopa.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Git(repo, "commit", "-m", "checks"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Launch{Worktrees: filepath.Join(t.TempDir(), "wt")}
	task := store.Task{ID: 4, Title: "Checks", Type: store.TypeFeature}
	sess, err := PreparePair(cfg, store.Project{RepoPath: repo, Slug: "demo"}, task, Session{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess.Worktree, "SUMMARY.md"), []byte("hecho\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, raw, ok := FinishPair(sess, repo)
	if !ok || summary != "hecho" || !strings.Contains(raw, "true") {
		t.Fatalf("summary=%q ok=%v raw=%s", summary, ok, raw)
	}
}

func TestWritePairScriptInteractive(t *testing.T) {
	dir := t.TempDir()
	args, err := InteractiveArgs("claude", "sonnet", PairPrompt())
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePairScript(dir, config.AgentSpec{Command: "claude"}, "/bin/echo", args); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "pair.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "PAIR.md") || strings.Contains(s, " -p ") {
		t.Fatalf("script: %s", s)
	}
}

func TestResolveTerminalConfiguredAndFallback(t *testing.T) {
	got, err := ResolveTerminal("alacritty -e")
	if err != nil || got != "alacritty -e" {
		t.Fatalf("%q %v", got, err)
	}
	old := lookPath
	lookPath = func(file string) (string, error) {
		if file == "wezterm" {
			return "/usr/bin/wezterm", nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { lookPath = old })
	got, err = ResolveTerminal("")
	if err != nil || got != "wezterm start --" {
		t.Fatalf("fallback %q %v", got, err)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
