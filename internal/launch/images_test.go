package launch

import (
	"os/exec"
	"strings"
	"testing"
)

func TestAddImagesCodexOnly(t *testing.T) {
	cmd := exec.Command("codex", "exec", "--dangerously-bypass-approvals-and-sandbox", "do it")
	AddImages(cmd, "codex", []string{"/a.png", "/b.jpg"})
	if got := strings.Join(cmd.Args, " "); got != "codex exec --image=/a.png --image=/b.jpg --dangerously-bypass-approvals-and-sandbox do it" {
		t.Fatalf("args %q", got)
	}
	other := exec.Command("claude", "-p", "x")
	AddImages(other, "claude", []string{"/a.png"})
	if len(other.Args) != 3 {
		t.Fatalf("claude args changed: %v", other.Args)
	}
}
