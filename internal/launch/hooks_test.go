//go:build unix

package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHooks(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	logPath := filepath.Join(dir, "hooks.log")
	global := map[string][]string{HookReview: {`cat > ` + out + `; echo "$TROOPA_TASK" >> ` + out}}
	project := map[string][]string{HookReview: {"echo project"}, HookApprove: {"echo nope"}}
	n := RunHooks(global, project, HookEvent{Event: HookReview, TaskID: 7, Task: "Cart"}, logPath)
	WaitHooks()
	if n != 2 {
		t.Fatalf("ran %d", n)
	}
	raw, _ := os.ReadFile(out)
	if !strings.Contains(string(raw), `"task_id":7`) || !strings.Contains(string(raw), "Cart") {
		t.Fatalf("hook input: %s", raw)
	}
	log, _ := os.ReadFile(logPath)
	if !strings.Contains(string(log), "project") || strings.Contains(string(log), "nope") {
		t.Fatalf("log: %s", log)
	}
}
