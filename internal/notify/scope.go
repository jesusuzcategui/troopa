package notify

import (
	"strconv"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/store"
)

// Other people may use the bot ([[telegram.users]]): they see and act on
// their projects only. The owner (b.user == nil) sees everything.

func (b *Bot) canProject(id int64) bool {
	if b.user == nil {
		return true
	}
	p, err := b.eng.Store.Get(id)
	return err == nil && b.user.Allows(p.Slug, p.Name)
}

// visibleProjects are the projects the current chat may use.
func (b *Bot) visibleProjects() []store.Project {
	all, err := b.eng.Store.List(false)
	if err != nil {
		return nil
	}
	if b.user == nil {
		return all
	}
	var out []store.Project
	for _, p := range all {
		if b.user.Allows(p.Slug, p.Name) {
			out = append(out, p)
		}
	}
	return out
}

func (b *Bot) runProject(id int64) (int64, bool) {
	r, err := b.eng.Store.GetRun(id)
	return r.ProjectID, err == nil
}

func (b *Bot) planProject(id int64) (int64, bool) {
	p, err := b.eng.Store.GetPlan(id)
	return p.ProjectID, err == nil
}

func (b *Bot) taskProject(id int64) (int64, bool) {
	t, err := b.eng.Store.GetTask(id)
	return t.ProjectID, err == nil
}

// callbackAllowed checks the project behind a button before acting on it.
func (b *Bot) callbackAllowed(data string) bool {
	if b.user == nil {
		return true
	}
	for _, part := range strings.Split(data, "|") {
		action, idText, _ := strings.Cut(part, ":")
		if idText == "" {
			continue
		}
		var projectID int64
		var ok bool
		switch action {
		case "approve", "changes", "diff", "log", "comment", "sendc", "merge", "pr", "domerge", "dopr", "imgs":
			id, _ := strconv.ParseInt(idText, 10, 64)
			projectID, ok = b.runProject(id)
		case "plan", "pstart", "ppause", "pmerge", "ppr", "dopmerge", "doppr":
			id, _ := strconv.ParseInt(idText, 10, 64)
			projectID, ok = b.planProject(id)
		case "task", "launch":
			first, _, _ := strings.Cut(idText, ":")
			id, _ := strconv.ParseInt(first, 10, 64)
			projectID, ok = b.taskProject(id)
		case "pull", "newtask", "photop":
			projectID, _ = strconv.ParseInt(idText, 10, 64)
			ok = true
		case "chatp":
			_, pid, _ := strings.Cut(idText, ":")
			projectID, _ = strconv.ParseInt(pid, 10, 64)
			ok = true
		default:
			continue
		}
		if !ok || !b.canProject(projectID) {
			return false
		}
	}
	return true
}

// commandAllowed checks the id a command names.
func (b *Bot) commandAllowed(kind string, id int64) bool {
	if b.user == nil {
		return true
	}
	var projectID int64
	var ok bool
	switch kind {
	case "run":
		projectID, ok = b.runProject(id)
	case "plan":
		projectID, ok = b.planProject(id)
	case "task":
		projectID, ok = b.taskProject(id)
	}
	return ok && b.canProject(projectID)
}

// projectSet is the set of visible project ids; nil means all (the owner).
func (b *Bot) projectSet() map[int64]bool {
	if b.user == nil {
		return nil
	}
	set := map[int64]bool{}
	for _, p := range b.visibleProjects() {
		set[p.ID] = true
	}
	return set
}

func inSet(set map[int64]bool, id int64) bool { return set == nil || set[id] }
