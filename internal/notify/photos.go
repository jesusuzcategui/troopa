package notify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// A photo sent to the bot becomes the attachment of a new task (its caption is
// the request), which launches right away or waits in the queue. Inside a
// conversation it goes to the agent with the message instead.

type pendingPhoto struct {
	path, caption string
}

// photoIn handles a message that carries an image.
func (b *Bot) photoIn(ctx context.Context, m *tgMessage, fileID, name string) {
	caption := strings.TrimSpace(m.Caption)
	if b.chatting() {
		b.mu.Lock()
		dir := b.convs[b.chat].Dir
		b.mu.Unlock()
		dest := filepath.Join(dir, "in-"+strconv.FormatInt(m.MessageID, 10)+"-"+name)
		if err := b.api.download(ctx, fileID, dest); err != nil {
			b.reply(ctx, "⚠️ "+esc(err.Error()))
			return
		}
		b.chatSend(ctx, caption, dest)
		return
	}
	dest := filepath.Join(b.dataDir, "inbox", b.chat+"-"+strconv.FormatInt(m.MessageID, 10)+"-"+name)
	if err := b.api.download(ctx, fileID, dest); err != nil {
		b.reply(ctx, "⚠️ "+esc(err.Error()))
		return
	}
	projects := b.visibleProjects()
	switch len(projects) {
	case 0:
		b.reply(ctx, esc(i18n.T("No projects yet.")))
	case 1:
		b.photoTask(ctx, projects[0], dest, caption)
	default:
		b.mu.Lock()
		if b.photos == nil {
			b.photos = map[string]pendingPhoto{}
		}
		b.photos[b.chat] = pendingPhoto{path: dest, caption: caption}
		b.mu.Unlock()
		kb := &tgKeyboard{}
		for i, p := range projects {
			if i == 20 {
				break
			}
			kb.Rows = append(kb.Rows, []tgButton{btn(short(p.Name, 50), "photop:"+strconv.FormatInt(p.ID, 10))})
		}
		if _, err := b.api.send(ctx, b.chat, "🖼 "+esc(i18n.T("Which project is this photo for?")), kb, 0); err != nil {
			b.logger.Printf("telegram send: %v", err)
		}
	}
}

// photoProject finishes a photo whose project was picked with a button.
func (b *Bot) photoProject(ctx context.Context, cq *tgCallback, projectID int64) {
	b.mu.Lock()
	ph, ok := b.photos[b.chat]
	delete(b.photos, b.chat)
	b.mu.Unlock()
	if !ok {
		b.api.answer(ctx, cq.ID, i18n.T("Send the photo again."))
		return
	}
	p, err := b.eng.Store.Get(projectID)
	if err != nil {
		b.api.answer(ctx, cq.ID, err.Error())
		return
	}
	b.api.answer(ctx, cq.ID, "")
	b.api.clearButtons(ctx, b.chat, cq.Message.MessageID)
	b.photoTask(ctx, p, ph.path, ph.caption)
}

// photoTask creates the task, attaches the photo and launches or queues it.
func (b *Bot) photoTask(ctx context.Context, p store.Project, path, caption string) {
	title := caption
	if i := strings.IndexByte(title, '\n'); i >= 0 {
		title = title[:i]
	}
	title = short(strings.TrimSpace(title), 80)
	if title == "" {
		title = i18n.T("Photo from Telegram %s", time.Now().Format("2006-01-02 15:04"))
	}
	id, msg := b.addTask(p, title)
	if id == 0 {
		b.reply(ctx, msg)
		return
	}
	t, err := b.eng.Store.GetTask(id)
	if err != nil {
		b.reply(ctx, "⚠️ "+esc(err.Error()))
		return
	}
	t.Description = strings.TrimSpace(caption + "\n\n" + i18n.T("The person sent the attached photo from Telegram. If the request is about the image, save the images you create or edit in the worktree (for example out/); they are sent back to the person."))
	_ = b.eng.Store.UpdateTask(t)
	dir, err := config.AttachmentsDir(id)
	if err == nil {
		_, err = b.eng.Store.AttachFile(id, path, dir, b.eng.Cfg.MaxAttachmentBytes())
	}
	if err != nil {
		b.reply(ctx, "⚠️ "+esc(err.Error()))
		return
	}
	_ = os.Remove(path)
	if t.Agent == "" || t.Agent == store.AgentMe || !launch.AgentInstalled(b.eng.Cfg.Launch, t.Agent) {
		b.added(ctx, id, msg)
		return
	}
	b.startOrQueue(ctx, b.chat, t, msg)
}

// startOrQueue launches the task now or puts it in the queue.
func (b *Bot) startOrQueue(ctx context.Context, chat string, t store.Task, header string) {
	if _, err := b.eng.Store.ActiveRun(); err == nil {
		pos, err := b.eng.Store.Enqueue(t.ID, chat)
		if err != nil {
			b.reply(ctx, "⚠️ "+esc(err.Error()))
			return
		}
		text := "⏳ " + esc(i18n.T("#%d is in the queue (position %d): it starts as soon as the agent working now finishes.", t.ID, pos))
		if header != "" {
			text = header + "\n" + text
		}
		_, _ = b.api.send(ctx, chat, text, nil, 0)
		return
	}
	res, err := b.eng.Launch(t)
	if err != nil {
		b.reply(ctx, "⚠️ "+esc(err.Error()))
		return
	}
	text := "🚀 " + esc(i18n.T("%s is working on #%d %s", launch.AgentName(t.Agent), t.ID, t.Title))
	if header != "" {
		text = header + "\n" + text
	}
	kb := &tgKeyboard{Rows: [][]tgButton{{btn("📜 Log", "log:"+strconv.FormatInt(res.Run.ID, 10)), btn("📝 "+i18n.T("Task"), "task:"+strconv.FormatInt(t.ID, 10))}}}
	_, _ = b.api.send(ctx, chat, text, kb, 0)
}

// drainQueue launches the oldest queued task when no agent is running.
func (b *Bot) drainQueue(ctx context.Context) {
	if _, err := b.eng.Store.ActiveRun(); err == nil {
		return
	}
	items, err := b.eng.Store.Queue()
	if err != nil {
		return
	}
	for _, it := range items {
		t, err := b.eng.Store.GetTask(it.TaskID)
		if err != nil || !pending(t) {
			_ = b.eng.Store.Dequeue(it.ID)
			continue
		}
		res, err := b.eng.Launch(t)
		if err != nil {
			if strings.Contains(err.Error(), i18n.T("An agent is already running")) {
				return
			}
			_ = b.eng.Store.Dequeue(it.ID)
			if it.Chat != "" {
				_, _ = b.api.send(ctx, it.Chat, "⚠️ "+esc(fmt.Sprintf("#%d: %v", t.ID, err)), nil, 0)
			}
			continue
		}
		_ = b.eng.Store.Dequeue(it.ID)
		if it.Chat != "" {
			kb := &tgKeyboard{Rows: [][]tgButton{{btn("📜 Log", "log:"+strconv.FormatInt(res.Run.ID, 10))}}}
			_, _ = b.api.send(ctx, it.Chat, "🚀 "+esc(i18n.T("Your turn: %s is working on #%d %s", launch.AgentName(t.Agent), t.ID, t.Title)), kb, 0)
		}
		return
	}
}

// sendImages sends the pictures a run created or changed.
func (b *Bot) sendImages(ctx context.Context, runID int64) {
	run, err := b.eng.Store.GetRun(runID)
	if err != nil {
		b.reply(ctx, "⚠️ "+esc(err.Error()))
		return
	}
	imgs := launch.ChangedImages(run.Worktree, run.Base, 10)
	if len(imgs) == 0 {
		b.reply(ctx, esc(i18n.T("This run has no images.")))
		return
	}
	for _, img := range imgs {
		if err := b.api.photo(ctx, b.chat, img, filepath.Base(img), nil); err != nil {
			b.logger.Printf("telegram photo: %v", err)
		}
	}
}
