package launch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// Hook events. A hook is a shell command from [hooks] in config.toml or in
// the project's troopa.toml; it gets the event as JSON on stdin and in
// TROOPA_* variables. Hooks run in the background and never block Troopa.
const (
	HookTaskCreated = "task_created"
	HookLaunch      = "launch"
	HookReview      = "review"
	HookBlocked     = "blocked"
	HookApprove     = "approve"
	HookMerge       = "merge"
	HookDiscard     = "discard"
	HookPlanStart   = "plan_start"
	HookPlanDone    = "plan_done"
)

var HookEvents = []string{HookTaskCreated, HookLaunch, HookReview, HookBlocked, HookApprove, HookMerge, HookDiscard, HookPlanStart, HookPlanDone}

// HookEvent is what a hook receives.
type HookEvent struct {
	Event    string `json:"event"`
	Project  string `json:"project,omitempty"`
	Repo     string `json:"repo,omitempty"`
	TaskID   int64  `json:"task_id,omitempty"`
	Task     string `json:"task,omitempty"`
	Status   string `json:"status,omitempty"`
	Agent    string `json:"agent,omitempty"`
	RunID    int64  `json:"run_id,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	PlanID   int64  `json:"plan_id,omitempty"`
	Plan     string `json:"plan,omitempty"`
	Message  string `json:"message,omitempty"`
}

const hookTimeout = 2 * time.Minute

var hookWG sync.WaitGroup

// RunHooks starts the commands of an event (global first, then the
// project's) in the background. Output goes to logPath.
func RunHooks(global, project map[string][]string, ev HookEvent, logPath string) int {
	cmds := append(append([]string{}, global[ev.Event]...), project[ev.Event]...)
	for _, c := range cmds {
		hookWG.Add(1)
		go func(c string) {
			defer hookWG.Done()
			runHook(c, ev, logPath)
		}(c)
	}
	return len(cmds)
}

// WaitHooks waits for running hooks (tests, and on exit).
func WaitHooks() { hookWG.Wait() }

func runHook(command string, ev HookEvent, logPath string) {
	payload, _ := json.Marshal(ev)
	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	}
	if ev.Worktree != "" {
		cmd.Dir = ev.Worktree
	} else if ev.Repo != "" {
		cmd.Dir = ev.Repo
	}
	cmd.Env = append(os.Environ(),
		"TROOPA_EVENT="+ev.Event,
		"TROOPA_PROJECT="+ev.Project,
		"TROOPA_REPO="+ev.Repo,
		"TROOPA_TASK_ID="+strconv.FormatInt(ev.TaskID, 10),
		"TROOPA_TASK="+ev.Task,
		"TROOPA_STATUS="+ev.Status,
		"TROOPA_AGENT="+ev.Agent,
		"TROOPA_RUN_ID="+strconv.FormatInt(ev.RunID, 10),
		"TROOPA_BRANCH="+ev.Branch,
		"TROOPA_WORKTREE="+ev.Worktree,
		"TROOPA_PLAN_ID="+strconv.FormatInt(ev.PlanID, 10),
		"TROOPA_PLAN="+ev.Plan,
	)
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.CombinedOutput()
	if logPath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	f, ferr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if ferr != nil {
		return
	}
	defer f.Close()
	status := "ok"
	if err != nil {
		status = err.Error()
	}
	fmt.Fprintf(f, "%s %s [%s] %s\n%s", time.Now().Format(time.RFC3339), ev.Event, status, command, out)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		fmt.Fprintln(f)
	}
}
