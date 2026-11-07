package notify

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// The bot's screens. Each one is a text plus inline buttons; tapping a
// button edits the same message, so the chat works like a small app with no
// server: plans → plan → approve, request changes, start or pause.

func btn(text, data string) tgButton { return tgButton{Text: text, Data: data} }

// show sends a screen (msgID 0) or redraws it in place.
func (b *Bot) show(ctx context.Context, msgID int64, view string) {
	text, kb := b.view(view)
	if msgID != 0 {
		if err := b.api.edit(ctx, b.chat, msgID, text, kb); err == nil {
			return
		}
	}
	if _, err := b.api.send(ctx, b.chat, text, kb, 0); err != nil {
		b.logger.Printf("telegram send: %v", err)
	}
}

func (b *Bot) view(view string) (string, *tgKeyboard) {
	name, idText, _ := strings.Cut(view, ":")
	switch name {
	case "status":
		return b.statusView()
	case "plan":
		if id, err := strconv.ParseInt(idText, 10, 64); err == nil {
			return b.planView(id)
		}
	case "newtask":
		return b.projectsView()
	case "tasks":
		return b.tasksView()
	case "task":
		if id, err := strconv.ParseInt(idText, 10, 64); err == nil {
			return b.taskView(id, "")
		}
	}
	return b.plansView()
}

// mainKeyboard is the persistent keyboard under the text box.
func mainKeyboard() replyKeyboard {
	return replyKeyboard{Resize: true, Persistent: true, Rows: [][]replyButton{
		{{Text: "📊 " + i18n.T("Status")}, {Text: "🗺 " + i18n.T("Plans")}},
		{{Text: "📋 " + i18n.T("Tasks")}, {Text: "➕ " + i18n.T("New task")}},
		{{Text: "❓ " + i18n.T("Help")}},
	}}
}

// keyboardView maps a tap on the persistent keyboard to a screen, whatever
// language the keyboard was drawn in.
func keyboardView(text string) (string, bool) {
	for prefix, view := range map[string]string{"📊": "status", "🗺": "plans", "📋": "tasks", "➕": "newtask", "❓": "help"} {
		if strings.HasPrefix(text, prefix) {
			return view, true
		}
	}
	return "", false
}

func (b *Bot) sendHelp(ctx context.Context) {
	if _, err := b.api.send(ctx, b.chat, b.help(), mainKeyboard(), 0); err != nil {
		b.logger.Printf("telegram send: %v", err)
	}
}

// commands fill Telegram's menu button.
func (b *Bot) commands() []tgCommand {
	return []tgCommand{
		{"status", i18n.T("what is running, waiting for review or blocked")},
		{"plans", i18n.T("plans and their progress")},
		{"tasks", i18n.T("pick a pending task and launch it")},
		{"task", i18n.T("add a task to a project's backlog")},
		{"plan", i18n.T("the steps of a plan")},
		{"run", i18n.T("start or resume a plan")},
		{"pause", i18n.T("pause a plan")},
		{"log", i18n.T("the log of a run")},
		{"diff", i18n.T("the diff of a run")},
		{"help", i18n.T("what I can do")},
	}
}

func (b *Bot) statusView() (string, *tgKeyboard) {
	s := b.eng.Store
	var out []string
	kb := &tgKeyboard{}
	if runs, err := s.ListRuns(store.RunRunning); err == nil {
		for _, r := range runs {
			t, _ := s.GetTask(r.TaskID)
			out = append(out, fmt.Sprintf("🔄 #%d %s · %s", r.ID, esc(t.Title), esc(launch.AgentName(r.Agent))))
		}
	}
	for _, state := range []string{store.RunReview, store.RunBlocked} {
		runs, err := s.ListRuns(state)
		if err != nil {
			continue
		}
		shown := 0
		for _, r := range runs {
			if shown == 5 {
				break
			}
			t, _ := s.GetTask(r.TaskID)
			if t.Status == store.StatusDone {
				continue
			}
			shown++
			id := strconv.FormatInt(r.ID, 10)
			if state == store.RunReview {
				out = append(out, fmt.Sprintf("👀 #%d %s", r.ID, esc(t.Title)))
				kb.Rows = append(kb.Rows, []tgButton{
					btn("✅ #"+id, "approve:"+id+"|status"), btn("✏️ #"+id, "changes:"+id), btn("📄 #"+id, "diff:"+id),
				})
			} else {
				out = append(out, fmt.Sprintf("⛔ #%d %s", r.ID, esc(t.Title)))
				kb.Rows = append(kb.Rows, []tgButton{btn("✏️ #"+id, "changes:"+id), btn("📜 #"+id, "log:"+id)})
			}
		}
	}
	if plans, err := s.ListPlans(0); err == nil {
		for _, p := range plans {
			if p.State == store.PlanRunning || p.State == store.PlanPaused {
				done, total, _ := s.PlanProgress(p.ID)
				out = append(out, fmt.Sprintf("🗺 %s · %d/%d · %s", esc(p.Title), done, total, esc(p.State)))
				kb.Rows = append(kb.Rows, []tgButton{btn("🗺 "+short(p.Title, 40), "plan:"+strconv.FormatInt(p.ID, 10))})
			}
		}
	}
	if len(out) == 0 {
		out = append(out, esc(i18n.T("Nothing running and nothing waiting for you.")))
	}
	kb.Rows = append(kb.Rows, []tgButton{btn("🔄 "+i18n.T("Refresh"), "status"), btn("🗺 "+i18n.T("Plans"), "plans"), btn("📋 "+i18n.T("Tasks"), "tasks")})
	return "📊 <b>" + esc(i18n.T("Status")) + "</b>\n" + strings.Join(out, "\n"), kb
}

func (b *Bot) plansView() (string, *tgKeyboard) {
	kb := &tgKeyboard{}
	list, err := b.eng.Store.ListPlans(0)
	if err != nil || len(list) == 0 {
		kb.Rows = [][]tgButton{{btn("📊 "+i18n.T("Status"), "status")}}
		return esc(i18n.T("No plans yet.")), kb
	}
	var out []string
	for i, p := range list {
		if i == 20 {
			break
		}
		done, total, _ := b.eng.Store.PlanProgress(p.ID)
		out = append(out, fmt.Sprintf("<b>%d</b> · %s · %d/%d · %s", p.ID, esc(p.Title), done, total, esc(p.State)))
		kb.Rows = append(kb.Rows, []tgButton{btn(fmt.Sprintf("%s %s · %d/%d", planMark(p.State), short(p.Title, 40), done, total), "plan:"+strconv.FormatInt(p.ID, 10))})
	}
	kb.Rows = append(kb.Rows, []tgButton{btn("🔄 "+i18n.T("Refresh"), "plans"), btn("📊 "+i18n.T("Status"), "status")})
	return "🗺 <b>" + esc(i18n.T("Plans")) + "</b>\n" + strings.Join(out, "\n"), kb
}

func planMark(state string) string {
	switch state {
	case store.PlanRunning:
		return "▶️"
	case store.PlanPaused:
		return "⏸"
	case store.PlanDone, store.PlanMerged:
		return "✅"
	case store.PlanPROpen:
		return "🔀"
	case store.PlanCancelled:
		return "✖️"
	}
	return "📝"
}

func (b *Bot) planView(id int64) (string, *tgKeyboard) {
	back := []tgButton{btn("⬅️ "+i18n.T("Plans"), "plans")}
	p, err := b.eng.Store.GetPlan(id)
	if err != nil {
		return esc(i18n.T("Plan %d not found", id)), &tgKeyboard{Rows: [][]tgButton{back}}
	}
	pid := strconv.FormatInt(id, 10)
	steps, _ := b.eng.Store.PlanSteps(id)
	out := []string{fmt.Sprintf("🗺 <b>%s</b> · %s", esc(p.Title), esc(p.State))}
	kb := &tgKeyboard{}
	for _, t := range steps {
		mark := "○"
		switch t.Status {
		case store.StatusDone:
			mark = "✅"
		case store.StatusInProgress:
			mark = "🔄"
		case store.StatusReview:
			mark = "👀"
		case store.StatusBlocked:
			mark = "⛔"
		}
		out = append(out, fmt.Sprintf("%s %d. %s", mark, t.PlanStep, esc(t.Title)))
		if t.Status != store.StatusReview && t.Status != store.StatusBlocked {
			continue
		}
		run, err := b.eng.Store.LatestRun(t.ID)
		if err != nil {
			continue
		}
		rid := strconv.FormatInt(run.ID, 10)
		n := strconv.Itoa(t.PlanStep)
		switch run.State {
		case store.RunReview:
			kb.Rows = append(kb.Rows, []tgButton{
				btn("✅ "+n+". "+i18n.T("Approve"), "approve:"+rid+"|plan:"+pid), btn("✏️ "+n, "changes:"+rid), btn("📄 "+n, "diff:"+rid),
			})
		case store.RunBlocked:
			kb.Rows = append(kb.Rows, []tgButton{btn("✏️ "+n+". "+i18n.T("Changes"), "changes:"+rid), btn("📜 "+n, "log:"+rid)})
		}
	}
	switch p.State {
	case store.PlanDraft, store.PlanPaused:
		kb.Rows = append(kb.Rows, []tgButton{btn("▶️ "+i18n.T("Start"), "pstart:"+pid)})
	case store.PlanRunning:
		kb.Rows = append(kb.Rows, []tgButton{btn("⏸ "+i18n.T("Pause"), "ppause:"+pid)})
	}
	kb.Rows = append(kb.Rows, []tgButton{btn("🔄 "+i18n.T("Refresh"), "plan:"+pid), back[0]})
	return strings.Join(out, "\n"), kb
}

// projectsView asks which project a new task goes to.
func (b *Bot) projectsView() (string, *tgKeyboard) {
	kb := &tgKeyboard{}
	projects, err := b.eng.Store.List(false)
	if err != nil || len(projects) == 0 {
		return esc(i18n.T("No projects yet.")), kb
	}
	for i, p := range projects {
		if i == 30 {
			break
		}
		kb.Rows = append(kb.Rows, []tgButton{btn(short(p.Name, 50), "newtask:"+strconv.FormatInt(p.ID, 10))})
	}
	return "➕ " + esc(i18n.T("Which project is the new task for?")), kb
}

// askTaskTitle asks for the title once the project was picked.
func (b *Bot) askTaskTitle(ctx context.Context, cq *tgCallback, idText string) {
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		b.api.answer(ctx, cq.ID, "?")
		return
	}
	p, err := b.eng.Store.Get(id)
	if err != nil {
		b.api.answer(ctx, cq.ID, err.Error())
		return
	}
	b.api.answer(ctx, cq.ID, "")
	b.ask(ctx, "➕ "+esc(i18n.T("Reply with the title of the new task for %s.", p.Name)), cq.Message.MessageID, prompt{kind: "task", id: id})
}

// added answers a new task with its launch buttons.
func (b *Bot) added(ctx context.Context, id int64, msg string) {
	if id == 0 {
		b.reply(ctx, msg)
		return
	}
	text, kb := b.taskView(id, msg)
	if _, err := b.api.send(ctx, b.chat, text, kb, 0); err != nil {
		b.logger.Printf("telegram send: %v", err)
	}
}

// pending reports whether a task can be launched from the bot.
func pending(t store.Task) bool {
	return t.Status == store.StatusInbox || t.Status == store.StatusBacklog || t.Status == store.StatusReady
}

// tasksView lists the tasks waiting to start, newest first.
func (b *Bot) tasksView() (string, *tgKeyboard) {
	kb := &tgKeyboard{}
	all, err := b.eng.Store.ListTasks(0, store.TaskFilter{})
	if err != nil {
		return "⚠️ " + esc(err.Error()), kb
	}
	names := map[int64]string{}
	if projects, err := b.eng.Store.List(false); err == nil {
		for _, p := range projects {
			names[p.ID] = p.Name
		}
	}
	var list []store.Task
	for _, t := range all {
		if pending(t) && t.PlanID == 0 {
			list = append(list, t)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID > list[j].ID })
	if len(list) > 12 {
		list = list[:12]
	}
	if len(list) == 0 {
		kb.Rows = [][]tgButton{{btn("➕ "+i18n.T("New task"), "newtask"), btn("📊 "+i18n.T("Status"), "status")}}
		return esc(i18n.T("No tasks waiting to start.")), kb
	}
	out := []string{"📋 <b>" + esc(i18n.T("Tasks to start")) + "</b>"}
	for _, t := range list {
		out = append(out, fmt.Sprintf("#%d %s · <i>%s</i>", t.ID, esc(t.Title), esc(names[t.ProjectID])))
		kb.Rows = append(kb.Rows, []tgButton{btn(fmt.Sprintf("#%d %s", t.ID, short(t.Title, 44)), "task:"+strconv.FormatInt(t.ID, 10))})
	}
	kb.Rows = append(kb.Rows, []tgButton{btn("🔄 "+i18n.T("Refresh"), "tasks"), btn("➕ "+i18n.T("New task"), "newtask")})
	return strings.Join(out, "\n"), kb
}

// launchAgents are the installed agents, the task's own first.
func (b *Bot) launchAgents(first string) []string {
	var ids []string
	for id := range b.eng.Cfg.Launch.Agents {
		if id != store.AgentMe && launch.AgentInstalled(b.eng.Cfg.Launch, id) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if (ids[i] == first) != (ids[j] == first) {
			return ids[i] == first
		}
		return ids[i] < ids[j]
	})
	return ids
}

// taskView shows one task and, while it waits to start, a button per agent.
func (b *Bot) taskView(id int64, header string) (string, *tgKeyboard) {
	back := []tgButton{btn("📋 "+i18n.T("Tasks"), "tasks"), btn("📊 "+i18n.T("Status"), "status")}
	kb := &tgKeyboard{}
	t, err := b.eng.Store.GetTask(id)
	if err != nil {
		kb.Rows = [][]tgButton{back}
		return esc(i18n.T("Task %d not found", id)), kb
	}
	project := ""
	if p, err := b.eng.Store.Get(t.ProjectID); err == nil {
		project = p.Name
	}
	var out []string
	if header != "" {
		out = append(out, header, "")
	}
	out = append(out, fmt.Sprintf("📝 <b>#%d %s</b>", t.ID, esc(t.Title)), fmt.Sprintf("%s · %s", esc(project), esc(t.Status)))
	if d := strings.TrimSpace(t.Description); d != "" {
		out = append(out, esc(short(d, 300)))
	}
	if pending(t) {
		agents := b.launchAgents(t.Agent)
		if len(agents) == 0 {
			out = append(out, esc(i18n.T("No agents installed: launch it from the TUI.")))
		} else {
			out = append(out, "", esc(i18n.T("Launch it with:")))
			tid := strconv.FormatInt(t.ID, 10)
			var row []tgButton
			for _, a := range agents {
				row = append(row, btn("▶️ "+launch.AgentName(a), "launch:"+tid+":"+a))
				if len(row) == 2 {
					kb.Rows = append(kb.Rows, row)
					row = nil
				}
			}
			if row != nil {
				kb.Rows = append(kb.Rows, row)
			}
		}
	}
	kb.Rows = append(kb.Rows, back)
	return strings.Join(out, "\n"), kb
}

// launchTask starts a task with the agent picked on its button.
func (b *Bot) launchTask(ctx context.Context, cq *tgCallback, arg string) {
	idText, agent, _ := strings.Cut(arg, ":")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || agent == "" {
		b.api.answer(ctx, cq.ID, "?")
		return
	}
	t, err := b.eng.Store.GetTask(id)
	if err != nil {
		b.api.answer(ctx, cq.ID, err.Error())
		return
	}
	if !pending(t) {
		b.api.answer(ctx, cq.ID, i18n.T("Task #%d already started", id))
		b.show(ctx, cq.Message.MessageID, "task:"+idText)
		return
	}
	if t.Agent != agent {
		t.Agent = agent
		t.Model = ""
		if err := b.eng.Store.UpdateTask(t); err != nil {
			b.api.answer(ctx, cq.ID, err.Error())
			return
		}
	}
	res, err := b.eng.Launch(t)
	if err != nil {
		b.api.answer(ctx, cq.ID, err.Error())
		return
	}
	b.api.answer(ctx, cq.ID, i18n.T("Launched"))
	msg := "🚀 " + esc(i18n.T("%s is working on #%d %s", launch.AgentName(agent), t.ID, t.Title))
	if res.Run.Branch != "" {
		msg += "\n<code>" + esc(res.Run.Branch) + "</code>"
	}
	if len(b.eng.Cfg.Launch.Hooks[launch.HookReview]) > 0 {
		msg += "\n" + esc(i18n.T("I'll write here when it is ready for review."))
	}
	kb := &tgKeyboard{Rows: [][]tgButton{{btn("📊 "+i18n.T("Status"), "status"), btn("📜 Log", "log:"+strconv.FormatInt(res.Run.ID, 10))}}}
	if err := b.api.edit(ctx, b.chat, cq.Message.MessageID, msg, kb); err != nil {
		b.api.send(ctx, b.chat, msg, kb, 0)
	}
}
