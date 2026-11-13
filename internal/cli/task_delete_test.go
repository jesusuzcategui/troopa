package cli

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jesusuzcategui/troopa/internal/store"
)

func TestTaskDelete(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.Create(store.Project{Name: "App", RepoPath: "/tmp/app"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(store.Task{ProjectID: p.ID, Title: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(s, []string{"task", "delete", "--json", "--id", fmt.Sprint(task.ID)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTask(task.ID); err == nil {
		t.Fatal("task should be gone")
	}
	if _, err := Execute(s, []string{"task", "delete", "--json", "--id", fmt.Sprint(task.ID)}); err == nil {
		t.Fatal("deleting again must fail with not found")
	}
}
