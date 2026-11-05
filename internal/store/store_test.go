package store

import (
	"path/filepath"
	"testing"
)

func TestCreateListUpdateArchivePersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "troopa.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	p, err := s.Create(Project{
		Name:        "Troppa",
		Slug:        "",
		RepoPath:    "/tmp/troppa",
		Stack:       "Go",
		Description: "TUI",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected id")
	}
	if p.Slug != "troppa" {
		t.Fatalf("slug = %q, want troppa", p.Slug)
	}

	list, err := s.List(false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Troppa" {
		t.Fatalf("list = %+v", list)
	}

	p.Name = "Troppa App"
	p.Stack = "Go · Bubble Tea"
	if err := s.Update(p); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := s.Archive(p.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	active, err := s.List(false)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active after archive = %+v", active)
	}
	all, err := s.List(true)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 1 || !all[0].Archived {
		t.Fatalf("all = %+v", all)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	again, err := s2.List(true)
	if err != nil {
		t.Fatalf("list after reopen: %v", err)
	}
	if len(again) != 1 || again[0].Name != "Troppa App" || again[0].Stack != "Go · Bubble Tea" || !again[0].Archived {
		t.Fatalf("persist = %+v", again)
	}
}

func TestCreateRejectsDuplicateSlug(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	if _, err := s.Create(Project{Name: "A", Slug: "same", RepoPath: "/a"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := s.Create(Project{Name: "B", Slug: "same", RepoPath: "/b"}); err == nil {
		t.Fatal("expected duplicate slug error")
	}
}

func TestCreateRequiresNameAndPath(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	if _, err := s.Create(Project{RepoPath: "/x"}); err == nil {
		t.Fatal("expected name error")
	}
	if _, err := s.Create(Project{Name: "X"}); err == nil {
		t.Fatal("expected path error")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Troppa", "troppa"},
		{"  Go App  ", "go-app"},
		{"A/B_c", "a-b-c"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := Slugify(tt.in); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
