package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
)

// A conversation sends each message to an agent CLI, one turn at a time, and
// relays the answer. The agent runs in the project's folder (a project called
// "Conversar", or any other) with the whole transcript in a file, so any
// agent works the same way and nothing ends up in a worktree or a run.

// ChatTimeout caps one turn.
var ChatTimeout = 15 * time.Minute

type chatState struct {
	Agent     string `json:"agent"`
	ProjectID int64  `json:"project_id"`
	Project   string `json:"project"`
	Dir       string `json:"dir"`
	Turns     int    `json:"turns"`
	busy      bool
}

func (b *Bot) chatFile(chat string) string {
	return filepath.Join(b.dataDir, "telegram.chat."+chat+".json")
}

// loadChats reopens the conversations that were open when the daemon stopped.
func (b *Bot) loadChats() {
	files, _ := filepath.Glob(filepath.Join(b.dataDir, "telegram.chat.*.json"))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var c chatState
		if json.Unmarshal(raw, &c) == nil && c.Agent != "" {
			chat := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "telegram.chat."), ".json")
			b.convs[chat] = &c
		}
	}
}

// saveChat persists the conversation of chat; call with b.mu held.
func (b *Bot) saveChat(chat string) {
	c := b.convs[chat]
	if c == nil {
		_ = os.Remove(b.chatFile(chat))
		return
	}
	raw, _ := json.Marshal(c)
	_ = os.WriteFile(b.chatFile(chat), raw, 0o600)
}

func closeButton() *tgKeyboard {
	return &tgKeyboard{Rows: [][]tgButton{{btn("🔚 "+i18n.T("End conversation"), "chatend")}}}
}

// chatAgentsView picks the agent to talk to.
func (b *Bot) chatAgentsView() (string, *tgKeyboard) {
	kb := &tgKeyboard{}
	first := b.eng.Cfg.DefaultAgent
	if b.user != nil && b.user.Agent != "" {
		first = b.user.Agent
	}
	agents := b.launchAgents(first)
	if len(agents) == 0 {
		return esc(i18n.T("No agents installed: launch it from the TUI.")), kb
	}
	var row []tgButton
	for _, a := range agents {
		row = append(row, btn(launch.AgentName(a), "chat:"+a))
		if len(row) == 2 {
			kb.Rows = append(kb.Rows, row)
			row = nil
		}
	}
	if row != nil {
		kb.Rows = append(kb.Rows, row)
	}
	return "💬 " + esc(i18n.T("Who do you want to talk to?")), kb
}

// chatProjectsView picks the folder the agent works in, "Conversar" first.
func (b *Bot) chatProjectsView(agent string) (string, *tgKeyboard) {
	kb := &tgKeyboard{}
	projects := b.visibleProjects()
	if len(projects) == 0 {
		return esc(i18n.T("No projects yet.")), kb
	}
	for i, p := range projects {
		if isChatProject(p.Name) || isChatProject(p.Slug) {
			projects[0], projects[i] = projects[i], projects[0]
			break
		}
	}
	for i, p := range projects {
		if i == 20 {
			break
		}
		kb.Rows = append(kb.Rows, []tgButton{btn(short(p.Name, 50), "chatp:"+agent+":"+strconv.FormatInt(p.ID, 10))})
	}
	return "💬 " + esc(i18n.T("In which project? %s reads its files if the question needs them.", launch.AgentName(agent))), kb
}

func isChatProject(s string) bool {
	s = strings.ToLower(s)
	return strings.HasPrefix(s, "convers") || s == "chat"
}

// startChat opens a conversation; an open one is closed first.
func (b *Bot) startChat(ctx context.Context, msgID int64, agent string, projectID int64) {
	p, err := b.eng.Store.Get(projectID)
	if err != nil {
		b.reply(ctx, "⚠️ "+esc(err.Error()))
		return
	}
	dir := filepath.Join(b.dataDir, "chats", time.Now().Format("20060102-150405")+"-"+agent)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.reply(ctx, "⚠️ "+esc(err.Error()))
		return
	}
	header := fmt.Sprintf("# %s · %s\n\n", launch.AgentName(agent), p.Name)
	_ = os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(header), 0o644)
	b.mu.Lock()
	b.convs[b.chat] = &chatState{Agent: agent, ProjectID: projectID, Project: p.Name, Dir: dir}
	b.saveChat(b.chat)
	b.mu.Unlock()
	text := "💬 " + esc(i18n.T("Talking with %s in %s. Write your message; every answer comes back here.", launch.AgentName(agent), p.Name))
	if err := b.api.edit(ctx, b.chat, msgID, text, closeButton()); err != nil {
		b.api.send(ctx, b.chat, text, closeButton(), 0)
	}
}

func (b *Bot) endChat(ctx context.Context) {
	b.mu.Lock()
	c := b.convs[b.chat]
	delete(b.convs, b.chat)
	b.saveChat(b.chat)
	b.mu.Unlock()
	if c == nil {
		b.reply(ctx, esc(i18n.T("There is no open conversation.")))
		return
	}
	b.reply(ctx, "🔚 "+esc(i18n.T("Conversation with %s closed after %d messages. The transcript stays in %s.", launch.AgentName(c.Agent), c.Turns, c.Dir)))
}

// chatting reports whether a conversation is open.
func (b *Bot) chatting() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.convs[b.chat] != nil
}

// chatSend runs one turn in the background and relays the answer. image is
// a picture the person sent with the message, already saved, or "".
func (b *Bot) chatSend(ctx context.Context, text, image string) {
	chat := b.chat
	b.mu.Lock()
	c := b.convs[chat]
	if c == nil {
		b.mu.Unlock()
		return
	}
	if c.busy {
		b.mu.Unlock()
		b.reply(ctx, "⏳ "+esc(i18n.T("%s is still answering your last message.", launch.AgentName(c.Agent))))
		return
	}
	c.busy = true
	c.Turns++
	turn := c.Turns
	agent, projectID, dir := c.Agent, c.ProjectID, c.Dir
	b.saveChat(chat)
	b.mu.Unlock()

	transcript := filepath.Join(dir, "transcript.md")
	entry := text
	if image != "" {
		entry = strings.TrimSpace(entry + "\n\n[" + i18n.T("image") + ": " + image + "]")
	}
	appendFile(transcript, "## "+i18n.T("Person")+"\n\n"+entry+"\n\n")
	replyPath := filepath.Join(dir, fmt.Sprintf("reply-%d.md", turn))
	outDir := filepath.Join(dir, fmt.Sprintf("out-%d", turn))
	_ = os.MkdirAll(outDir, 0o755)
	prompt := fmt.Sprintf("You are chatting with a person through Telegram; this is a conversation, not a coding task. "+
		"Read %s: it holds the whole conversation, newest message last. Answer that last message. "+
		"You are in the folder of the project; read its files only if the question needs them, and do not change anything unless the person explicitly asks. "+
		"If the person asks for an image, or to change one they sent, make it with your image tools and save every resulting image in %s; they are sent to the person. "+
		"Write ONLY your answer, in the person's language and in Markdown, to %s. Keep it short enough for a chat (under 3000 characters).",
		transcript, outDir, replyPath)
	if image != "" {
		prompt += fmt.Sprintf(" The person attached an image to the last message: %s.", image)
	}
	var images []string
	if image != "" {
		images = append(images, image)
	}
	cmd, err := b.eng.ChatCommand(projectID, agent, prompt, images...)
	if err != nil {
		b.finishTurn(ctx, chat, "⚠️ "+esc(err.Error()), nil)
		return
	}
	b.typing(ctx, chat)
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		tctx, cancel := context.WithTimeout(context.Background(), ChatTimeout)
		defer cancel()
		logF, _ := os.Create(filepath.Join(dir, fmt.Sprintf("turn-%d.log", turn)))
		if logF != nil {
			cmd.Stdout, cmd.Stderr = logF, logF
			defer logF.Close()
		}
		done := make(chan error, 1)
		if err := cmd.Start(); err != nil {
			b.finishTurn(ctx, chat, "⚠️ "+esc(err.Error()), nil)
			return
		}
		go func() { done <- cmd.Wait() }()
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		var runErr error
	wait:
		for {
			select {
			case runErr = <-done:
				break wait
			case <-tick.C:
				b.typing(ctx, chat)
			case <-tctx.Done():
				_ = cmd.Process.Kill()
				runErr = <-done
				if runErr == nil {
					runErr = tctx.Err()
				}
				break wait
			}
		}
		pics := imagesIn(outDir, 5)
		answer, err := os.ReadFile(replyPath)
		if err != nil || strings.TrimSpace(string(answer)) == "" {
			if len(pics) > 0 {
				appendFile(transcript, "## "+launch.AgentName(agent)+"\n\n["+i18n.T("image")+"]\n\n")
				b.finishTurn(ctx, chat, "", pics)
				return
			}
			msg := "⚠️ " + esc(i18n.T("%s did not write an answer.", launch.AgentName(agent)))
			if runErr != nil {
				msg += " " + esc(runErr.Error())
			}
			b.finishTurn(ctx, chat, msg, nil)
			return
		}
		appendFile(transcript, "## "+launch.AgentName(agent)+"\n\n"+strings.TrimSpace(string(answer))+"\n\n")
		b.finishTurn(ctx, chat, limit(markdownToHTML(string(answer))), pics)
	}()
}

func (b *Bot) typing(ctx context.Context, chat string) {
	_, _ = b.api.call(ctx, "sendChatAction", map[string][]string{"chat_id": {chat}, "action": {"typing"}})
}

// finishTurn sends the images and the answer of a turn to chat.
func (b *Bot) finishTurn(ctx context.Context, chat, text string, pics []string) {
	b.mu.Lock()
	if c := b.convs[chat]; c != nil {
		c.busy = false
	}
	b.mu.Unlock()
	for _, p := range pics {
		if err := b.api.photo(ctx, chat, p, "", nil); err != nil {
			b.logger.Printf("telegram photo: %v", err)
		}
	}
	if text == "" {
		text = "🖼"
	}
	if _, err := b.api.send(ctx, chat, text, closeButton(), 0); err != nil {
		b.logger.Printf("telegram send: %v", err)
	}
}

// imagesIn lists up to max image files in dir (not recursive), sorted.
func imagesIn(dir string, max int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && isImage(e.Name()) {
			out = append(out, filepath.Join(dir, e.Name()))
			if len(out) == max {
				break
			}
		}
	}
	return out
}

func appendFile(path, text string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(text)
}
