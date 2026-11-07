package notify

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jesusuzcategui/troopa/internal/engine"
	"github.com/jesusuzcategui/troopa/internal/i18n"
)

// New projects from Telegram: a name starts a fresh repository, a git URL is
// cloned; the folder is picked with buttons among the ones the other projects
// live in. Only the owner can create projects.

type newProject struct {
	source string
	dirs   []string
}

func (b *Bot) ownerOnly(ctx context.Context) bool {
	if b.user != nil {
		b.reply(ctx, esc(i18n.T("Only the owner can create projects.")))
		return false
	}
	return true
}

func (b *Bot) askProjectName(ctx context.Context) {
	if !b.ownerOnly(ctx) {
		return
	}
	b.ask(ctx, "📁 "+esc(i18n.T("Reply with the name of the new project, or the URL of a git repository to clone.")), 0, prompt{kind: "project"})
}

func (b *Bot) askProjectFolder(ctx context.Context, source string) {
	if !b.ownerOnly(ctx) {
		return
	}
	if source == "" {
		b.reply(ctx, esc(i18n.T("The message was empty: nothing was sent.")))
		return
	}
	dirs := b.eng.ProjectParents(6)
	b.mu.Lock()
	if b.newProj == nil {
		b.newProj = map[string]newProject{}
	}
	b.newProj[b.chat] = newProject{source: source, dirs: dirs}
	b.mu.Unlock()
	kb := &tgKeyboard{}
	for i, d := range dirs {
		kb.Rows = append(kb.Rows, []tgButton{btn("📂 "+short(d, 60), "projdir:"+strconv.Itoa(i))})
	}
	what := i18n.T("New project %s", source)
	if engine.IsGitURL(source) {
		what = i18n.T("Clone %s", engine.RepoName(source))
	}
	_, _ = b.api.send(ctx, b.chat, "📁 <b>"+esc(what)+"</b>\n"+esc(i18n.T("In which folder? It goes in a subfolder named after the project.")), kb, 0)
}

func (b *Bot) createProject(ctx context.Context, cq *tgCallback, i int) {
	if b.user != nil {
		b.api.answer(ctx, cq.ID, i18n.T("Only the owner can create projects."))
		return
	}
	b.mu.Lock()
	np, ok := b.newProj[b.chat]
	delete(b.newProj, b.chat)
	b.mu.Unlock()
	if !ok || i < 0 || i >= len(np.dirs) {
		b.api.answer(ctx, cq.ID, i18n.T("Start again with /project."))
		return
	}
	b.api.answer(ctx, cq.ID, "")
	if engine.IsGitURL(np.source) {
		_ = b.api.edit(ctx, b.chat, cq.Message.MessageID, "⏳ "+esc(i18n.T("Cloning %s…", np.source)), nil)
	}
	p, err := b.eng.CreateProject(np.source, np.dirs[i])
	if err != nil {
		_ = b.api.edit(ctx, b.chat, cq.Message.MessageID, "⚠️ "+esc(err.Error()), nil)
		return
	}
	id := strconv.FormatInt(p.ID, 10)
	kb := &tgKeyboard{Rows: [][]tgButton{{btn("➕ "+i18n.T("New task"), "newtask:"+id), btn("📋 "+i18n.T("Tasks"), "tasks")}}}
	text := "✅ " + esc(i18n.T("Project %s created", p.Name)) + fmt.Sprintf("\n<code>%s</code>", esc(p.RepoPath))
	if err := b.api.edit(ctx, b.chat, cq.Message.MessageID, text, kb); err != nil {
		_, _ = b.api.send(ctx, b.chat, text, kb, 0)
	}
}
