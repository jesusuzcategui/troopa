package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestProfileGoesIntoTASK(t *testing.T) {
	dir := t.TempDir()
	p := store.Profile{Name: "Ana", Profession: "Chef", About: "Runs a bakery.", Instructions: "Metric units."}
	if err := WriteProfile(dir, &p); err != nil {
		t.Fatal(err)
	}
	if err := WriteTASK(dir, store.Task{Title: "Menu", Commits: store.CommitsNone}, nil); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "TASK.md"))
	for _, want := range []string{"## Who you work for", "Ana (Chef)", "Runs a bakery.", "Metric units."} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("missing %q in\n%s", want, raw)
		}
	}
	if err := WriteProfile(dir, nil); err != nil {
		t.Fatal(err)
	}
	_ = WriteTASK(dir, store.Task{Title: "Menu"}, nil)
	raw, _ = os.ReadFile(filepath.Join(dir, "TASK.md"))
	if strings.Contains(string(raw), "Who you work for") {
		t.Fatal("profile should be gone")
	}
}
