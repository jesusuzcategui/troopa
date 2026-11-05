// Package cli implements Troopa's versioned machine-readable command contract.
package cli

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/store"
)

const SchemaVersion = 1

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string   { return e.Message }
func input(message string) error { return &Error{Code: "invalid_argument", Message: message} }
func notFound(kind string) error { return &Error{Code: "not_found", Message: kind + " not found"} }

// Execute performs a non-interactive command using the same store layer as the TUI.
func Execute(s *store.Store, args []string) (any, error) {
	if len(args) < 2 {
		return nil, input("expected a resource and command")
	}
	resource, command := args[0], args[1]
	rest := removeJSON(args[2:])
	switch resource {
	case "project":
		return project(s, command, rest)
	case "client":
		return client(s, command, rest)
	case "task":
		return task(s, command, rest)
	case "run":
		return run(s, command, rest)
	default:
		return nil, input("unknown resource: " + resource)
	}
}
func removeJSON(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a != "--json" {
			out = append(out, a)
		}
	}
	return out
}
func flags(name string, args []string) (*flag.FlagSet, error) {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(new(strings.Builder))
	return f, nil
}
func id(f *flag.FlagSet, name string) (int64, error) {
	v := f.Lookup(name).Value.String()
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 1 {
		return 0, input("--" + name + " must be a positive integer")
	}
	return n, nil
}
func optionalID(f *flag.FlagSet, name string) (int64, error) {
	v := f.Lookup(name).Value.String()
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 1 {
		return 0, input("--" + name + " must be a positive integer")
	}
	return n, nil
}
func getProject(s *store.Store, n int64) (store.Project, error) {
	p, e := s.Get(n)
	if errors.Is(e, sql.ErrNoRows) {
		return p, notFound("project")
	}
	return p, e
}
func getClient(s *store.Store, n int64) (store.Client, error) {
	c, e := s.GetClient(n)
	if errors.Is(e, sql.ErrNoRows) {
		return c, notFound("client")
	}
	return c, e
}
func getTask(s *store.Store, n int64) (store.Task, error) {
	t, e := s.GetTask(n)
	if errors.Is(e, sql.ErrNoRows) {
		return t, notFound("task")
	}
	return t, e
}
func getRun(s *store.Store, n int64) (store.Run, error) {
	r, e := s.GetRun(n)
	if errors.Is(e, sql.ErrNoRows) {
		return r, notFound("run")
	}
	return r, e
}

func project(s *store.Store, cmd string, args []string) (any, error) {
	f, e := flags("project "+cmd, args)
	if e != nil {
		return nil, e
	}
	archived := f.Bool("archived", false, "")
	_ = f.String("client", "", "")
	_ = f.String("id", "", "")
	name := f.String("name", "", "")
	slug := f.String("slug", "", "")
	repo := f.String("repo", "", "")
	stack := f.String("stack", "", "")
	desc := f.String("description", "", "")
	if e := f.Parse(args); e != nil {
		return nil, input(e.Error())
	}
	if f.NArg() != 0 {
		return nil, input("unexpected argument: " + f.Arg(0))
	}
	switch cmd {
	case "list":
		cid, e := optionalID(f, "client")
		if e != nil {
			return nil, e
		}
		items, err := s.ListFiltered(store.ProjectFilter{IncludeArchived: *archived, ClientID: cid})
		return projects(items), err
	case "show":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		return getProject(s, n)
	case "create":
		cid, e := optionalID(f, "client")
		if e != nil {
			return nil, e
		}
		return s.Create(store.Project{Name: *name, Slug: *slug, RepoPath: *repo, Stack: *stack, Description: *desc, ClientID: cid})
	case "update":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		p, e := getProject(s, n)
		if e != nil {
			return nil, e
		}
		patchProject(&p, f)
		if e = s.Update(p); e != nil {
			return nil, e
		}
		return getProject(s, n)
	case "archive":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		if e = s.Archive(n); e != nil {
			return nil, normalize(e, "project")
		}
		return map[string]any{"id": n, "archived": true}, nil
	default:
		return nil, input("unknown project command: " + cmd)
	}
}
func patchProject(p *store.Project, f *flag.FlagSet) {
	if seen(f, "name") {
		p.Name = f.Lookup("name").Value.String()
	}
	if seen(f, "slug") {
		p.Slug = f.Lookup("slug").Value.String()
	}
	if seen(f, "repo") {
		p.RepoPath = f.Lookup("repo").Value.String()
	}
	if seen(f, "stack") {
		p.Stack = f.Lookup("stack").Value.String()
	}
	if seen(f, "description") {
		p.Description = f.Lookup("description").Value.String()
	}
	if seen(f, "client") {
		p.ClientID, _ = optionalID(f, "client")
	}
}
func seen(f *flag.FlagSet, name string) bool {
	ok := false
	f.Visit(func(x *flag.Flag) {
		if x.Name == name {
			ok = true
		}
	})
	return ok
}

func client(s *store.Store, cmd string, args []string) (any, error) {
	f, e := flags("client "+cmd, args)
	if e != nil {
		return nil, e
	}
	archived := f.Bool("archived", false, "")
	cid := f.String("id", "", "")
	name := f.String("name", "", "")
	contact := f.String("contact", "", "")
	notes := f.String("notes", "", "")
	_ = cid
	if e := f.Parse(args); e != nil {
		return nil, input(e.Error())
	}
	if f.NArg() != 0 {
		return nil, input("unexpected argument: " + f.Arg(0))
	}
	switch cmd {
	case "list":
		items, err := s.ListClients(*archived)
		return clients(items), err
	case "show":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		return getClient(s, n)
	case "create":
		return s.CreateClient(store.Client{Name: *name, Contact: *contact, Notes: *notes})
	case "update":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		c, e := getClient(s, n)
		if e != nil {
			return nil, e
		}
		if seen(f, "name") {
			c.Name = *name
		}
		if seen(f, "contact") {
			c.Contact = *contact
		}
		if seen(f, "notes") {
			c.Notes = *notes
		}
		if e = s.UpdateClient(c); e != nil {
			return nil, e
		}
		return getClient(s, n)
	default:
		return nil, input("unknown client command: " + cmd)
	}
}

func task(s *store.Store, cmd string, args []string) (any, error) {
	f, e := flags("task "+cmd, args)
	if e != nil {
		return nil, e
	}
	tid := f.String("id", "", "")
	_ = f.String("project", "", "")
	_ = f.String("client", "", "")
	status := f.String("status", "", "")
	priority := f.String("priority", "", "")
	title := f.String("title", "", "")
	desc := f.String("description", "", "")
	criteria := f.String("criteria", "", "")
	agent := f.String("agent", "", "")
	model := f.String("model", "", "")
	mode := f.String("mode", "", "")
	commits := f.String("commits", "", "")
	typ := f.String("type", "", "")
	_ = tid
	if e := f.Parse(args); e != nil {
		return nil, input(e.Error())
	}
	if f.NArg() != 0 {
		return nil, input("unexpected argument: " + f.Arg(0))
	}
	switch cmd {
	case "list":
		p, e := optionalID(f, "project")
		if e != nil {
			return nil, e
		}
		c, e := optionalID(f, "client")
		if e != nil {
			return nil, e
		}
		items, err := s.ListTasks(p, store.TaskFilter{Status: *status, Priority: *priority, ClientID: c})
		return tasks(items), err
	case "show":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		return getTask(s, n)
	case "create":
		p, e := id(f, "project")
		if e != nil {
			return nil, e
		}
		return s.CreateTask(store.Task{ProjectID: p, Title: *title, Description: *desc, AcceptanceCriteria: *criteria, Status: *status, Priority: *priority, Agent: *agent, Model: *model, Mode: *mode, Commits: *commits, Type: *typ})
	case "update":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		t, e := getTask(s, n)
		if e != nil {
			return nil, e
		}
		patchTask(&t, f)
		if e = s.UpdateTask(t); e != nil {
			return nil, e
		}
		return getTask(s, n)
	case "move", "launch":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		dest := *status
		if cmd == "launch" && dest == "" {
			dest = store.StatusReady
		}
		if dest == "" {
			return nil, input("--status is required")
		}
		if e = s.MoveTask(n, dest); e != nil {
			return nil, normalize(e, "task")
		}
		return getTask(s, n)
	case "approve", "discard":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		return transitionTaskRun(s, n, cmd)
	default:
		return nil, input("unknown task command: " + cmd)
	}
}
func patchTask(t *store.Task, f *flag.FlagSet) {
	if seen(f, "project") {
		t.ProjectID, _ = optionalID(f, "project")
	}
	if seen(f, "title") {
		t.Title = f.Lookup("title").Value.String()
	}
	if seen(f, "description") {
		t.Description = f.Lookup("description").Value.String()
	}
	if seen(f, "criteria") {
		t.AcceptanceCriteria = f.Lookup("criteria").Value.String()
	}
	if seen(f, "status") {
		t.Status = f.Lookup("status").Value.String()
	}
	if seen(f, "priority") {
		t.Priority = f.Lookup("priority").Value.String()
	}
	if seen(f, "agent") {
		t.Agent = f.Lookup("agent").Value.String()
	}
	if seen(f, "model") {
		t.Model = f.Lookup("model").Value.String()
	}
	if seen(f, "mode") {
		t.Mode = f.Lookup("mode").Value.String()
	}
	if seen(f, "commits") {
		t.Commits = f.Lookup("commits").Value.String()
	}
	if seen(f, "type") {
		t.Type = f.Lookup("type").Value.String()
	}
}
func transitionTaskRun(s *store.Store, taskID int64, action string) (any, error) {
	t, e := getTask(s, taskID)
	if e != nil {
		return nil, e
	}
	r, e := s.LatestRun(taskID)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, notFound("run for task")
	}
	if e != nil {
		return nil, e
	}
	if action == "approve" {
		r.State = store.RunApproved
		e = s.MoveTask(taskID, store.StatusDone)
	} else {
		r.State = store.RunDiscarded
		e = s.MoveTask(taskID, store.StatusBacklog)
	}
	if e != nil {
		return nil, e
	}
	if e = s.UpdateRun(r); e != nil {
		return nil, e
	}
	return map[string]any{"task": t.ID, "run": r.ID, "state": r.State}, nil
}

func run(s *store.Store, cmd string, args []string) (any, error) {
	f, e := flags("run "+cmd, args)
	if e != nil {
		return nil, e
	}
	rid := f.String("id", "", "")
	state := f.String("state", "", "")
	_ = rid
	if e := f.Parse(args); e != nil {
		return nil, input(e.Error())
	}
	if f.NArg() != 0 {
		return nil, input("unexpected argument: " + f.Arg(0))
	}
	switch cmd {
	case "list":
		items, err := s.ListRuns(*state)
		return runs(items), err
	case "show":
		n, e := id(f, "id")
		if e != nil {
			return nil, e
		}
		return getRun(s, n)
	default:
		return nil, input("unknown run command: " + cmd)
	}
}
func normalize(err error, kind string) error {
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no encontrado") {
		return notFound(kind)
	}
	return err
}
func Usage() string { return "usage: troopa <project|client|task|run> <command> --json [flags]" }
func ValidateJSON(args []string) error {
	for _, a := range args {
		if a == "--json" {
			return nil
		}
	}
	return fmt.Errorf("--json is required for non-interactive commands")
}

func projects(items []store.Project) []store.Project {
	if items == nil {
		return []store.Project{}
	}
	return items
}
func clients(items []store.Client) []store.Client {
	if items == nil {
		return []store.Client{}
	}
	return items
}
func tasks(items []store.Task) []store.Task {
	if items == nil {
		return []store.Task{}
	}
	return items
}
func runs(items []store.Run) []store.Run {
	if items == nil {
		return []store.Run{}
	}
	return items
}
