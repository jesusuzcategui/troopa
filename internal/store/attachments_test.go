package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachListDetach(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	task, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Docs"})
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "attachments", "1")
	a, err := s.AttachFile(task.ID, src, dest, 50<<20)
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "note.txt" || a.Size != 5 || !strings.Contains(a.MIME, "text") {
		t.Fatalf("got %+v", a)
	}
	if _, err := os.Stat(a.Path); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListAttachments(task.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list %+v %v", list, err)
	}
	if err := s.DeleteAttachment(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.Path); !os.IsNotExist(err) {
		t.Fatal("file should be gone")
	}
	list, err = s.ListAttachments(task.ID)
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete %+v %v", list, err)
	}
}

func TestAttachRejectsOversize(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := testProject(t, s)
	task, err := s.CreateTask(Task{ProjectID: p.ID, Title: "Big"})
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(src, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = s.AttachFile(task.ID, src, t.TempDir(), 5)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("want too large, got %v", err)
	}
}
