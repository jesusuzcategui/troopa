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
	dataDir string

	mu      sync.Mutex
	pending map[int64]prompt // prompt message id -> what the reply is for
	conv    *chatState       // the open conversation with an agent, if any
	wg      sync.WaitGroup   // conversation turns in flight
}

// prompt is a question the bot asked with a forced reply.
type prompt struct {
	kind string // "changes" (id = run) or "task" (id = project)
	id   int64
}

// NewBot builds the bot. dataDir keeps the last update id between restarts.
func NewBot(token, chat string, eng *engine.Engine, dataDir string, logger *log.Logger) *Bot {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	b := &Bot{
		api: newTG(token), chat: strings.TrimSpace(chat), eng: eng, logger: logger,
		offsetF: filepath.Join(dataDir, "telegram.offset"), dataDir: dataDir, pending: map[int64]prompt{},
	}
	b.loadChat()
	return b
}

// Run polls Telegram until ctx ends.
func (b *Bot) Run(ctx context.Context) {
	offset := b.loadOffset()
	if err := b.api.setCommands(ctx, b.commands()); err != nil {
		b.logger.Printf("telegram setMyCommands: %v", err)
	}
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
	if _, err := b.api.send(ctx, b.chat, text, nil, 0); err != nil {
		b.logger.Printf("telegram send: %v", err)
	}
}

func (b *Bot) callback(ctx context.Context, cq *tgCallback) {
	data, back, _ := strings.Cut(cq.Data, "|")
	action, idText, _ := strings.Cut(data, ":")
	msgID := cq.Message.MessageID
	switch action {
	case "status", "plans", "plan", "newtask", "task":
		if action == "newtask" && idText != "" {
			b.askTaskTitle(ctx, cq, idText)
			return
		}
		b.api.answer(ctx, cq.ID, "")
		b.show(ctx, msgID, data)
		return
	}
	switch action {
	case "chat":
		b.api.answer(ctx, cq.ID, "")
		if idText == "" {
			b.show(ctx, msgID, "chat")
		} else {
			text, kb := b.chatProjectsView(idText)
			if err := b.api.edit(ctx, b.chat, msgID, text, kb); err != nil {
				b.api.send(ctx, b.chat, text, kb, 0)
			}
		}
		return
	case "chatp":
		b.api.answer(ctx, cq.ID, "")
		agent, pid, _ := strings.Cut(idText, ":")
		id, _ := strconv.ParseInt(pid, 10, 64)
		b.startChat(ctx, msgID, agent, id)
		return
	case "chatend":
		b.api.answer(ctx, cq.ID, "")
		b.api.clearButtons(ctx, b.chat, msgID)
		b.endChat(ctx)
		return
	case "launch":
		b.launchTask(ctx, cq, idText)
		return
	case "tasks":
		b.api.answer(ctx, cq.ID, "")
		b.show(ctx, msgID, data)
		return
	}
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		b.api.answer(ctx, cq.ID, "?")
		return
	}
	switch action {
	case "pstart", "ppause":
		var res engine.Result
		var note string
		if action == "pstart" {
			res, err = b.eng.StartPlan(id)
			note = i18n.T("Plan %s started", res.Plan.Title)
		} else {
			res, err = b.eng.PausePlan(id)
			note = i18n.T("Plan %s paused: the running step finishes, the next one waits.", res.Plan.Title)
		}
		if err != nil {
			b.api.answer(ctx, cq.ID, err.Error())
			return
		}
		b.api.answer(ctx, cq.ID, note)
		b.show(ctx, msgID, "plan:"+idText)
	case "approve":
		res, err := b.eng.Approve(id)
		if err != nil {
			b.api.answer(ctx, cq.ID, err.Error())
			b.reply(ctx, "⚠️ "+esc(err.Error()))
			return
		}
		b.api.answer(ctx, cq.ID, i18n.T("Approved"))
		if back != "" {
			b.show(ctx, msgID, back)
		} else {
			b.api.clearButtons(ctx, b.chat, msgID)
		}
		if len(b.eng.Cfg.Launch.Hooks[launch.HookApprove]) == 0 {
			// Without an approve hook nobody else reports it.
			msg := "👍 <b>" + esc(i18n.T("Approved")) + "</b> · " + esc(res.Task.Title)
			if res.Message != "" {
				msg += "\n" + esc(res.Message)
			}
			b.reply(ctx, msg)
		}
	case "changes":
		run, err := b.eng.Store.GetRun(id)
		if err != nil {
			b.api.answer(ctx, cq.ID, err.Error())
			return
		}
		task, _ := b.eng.Store.GetTask(run.TaskID)
		b.api.answer(ctx, cq.ID, "")
		text := "✏️ " + esc(i18n.T("Reply to this message with what %s should change in “%s”.", launch.AgentName(run.Agent), task.Title))
		b.ask(ctx, text, msgID, prompt{kind: "changes", id: id})
	case "comment":
		b.api.answer(ctx, cq.ID, "")
		b.ask(ctx, "💬 "+esc(i18n.T("Reply to this message with your comment. It waits with the other comments until you request changes.")), msgID, prompt{kind: "comment", id: id})
	case "sendc":
		b.api.answer(ctx, cq.ID, "")
		b.requestChanges(ctx, id, "")
	case "merge", "pr", "pmerge", "ppr":
		b.api.answer(ctx, cq.ID, "")
		b.confirmMerge(ctx, msgID, action, id, back)
	case "domerge", "dopr", "dopmerge", "doppr":
		b.doMerge(ctx, cq, action, id, back)
	case "pull":
		b.pull(ctx, cq, id)
	case "diff":
		b.api.answer(ctx, cq.ID, "")
		b.sendDiff(ctx, id)
	case "log":
		b.api.answer(ctx, cq.ID, "")
		b.sendLog(ctx, id)
	default:
		b.api.answer(ctx, cq.ID, "?")
	}
}

// ask sends a question that opens a reply and remembers what it was for.
func (b *Bot) ask(ctx context.Context, text string, replyTo int64, p prompt) {
	id, err := b.api.send(ctx, b.chat, text, forceReply, replyTo)
	if err != nil {
		b.logger.Printf("telegram send: %v", err)
		return
	}
	b.mu.Lock()
	b.pending[id] = p
	b.mu.Unlock()
}

func (b *Bot) message(ctx context.Context, m *tgMessage) {
	text := strings.TrimSpace(m.Text)
	if m.ReplyTo != nil {
		b.mu.Lock()
		p, ok := b.pending[m.ReplyTo.MessageID]
		if ok {
			delete(b.pending, m.ReplyTo.MessageID)
		}
		b.mu.Unlock()
		if ok {
			switch p.kind {
			case "task":
				id, msg := b.createTask(p.id, text)
				b.added(ctx, id, msg)
			case "comment":
				b.addComment(ctx, p.id, text)
			default:
				b.requestChanges(ctx, p.id, text)
			}
			return
		}
	}
	if view, ok := keyboardView(text); ok {
		if view == "help" {
			b.sendHelp(ctx)
		} else {
			b.show(ctx, 0, view)
		}
		return
	}
	if !strings.HasPrefix(text, "/") && b.chatting() {
		if text != "" {
			b.chatSend(ctx, text)
		}
		return
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
		b.sendHelp(ctx)
	case "/status", "/estado":
		b.show(ctx, 0, "status")
	case "/plans", "/planes":
		b.show(ctx, 0, "plans")
	case "/plan":
		if id, ok := argID(args); ok {
			b.show(ctx, 0, "plan:"+strconv.FormatInt(id, 10))
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
	case "/chat", "/conversar":
		b.show(ctx, 0, "chat")
	case "/end", "/cerrar":
		b.endChat(ctx)
	case "/tasks", "/tareas":
		b.show(ctx, 0, "tasks")
	case "/launch", "/lanzar":
		if id, ok := argID(args); ok {
			b.show(ctx, 0, "task:"+strconv.FormatInt(id, 10))
			return
		}
		b.show(ctx, 0, "tasks")
	case "/task", "/tarea":
		if len(args) == 0 {
			b.show(ctx, 0, "newtask")
			return
		}
		id, msg := b.newTask(args)
		b.added(ctx, id, msg)
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
	comments, _ := b.eng.Store.ListPendingComments(runID)
	if strings.TrimSpace(feedback) == "" && len(comments) == 0 {
		b.reply(ctx, esc(i18n.T("The message was empty: nothing was sent.")))
		return
	}
	res, err := b.eng.RequestChanges(runID, feedback, comments)
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
		"/tasks · " + esc(i18n.T("tasks by status: start, review, comment or merge them")),
		"/chat · " + esc(i18n.T("talk with an agent; /end closes the conversation")),
		"/log &lt;run&gt; · /diff &lt;run&gt;",
		"",
		esc(i18n.T("The buttons under each alert approve, request changes, comment, merge, or show the diff and the log.")),
		esc(i18n.T("Use the keyboard below or the menu: plans, steps and reviews open as buttons.")),
	}
	return strings.Join(lines, "\n")
}

// newTask adds a task: /task <project id or slug> <title…>.
func (b *Bot) newTask(args []string) (int64, string) {
	if len(args) < 2 {
		return 0, esc(i18n.T("Usage: /task <project> <title>"))
	}
	projects, err := b.eng.Store.List(false)
	if err != nil {
		return 0, "⚠️ " + esc(err.Error())
	}
	var proj *store.Project
	for i, p := range projects {
		if strconv.FormatInt(p.ID, 10) == args[0] || strings.EqualFold(p.Slug, args[0]) || strings.EqualFold(p.Name, args[0]) {
			proj = &projects[i]
			break
		}
	}
	if proj == nil {
		return 0, esc(i18n.T("Project %s not found. Use its id or slug.", args[0]))
	}
	return b.addTask(*proj, strings.Join(args[1:], " "))
}

// createTask adds a task to a project picked with the buttons.
func (b *Bot) createTask(projectID int64, title string) (int64, string) {
	if strings.TrimSpace(title) == "" {
		return 0, esc(i18n.T("The message was empty: nothing was sent."))
	}
	p, err := b.eng.Store.Get(projectID)
	if err != nil {
		return 0, "⚠️ " + esc(err.Error())
	}
	return b.addTask(p, strings.TrimSpace(title))
}

func (b *Bot) addTask(proj store.Project, title string) (int64, string) {
	agent := b.eng.Cfg.DefaultAgent
	if agent == "" {
		agent = store.AgentMe
	}
	t, err := b.eng.Store.CreateTask(store.Task{
		ProjectID: proj.ID, Title: title, Status: store.StatusBacklog,
		Priority: store.PriorityMedium, Agent: agent, Mode: store.ModeYolo, Commits: store.CommitsNone, Type: store.TypeFeature,
	})
	if err != nil {
		return 0, "⚠️ " + esc(err.Error())
	}
	return t.ID, "🆕 " + esc(i18n.T("Task #%d added to %s's backlog: %s", t.ID, proj.Name, t.Title))
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
