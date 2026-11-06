package cli

import (
	"github.com/jesusuzcategui/troopa/internal/store"
)

type planDetail struct {
	store.Plan
	Done  int          `json:"done"`
	Total int          `json:"total"`
	Steps []store.Task `json:"steps"`
}

func planDetailOf(s *store.Store, p store.Plan) (planDetail, error) {
	steps, err := s.PlanSteps(p.ID)
	if err != nil {
		return planDetail{}, err
	}
	d := planDetail{Plan: p, Steps: tasks(steps), Total: len(steps)}
	for _, t := range steps {
		if t.Status == store.StatusDone {
			d.Done++
		}
	}
	return d, nil
}

// planCmd: plan list|show|create|add-step|delete. Starting a plan needs the
// TUI (it launches agents and reviews each step).
func planCmd(s *store.Store, cmd string, args []string) (any, error) {
	f, e := flags("plan "+cmd, args)
	if e != nil {
		return nil, e
	}
	_ = f.String("id", "", "")
	_ = f.String("project", "", "")
	title := f.String("title", "", "")
	goal := f.String("goal", "", "")
	agent := f.String("agent", "", "")
	model := f.String("model", "", "")
	desc := f.String("description", "", "")
	crit := f.String("criteria", "", "")
	manual := f.Bool("manual", false, "")
	autoApprove := f.Bool("auto-approve", false, "")
	if e := f.Parse(args); e != nil {
		return nil, input(e.Error())
	}
	if f.NArg() != 0 {
		return nil, input("unexpected argument: " + f.Arg(0))
	}
	getPlan := func() (store.Plan, error) {
		n, e := id(f, "id")
		if e != nil {
			return store.Plan{}, e
		}
		p, err := s.GetPlan(n)
		if err != nil {
			return store.Plan{}, notFound("plan")
		}
		return p, nil
	}
	switch cmd {
	case "list":
		pid, e := optionalID(f, "project")
		if e != nil {
			return nil, e
		}
		list, err := s.ListPlans(pid)
		if err != nil {
			return nil, err
		}
		out := []planDetail{}
		for _, p := range list {
			d, err := planDetailOf(s, p)
			if err != nil {
				return nil, err
			}
			d.Steps = nil
			out = append(out, d)
		}
		return out, nil
	case "show":
		p, e := getPlan()
		if e != nil {
			return nil, e
		}
		return planDetailOf(s, p)
	case "update":
		p, e := getPlan()
		if e != nil {
			return nil, e
		}
		if seen(f, "title") {
			p.Title = *title
		}
		if seen(f, "goal") {
			p.Goal = *goal
		}
		if seen(f, "agent") {
			p.Agent = *agent
		}
		if seen(f, "model") {
			p.Model = *model
		}
		if seen(f, "manual") {
			p.Auto = !*manual
		}
		if seen(f, "auto-approve") {
			p.AutoApprove = *autoApprove
		}
		if err := s.UpdatePlan(p); err != nil {
			return nil, input(err.Error())
		}
		return planDetailOf(s, p)
	case "create":
		pid, e := id(f, "project")
		if e != nil {
			return nil, e
		}
		if _, err := getProject(s, pid); err != nil {
			return nil, err
		}
		p, err := s.CreatePlan(store.Plan{ProjectID: pid, Title: *title, Goal: *goal, Agent: *agent, Model: *model, Auto: !*manual, AutoApprove: *autoApprove})
		if err != nil {
			return nil, input(err.Error())
		}
		return planDetailOf(s, p)
	case "add-step":
		p, e := getPlan()
		if e != nil {
			return nil, e
		}
		t, err := s.AddPlanStep(p.ID, store.Task{Title: *title, Description: *desc, AcceptanceCriteria: *crit, Agent: *agent, Model: *model})
		if err != nil {
			return nil, input(err.Error())
		}
		rememberTask(s, t)
		return t, nil
	case "delete":
		p, e := getPlan()
		if e != nil {
			return nil, e
		}
		if err := s.DeletePlan(p.ID); err != nil {
			return nil, err
		}
		return map[string]any{"id": p.ID, "deleted": true}, nil
	default:
		return nil, input("unknown plan command: " + cmd)
	}
}

// profileCmd: profile list|show|create|update|delete.
func profileCmd(s *store.Store, cmd string, args []string) (any, error) {
	f, e := flags("profile "+cmd, args)
	if e != nil {
		return nil, e
	}
	_ = f.String("id", "", "")
	name := f.String("name", "", "")
	prof := f.String("profession", "", "")
	about := f.String("about", "", "")
	instr := f.String("instructions", "", "")
	if e := f.Parse(args); e != nil {
		return nil, input(e.Error())
	}
	if f.NArg() != 0 {
		return nil, input("unexpected argument: " + f.Arg(0))
	}
	get := func() (store.Profile, error) {
		n, e := id(f, "id")
		if e != nil {
			return store.Profile{}, e
		}
		p, err := s.GetProfile(n)
		if err != nil {
			return store.Profile{}, notFound("profile")
		}
		return p, nil
	}
	switch cmd {
	case "list":
		list, err := s.ListProfiles()
		if list == nil {
			list = []store.Profile{}
		}
		return list, err
	case "show":
		return get()
	case "create":
		p, err := s.CreateProfile(store.Profile{Name: *name, Profession: *prof, About: *about, Instructions: *instr})
		if err != nil {
			return nil, input(err.Error())
		}
		return p, nil
	case "update":
		p, e := get()
		if e != nil {
			return nil, e
		}
		if seen(f, "name") {
			p.Name = *name
		}
		if seen(f, "profession") {
			p.Profession = *prof
		}
		if seen(f, "about") {
			p.About = *about
		}
		if seen(f, "instructions") {
			p.Instructions = *instr
		}
		if err := s.UpdateProfile(p); err != nil {
			return nil, input(err.Error())
		}
		return s.GetProfile(p.ID)
	case "delete":
		p, e := get()
		if e != nil {
			return nil, e
		}
		if err := s.DeleteProfile(p.ID); err != nil {
			return nil, err
		}
		return map[string]any{"id": p.ID, "deleted": true}, nil
	default:
		return nil, input("unknown profile command: " + cmd)
	}
}
