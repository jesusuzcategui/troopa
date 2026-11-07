package notify

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// Bot answers the buttons of Troopa's Telegram messages and a few commands.
// It runs inside `troopa daemon` when [telegram] actions = true, polls
// Telegram (no public server needed) and obeys one chat only.
type Bot struct {
	api     *tgClient
	chat    string
	eng     *engine.Engine
	logger  *log.Logger
	offsetF string

	mu      sync.Mutex
	pending map[int64]int64 // prompt message id -> run id waiting for feedback
}

// NewBot builds the bot. dataDir keeps the last update id between restarts.
func NewBot(token, chat string, eng *engine.Engine, dataDir string, logger *log.Logger) *Bot {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &Bot{
		api: newTG(token), chat: strings.TrimSpace(chat), eng: eng, logger: logger,
		offsetF: filepath.Join(dataDir, "telegram.offset"), pending: map[int64]int64{},
	}
}

// Run polls Telegram until ctx ends.
func (b *Bot) Run(ctx context.Context) {
	offset := b.loadOffset()
	b.logger.Println("telegram bot listening")
	for ctx.Err() == nil {
		ups, err := b.api.updates(ctx, offset, 50)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.logger.Printf("telegram: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}
		for _, u := range ups {
			offset = u.UpdateID + 1
			b.Handle(ctx, u)
		}
		if len(ups) > 0 {
			b.saveOffset(offset)
		}
	}
}

func (b *Bot) loadOffset() int64 {
	raw, err := os.ReadFile(b.offsetF)
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	return n
}

func (b *Bot) saveOffset(n int64) {
	_ = os.WriteFile(b.offsetF, []byte(strconv.FormatInt(n, 10)), 0o600)
}

// allowed is true only for the configured chat.
func (b *Bot) allowed(chat int64) bool {
	return b.chat != "" && strconv.FormatInt(chat, 10) == b.chat
}

// Handle processes one update (exported for tests).
func (b *Bot) Handle(ctx context.Context, u tgUpdate) {
	switch {
	case u.CallbackQuery != nil:
		cq := u.CallbackQuery
		if cq.Message == nil || !b.allowed(cq.Message.Chat.ID) {
			return
		}
		b.callback(ctx, cq)
	case u.Message != nil:
		if !b.allowed(u.Message.Chat.ID) {
			return
		}
		b.message(ctx, u.Message)
	}
}

func (b *Bot) reply(ctx context.Context, text string) {
	if _, err := b.api.send(ctx, b.chat, text, nil, 0, false); err != nil {
		b.logger.Printf("telegram send: %v", err)
	}
}

func (b *Bot) callback(ctx context.Context, cq *tgCallback) {
	action, idText, _ := strings.Cut(cq.Data, ":")
	runID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		b.api.answer(ctx, cq.ID, "?")
		return
	}
	switch action {
	case "approve":
		res, err := b.eng.Approve(runID)
		if err != nil {
			b.api.answer(ctx, cq.ID, err.Error())
			b.reply(ctx, "⚠️ "+esc(err.Error()))
			return
		}
		b.api.answer(ctx, cq.ID, i18n.T("Approved"))
		b.api.clearButtons(ctx, b.chat, cq.Message.MessageID)
		if len(b.eng.Cfg.Launch.Hooks[launch.HookApprove]) == 0 {
			// Without an approve hook nobody else reports it.
			msg := "👍 <b>" + esc(i18n.T("Approved")) + "</b> · " + esc(res.Task.Title)
			if res.Message != "" {
				msg += "\n" + esc(res.Message)
			}
			b.reply(ctx, msg)
		}
	case "changes":
		run, err := b.eng.Store.GetRun(runID)
		if err != nil {
			b.api.answer(ctx, cq.ID, err.Error())
			return
		}
		task, _ := b.eng.Store.GetTask(run.TaskID)
		b.api.answer(ctx, cq.ID, "")
		prompt := "✏️ " + esc(i18n.T("Reply to this message with what %s should change in “%s”.", launch.AgentName(run.Agent), task.Title))
		id, err := b.api.send(ctx, b.chat, prompt, nil, cq.Message.MessageID, true)
		if err != nil {
			b.logger.Printf("telegram send: %v", err)
			return
		}
		b.mu.Lock()
		b.pending[id] = runID
		b.mu.Unlock()
	case "diff":
		b.api.answer(ctx, cq.ID, "")
		b.sendDiff(ctx, runID)
	case "log":
		b.api.answer(ctx, cq.ID, "")
		b.sendLog(ctx, runID)
	default:
		b.api.answer(ctx, cq.ID, "?")
	}
}

func (b *Bot) message(ctx context.Context, m *tgMessage) {
	text := strings.TrimSpace(m.Text)
	if m.ReplyTo != nil {
		b.mu.Lock()
		runID, ok := b.pending[m.ReplyTo.MessageID]
		if ok {
			delete(b.pending, m.ReplyTo.MessageID)
		}
		b.mu.Unlock()
		if ok {
			b.requestChanges(ctx, runID, text)
			return
		}
	}
	if !strings.HasPrefix(text, "/") {
		b.reply(ctx, esc(i18n.T("Send /help to see what I can do.")))
		return
	}
	fields := strings.Fields(text)
	cmd := strings.ToLower(strings.SplitN(fields[0], "@", 2)[0])
	args := fields[1:]
	switch cmd {
	case "/start", "/help", "/ayuda":
		b.reply(ctx, b.help())
	case "/status", "/estado":
		b.reply(ctx, b.status())
	case "/plans", "/planes":
		b.reply(ctx, b.plans())
	case "/plan":
		if id, ok := argID(args); ok {
			b.reply(ctx, b.plan(id))
			return
		}
		b.reply(ctx, esc(i18n.T("Usage: /plan <id>")))
	case "/run", "/iniciar":
		id, ok := argID(args)
		if !ok {
			b.reply(ctx, esc(i18n.T("Usage: /run <plan id>")))
			return
		}
		res, err := b.eng.StartPlan(id)
		if err != nil {
			b.reply(ctx, "⚠️ "+esc(err.Error()))
			return
		}
		msg := "🗺 " + esc(i18n.T("Plan %s started", res.Plan.Title))
		if res.Watch {
			msg += "\n🚀 " + esc(res.Task.Title)
		}
		b.reply(ctx, msg)
	case "/pause", "/pausar":
		id, ok := argID(args)
		if !ok {
			b.reply(ctx, esc(i18n.T("Usage: /pause <plan id>")))
			return
		}
		res, err := b.eng.PausePlan(id)
		if err != nil {
			b.reply(ctx, "⚠️ "+esc(err.Error()))
			return
		}
		b.reply(ctx, "⏸ "+esc(i18n.T("Plan %s paused: the running step finishes, the next one waits.", res.Plan.Title)))
	case "/task", "/tarea":
		b.reply(ctx, b.newTask(args))
	case "/log":
		if id, ok := argID(args); ok {
			b.sendLog(ctx, id)
			return
		}
		b.reply(ctx, esc(i18n.T("Usage: /log <run id>")))
	case "/diff":
		if id, ok := argID(args); ok {
			b.sendDiff(ctx, id)
			return
		}
		b.reply(ctx, esc(i18n.T("Usage: /diff <run id>")))
	default:
		b.reply(ctx, esc(i18n.T("Unknown command. Send /help.")))
	}
}

func argID(args []string) (int64, bool) {
	if len(args) == 0 {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(args[0], "#"), 10, 64)
	return n, err == nil && n > 0
}

func (b *Bot) requestChanges(ctx context.Context, runID int64, feedback string) {
	if feedback == "" {
		b.reply(ctx, esc(i18n.T("The message was empty: nothing was sent.")))
		return
	}
	res, err := b.eng.RequestChanges(runID, feedback, nil)
	if err != nil {
		b.reply(ctx, "⚠️ "+esc(err.Error()))
		return
	}
	b.reply(ctx, "🔁 "+esc(i18n.T("Sent to %s: “%s” is running again.", launch.AgentName(res.Run.Agent), res.Task.Title)))
}

func (b *Bot) help() string {
	lines := []string{
		"<b>Troopa</b>",
		"/status · " + esc(i18n.T("what is running, waiting for review or blocked")),
		"/plans · " + esc(i18n.T("plans and their progress")),
		"/plan &lt;id&gt; · " + esc(i18n.T("the steps of a plan")),
		"/run &lt;id&gt; · " + esc(i18n.T("start or resume a plan")),
		"/pause &lt;id&gt; · " + esc(i18n.T("pause a plan")),
		"/task &lt;project&gt; &lt;title&gt; · " + esc(i18n.T("add a task to a project's backlog")),
		"/log &lt;run&gt; · /diff &lt;run&gt;",
		"",
		esc(i18n.T("The buttons under each alert approve, request changes, or show the diff and the log. Merging stays in the TUI.")),
	}
	return strings.Join(lines, "\n")
}

func (b *Bot) status() string {
	s := b.eng.Store
	var out []string
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
		mark := "👀"
		if state == store.RunBlocked {
			mark = "⛔"
		}
		for i, r := range runs {
			if i == 5 {
				break
			}
			t, _ := s.GetTask(r.TaskID)
			if t.Status == store.StatusDone {
				continue
			}
			out = append(out, fmt.Sprintf("%s #%d %s", mark, r.ID, esc(t.Title)))
		}
	}
	if plans, err := s.ListPlans(0); err == nil {
		for _, p := range plans {
			if p.State == store.PlanRunning || p.State == store.PlanPaused {
				done, total, _ := s.PlanProgress(p.ID)
				out = append(out, fmt.Sprintf("🗺 %s · %d/%d · %s", esc(p.Title), done, total, esc(p.State)))
			}
		}
	}
	if len(out) == 0 {
		return esc(i18n.T("Nothing running and nothing waiting for you."))
	}
	return strings.Join(out, "\n")
}

func (b *Bot) plans() string {
	list, err := b.eng.Store.ListPlans(0)
	if err != nil || len(list) == 0 {
		return esc(i18n.T("No plans yet."))
	}
	var out []string
	for i, p := range list {
		if i == 20 {
			break
		}
		done, total, _ := b.eng.Store.PlanProgress(p.ID)
		out = append(out, fmt.Sprintf("<b>%d</b> · %s · %d/%d · %s", p.ID, esc(p.Title), done, total, esc(p.State)))
	}
	return strings.Join(out, "\n")
}

func (b *Bot) plan(id int64) string {
	p, err := b.eng.Store.GetPlan(id)
	if err != nil {
		return esc(i18n.T("Plan %d not found", id))
	}
	steps, _ := b.eng.Store.PlanSteps(id)
	out := []string{fmt.Sprintf("🗺 <b>%s</b> · %s", esc(p.Title), esc(p.State))}
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
	}
	return strings.Join(out, "\n")
}

// newTask adds a task: /task <project id or slug> <title…>.
func (b *Bot) newTask(args []string) string {
	if len(args) < 2 {
		return esc(i18n.T("Usage: /task <project> <title>"))
	}
	projects, err := b.eng.Store.List(false)
	if err != nil {
		return "⚠️ " + esc(err.Error())
	}
	var proj *store.Project
	for i, p := range projects {
		if strconv.FormatInt(p.ID, 10) == args[0] || strings.EqualFold(p.Slug, args[0]) || strings.EqualFold(p.Name, args[0]) {
			proj = &projects[i]
			break
		}
	}
	if proj == nil {
		return esc(i18n.T("Project %s not found. Use its id or slug.", args[0]))
	}
	agent := b.eng.Cfg.DefaultAgent
	if agent == "" {
		agent = store.AgentMe
	}
	t, err := b.eng.Store.CreateTask(store.Task{
		ProjectID: proj.ID, Title: strings.Join(args[1:], " "), Status: store.StatusBacklog,
		Priority: store.PriorityMedium, Agent: agent, Mode: store.ModeYolo, Commits: store.CommitsNone, Type: store.TypeFeature,
	})
	if err != nil {
		return "⚠️ " + esc(err.Error())
	}
	return "🆕 " + esc(i18n.T("Task #%d added to %s's backlog: %s", t.ID, proj.Name, t.Title))
}

func (b *Bot) sendLog(ctx context.Context, runID int64) {
	run, err := b.eng.Store.GetRun(runID)
	if err != nil {
		b.reply(ctx, esc(i18n.T("Run %d not found", runID)))
		return
	}
	raw, err := os.ReadFile(run.LogPath)
	if err != nil {
		b.reply(ctx, esc(i18n.T("No log yet.")))
		return
	}
	b.reply(ctx, "📜 <b>Log #"+strconv.FormatInt(runID, 10)+"</b>\n<pre>"+esc(lastLines(string(raw), 40, 3200))+"</pre>")
}

func (b *Bot) sendDiff(ctx context.Context, runID int64) {
	run, err := b.eng.Store.GetRun(runID)
	if err != nil {
		b.reply(ctx, esc(i18n.T("Run %d not found", runID)))
		return
	}
	stat, _ := launch.Git(run.Worktree, "diff", "--stat", run.Base)
	patch, err := launch.Git(run.Worktree, "diff", "--no-color", run.Base)
	if err != nil || strings.TrimSpace(patch) == "" {
		b.reply(ctx, esc(i18n.T("No changes to show (the worktree may be gone).")))
		return
	}
	caption := "📄 " + i18n.T("Diff of run %d", runID)
	if len(patch) < 3000 {
		b.reply(ctx, "<b>"+esc(caption)+"</b>\n<pre>"+esc(patch)+"</pre>")
		return
	}
	b.reply(ctx, "<b>"+esc(caption)+"</b>\n<pre>"+esc(short(stat, 3000))+"</pre>")
	if err := b.api.document(ctx, b.chat, fmt.Sprintf("run-%d.diff", runID), []byte(patch), caption); err != nil {
		b.logger.Printf("telegram document: %v", err)
	}
}
