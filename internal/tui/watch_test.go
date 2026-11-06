package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// runAgent waits for the agent Troopa already started in the background for
// this run and returns the message the watcher would deliver.
func runAgent(cmd *exec.Cmd, _ string, runID int64) func() tea.Msg {
	return func() tea.Msg {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if st, _ := launch.AgentStatus(cmd.Dir); st != launch.AgentRunning {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		return agentStatusMsg(runID, cmd.Dir)
	}
}

func TestReconcileMarksLostRunBlocked(t *testing.T) {
	m := newTestModel(t)
	p, err := m.store.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Solar", Agent: store.AgentClaude, Mode: store.ModeYolo, Status: store.StatusInProgress})
	if err != nil {
		t.Fatal(err)
	}
	wt := t.TempDir()
	logPath := filepath.Join(wt, "agent.log")
	_ = os.WriteFile(logPath, []byte("half way\n"), 0o644)
	run, err := m.store.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, State: store.RunRunning, Worktree: wt, LogPath: logPath, Agent: store.AgentClaude})
	if err != nil {
		t.Fatal(err)
	}
	cmds := reconcileRuns(m.store)
	if len(cmds) != 1 {
		t.Fatalf("cmds=%d", len(cmds))
	}
	msg, ok := cmds[0]().(agentDoneMsg)
	if !ok || msg.runID != run.ID || msg.err == nil {
		t.Fatalf("msg=%#v", msg)
	}
	before := m.screen
	next, _ := m.Update(msg)
	got := next.(Model)
	saved, _ := got.store.GetRun(run.ID)
	if saved.State != store.RunBlocked || !strings.Contains(saved.Note, "x") {
		t.Fatalf("run=%+v", saved)
	}
	tk, _ := got.store.GetTask(task.ID)
	if tk.Status != store.StatusBlocked {
		t.Fatalf("task status %s", tk.Status)
	}
	if got.screen != before {
		t.Fatalf("a background run must not change the screen: %v", got.screen)
	}
	if cmds := reconcileRuns(got.store); len(cmds) != 0 {
		t.Fatalf("nothing should be left running: %d", len(cmds))
	}
}

func TestReconcileWatchesLiveAgent(t *testing.T) {
	m := newTestModel(t)
	p, _ := m.store.Create(store.Project{Name: "App", RepoPath: t.TempDir()})
	task, _ := m.store.CreateTask(store.Task{ProjectID: p.ID, Title: "Live", Agent: store.AgentClaude, Mode: store.ModeYolo, Status: store.StatusInProgress})
	wt := t.TempDir()
	run, err := m.store.CreateRun(store.Run{TaskID: task.ID, ProjectID: p.ID, State: store.RunRunning, Worktree: wt, LogPath: filepath.Join(wt, "agent.log"), Agent: store.AgentClaude})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", "sleep 0.4; echo ok > SUMMARY.md")
	cmd.Dir = wt
	if err := launch.StartAgent(cmd, wt, run.LogPath); err != nil {
		t.Fatal(err)
	}
	if st, _ := launch.AgentStatus(wt); st != launch.AgentRunning {
		t.Fatal("agent should be running")
	}
	if cmds := reconcileRuns(m.store); len(cmds) != 1 {
		t.Fatalf("cmds=%d", len(cmds))
	}
	msg := runAgent(cmd, "", run.ID)()
	if done, ok := msg.(agentDoneMsg); !ok || done.err != nil {
		t.Fatalf("msg=%#v", msg)
	}
}
