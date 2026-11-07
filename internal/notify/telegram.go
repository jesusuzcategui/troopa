// Package notify sends Troopa events to chat apps. It runs as a hook:
// [hooks] review = "troopa notify telegram --token … --chat …" gets the event
// as JSON on stdin, reads the run from the database and sends a full message:
// the agent's summary, checks, time, tokens, cost and the plan step.
package notify

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

// TelegramAPI is the Bot API base URL (tests point it at a fake server).
var TelegramAPI = "https://api.telegram.org"

const maxMessage = 4000 // Telegram allows 4096 characters

// Telegram runs `troopa notify telegram`. open opens the store (nil when
// there is none, e.g. in --test without a database).
func Telegram(args []string, stdin io.Reader, stdout, stderr io.Writer, open func() (*store.Store, error)) int {
	f := flag.NewFlagSet("notify telegram", flag.ContinueOnError)
	f.SetOutput(stderr)
	token := f.String("token", os.Getenv("TROOPA_TELEGRAM_TOKEN"), "bot token (or TROOPA_TELEGRAM_TOKEN)")
	chat := f.String("chat", os.Getenv("TROOPA_TELEGRAM_CHAT"), "chat id (or TROOPA_TELEGRAM_CHAT)")
	dry := f.Bool("dry-run", false, "print the message instead of sending it")
	test := f.Bool("test", false, "send a sample message built from the latest run")
	if err := f.Parse(args); err != nil {
		return 2
	}
	var ev launch.HookEvent
	if *test {
		ev = launch.HookEvent{Event: launch.HookReview}
	} else {
		raw, err := io.ReadAll(stdin)
		if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
			fmt.Fprintln(stderr, "troopa notify: no event on stdin (run it from a hook, or use --test)")
			return 2
		}
		if err := json.Unmarshal(raw, &ev); err != nil {
			fmt.Fprintf(stderr, "troopa notify: bad event: %v\n", err)
			return 2
		}
	}
	var s *store.Store
	if open != nil {
		if st, err := open(); err == nil {
			s = st
			defer s.Close()
		}
	}
	if *test && s != nil {
		if runs, err := s.ListRuns(""); err == nil && len(runs) > 0 {
			r := runs[0]
			ev.RunID, ev.TaskID = r.ID, r.TaskID
		}
	}
	msg := BuildMessage(s, ev)
	if *dry {
		fmt.Fprintln(stdout, msg)
		return 0
	}
	if *token == "" || *chat == "" {
		fmt.Fprintln(stderr, "troopa notify: --token and --chat are required (or TROOPA_TELEGRAM_TOKEN and TROOPA_TELEGRAM_CHAT)")
		return 2
	}
	if err := sendTelegram(*token, *chat, msg); err != nil {
		fmt.Fprintf(stderr, "troopa notify: %v\n", err)
		return 1
	}
	return 0
}

func sendTelegram(token, chat, text string) error {
	post := func(mode string) (int, string, error) {
		form := url.Values{"chat_id": {chat}, "text": {text}, "disable_web_page_preview": {"true"}}
		if mode != "" {
			form.Set("parse_mode", mode)
		} else {
			form.Set("text", stripTags(text))
		}
		c := &http.Client{Timeout: 20 * time.Second}
		resp, err := c.PostForm(TelegramAPI+"/bot"+token+"/sendMessage", form)
		if err != nil {
			return 0, "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return resp.StatusCode, string(body), nil
	}
	code, body, err := post("HTML")
	if err != nil {
		return errors.New(strings.ReplaceAll(err.Error(), token, "<token>"))
	}
	if code == http.StatusBadRequest && strings.Contains(body, "parse") {
		// Something in the summary broke the HTML: send it as plain text.
		code, body, err = post("")
		if err != nil {
			return errors.New(strings.ReplaceAll(err.Error(), token, "<token>"))
		}
	}
	if code != http.StatusOK {
		return fmt.Errorf("telegram answered %d: %s", code, body)
	}
	return nil
}

// BuildMessage writes the Telegram message (HTML) for an event.
func BuildMessage(s *store.Store, ev launch.HookEvent) string {
	var task store.Task
	var run store.Run
	var plan store.Plan
	var project store.Project
	hasRun, hasPlan := false, false
	if s != nil {
		if ev.RunID != 0 {
			if r, err := s.GetRun(ev.RunID); err == nil {
				run, hasRun = r, true
				if ev.TaskID == 0 {
					ev.TaskID = r.TaskID
				}
			}
		}
		if ev.TaskID != 0 {
			if t, err := s.GetTask(ev.TaskID); err == nil {
				task = t
				if ev.PlanID == 0 {
					ev.PlanID = t.PlanID
				}
				if p, err := s.Get(t.ProjectID); err == nil {
					project = p
				}
			}
		}
		if ev.PlanID != 0 {
			if p, err := s.GetPlan(ev.PlanID); err == nil {
				plan, hasPlan = p, true
				if project.ID == 0 {
					if pr, err := s.Get(p.ProjectID); err == nil {
						project = pr
					}
				}
			}
		}
	}
	title := ev.Task
	if task.Title != "" {
		title = task.Title
	}
	projectName := ev.Project
	if project.Name != "" {
		projectName = project.Name
	}
	var b strings.Builder
	b.WriteString("<b>" + esc(eventHeadline(ev.Event)) + "</b>\n\n")
	if projectName != "" {
		b.WriteString("📁 " + esc(projectName) + "\n")
	}
	if hasPlan {
		line := "🗺 " + esc(plan.Title)
		if task.PlanStep > 0 {
			_, total, _ := s.PlanProgress(plan.ID)
			line += " · " + esc(i18n.T("step %d of %d", task.PlanStep, total))
		} else if done, total, err := s.PlanProgress(plan.ID); err == nil {
			line += " · " + esc(i18n.T("%d of %d steps done", done, total))
		}
		b.WriteString(line + "\n")
	}
	if title != "" {
		b.WriteString("📝 <b>" + esc(title) + "</b>\n")
	}
	agent := ev.Agent
	if run.Agent != "" {
		agent = run.Agent
	} else if agent == "" {
		agent = task.Agent
	}
	var facts []string
	if agent != "" {
		facts = append(facts, "🤖 "+esc(launch.AgentName(agent)))
	}
	if hasRun && run.DurationMS.Valid {
		facts = append(facts, "⏱ "+fmtDuration(time.Duration(run.DurationMS.Int64)*time.Millisecond))
	}
	if hasRun && (run.TokensIn.Valid || run.TokensOut.Valid) {
		facts = append(facts, "🔤 "+fmtTokens(run.TokensIn.Int64)+" in · "+fmtTokens(run.TokensOut.Int64)+" out")
	}
	if hasRun && run.CostUSD.Valid && run.CostUSD.Float64 > 0 {
		facts = append(facts, fmt.Sprintf("💰 $%.2f", run.CostUSD.Float64))
	}
	if len(facts) > 0 {
		b.WriteString(strings.Join(facts, "  ·  ") + "\n")
	}
	branch := ev.Branch
	if run.Branch != "" {
		branch = run.Branch
	}
	if branch != "" {
		b.WriteString("🌿 <code>" + esc(branch) + "</code>\n")
	}
	if hasRun && run.Checks != "" && run.Checks != "null" {
		var checks []launch.CheckResult
		if json.Unmarshal([]byte(run.Checks), &checks) == nil && len(checks) > 0 {
			b.WriteString("\n<b>" + esc(i18n.T("Checks")) + "</b>\n")
			for _, c := range checks {
				mark := "✅"
				if !c.OK {
					mark = "❌"
				}
				b.WriteString(mark + " <code>" + esc(short(c.Cmd, 70)) + "</code>\n")
				if !c.OK && strings.TrimSpace(c.Output) != "" {
					b.WriteString("<pre>" + esc(lastLines(c.Output, 8, 600)) + "</pre>\n")
				}
			}
		}
	}
	note := ""
	switch {
	case ev.Event == launch.HookBlocked && run.Note != "":
		note = run.Note
	case ev.Message != "" && ev.Event != launch.HookReview:
		note = ev.Message
	}
	if note != "" {
		b.WriteString("\n" + esc(short(note, 500)) + "\n")
	}
	summary := ""
	if hasRun {
		summary = run.Summary
	} else if ev.Event == launch.HookReview {
		summary = ev.Message
	}
	if body := summaryExcerpt(summary); body != "" && (ev.Event == launch.HookReview || ev.Event == launch.HookBlocked || ev.Event == launch.HookApprove) {
		b.WriteString("\n<b>" + esc(i18n.T("What the agent says")) + "</b>\n")
		b.WriteString("<blockquote expandable>" + markdownToHTML(body) + "</blockquote>\n")
	}
	if next := nextStep(s, ev, plan, hasPlan); next != "" {
		b.WriteString("\n👉 " + next)
	}
	return limit(b.String())
}

func eventHeadline(event string) string {
	switch event {
	case launch.HookReview:
		return "✅ " + i18n.T("Ready for review")
	case launch.HookBlocked:
		return "⛔ " + i18n.T("Blocked: it needs you")
	case launch.HookApprove:
		return "👍 " + i18n.T("Approved")
	case launch.HookMerge:
		return "🔀 " + i18n.T("Merged")
	case launch.HookLaunch:
		return "🚀 " + i18n.T("Agent started")
	case launch.HookDiscard:
		return "🗑 " + i18n.T("Discarded")
	case launch.HookTaskCreated:
		return "🆕 " + i18n.T("New task")
	case launch.HookPlanStart:
		return "🗺 " + i18n.T("Plan started")
	case launch.HookPlanDone:
		return "🏁 " + i18n.T("Plan done: ready to merge")
	}
	return "Troopa · " + event
}

func nextStep(s *store.Store, ev launch.HookEvent, plan store.Plan, hasPlan bool) string {
	switch ev.Event {
	case launch.HookReview:
		return esc(i18n.T("In Troopa: y approves, x sends it back with your comments."))
	case launch.HookBlocked:
		return esc(i18n.T("Open the task in Troopa and press x to tell the agent what to fix."))
	case launch.HookPlanDone:
		return esc(i18n.T("Open the plan (w) and press g to merge it."))
	case launch.HookApprove:
		if hasPlan && s != nil {
			if next, ok, err := s.NextPlanStep(plan.ID); err == nil && ok {
				return esc(i18n.T("Next step: %s", next.Title))
			}
		}
	}
	return ""
}

// summaryExcerpt keeps the most useful part of SUMMARY.md: the "what
// changed" section when there is one, then "how to run".
func summaryExcerpt(md string) string {
	md = strings.TrimSpace(md)
	if md == "" {
		return ""
	}
	sections := splitSections(md)
	var pick []string
	for _, want := range []string{"what changed", "changes", "summary", "qué cambió", "cambios", "resumen"} {
		for _, s := range sections {
			if strings.Contains(strings.ToLower(s.title), want) && strings.TrimSpace(s.body) != "" {
				pick = append(pick, "## "+s.title+"\n"+strings.TrimSpace(s.body))
				break
			}
		}
		if len(pick) > 0 {
			break
		}
	}
	for _, s := range sections {
		t := strings.ToLower(s.title)
		if (strings.Contains(t, "how to run") || strings.Contains(t, "cómo correr")) && strings.TrimSpace(s.body) != "" {
			pick = append(pick, "## "+s.title+"\n"+short(strings.TrimSpace(s.body), 700))
			break
		}
	}
	out := strings.Join(pick, "\n\n")
	if out == "" {
		out = md
	}
	return short(out, 2200)
}

type section struct{ title, body string }

func splitSections(md string) []section {
	var out []section
	cur := section{}
	for _, line := range strings.Split(md, "\n") {
		if t := strings.TrimLeft(line, "#"); len(t) < len(line) && strings.HasPrefix(t, " ") {
			if cur.title != "" || strings.TrimSpace(cur.body) != "" {
				out = append(out, cur)
			}
			cur = section{title: strings.TrimSpace(t)}
			continue
		}
		cur.body += line + "\n"
	}
	return append(out, cur)
}

var (
	reFence  = regexp.MustCompile("(?s)```[a-zA-Z0-9]*\n(.*?)```")
	reInline = regexp.MustCompile("`([^`\n]+)`")
	reBold   = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
)

// markdownToHTML turns the bits of Markdown agents use into Telegram HTML.
func markdownToHTML(md string) string {
	var blocks []string
	md = reFence.ReplaceAllStringFunc(md, func(m string) string {
		inner := reFence.FindStringSubmatch(m)[1]
		blocks = append(blocks, "<pre>"+esc(strings.TrimRight(inner, "\n"))+"</pre>")
		return fmt.Sprintf("\x00%d\x00", len(blocks)-1)
	})
	var lines []string
	for _, line := range strings.Split(md, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "#"):
			lines = append(lines, "<b>"+esc(strings.TrimSpace(strings.TrimLeft(trim, "#")))+"</b>")
		case strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* "):
			lines = append(lines, "• "+inline(trim[2:]))
		default:
			lines = append(lines, inline(line))
		}
	}
	out := strings.Join(lines, "\n")
	for i, blk := range blocks {
		out = strings.Replace(out, fmt.Sprintf("\x00%d\x00", i), blk, 1)
	}
	return strings.TrimSpace(out)
}

func inline(s string) string {
	s = esc(s)
	s = reInline.ReplaceAllString(s, "<code>$1</code>")
	return reBold.ReplaceAllString(s, "<b>$1</b>")
}

var reTag = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string { return html.UnescapeString(reTag.ReplaceAllString(s, "")) }

func esc(s string) string { return html.EscapeString(s) }

func short(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func lastLines(s string, n, maxChars int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := strings.Join(lines, "\n")
	if r := []rune(out); len(r) > maxChars {
		out = "…" + string(r[len(r)-maxChars:])
	}
	return out
}

// limit keeps the message under Telegram's size without cutting a tag.
func limit(s string) string {
	if len([]rune(s)) <= maxMessage {
		return s
	}
	cut := string([]rune(s)[:maxMessage])
	if i := strings.LastIndex(cut, "<blockquote"); i >= 0 && !strings.Contains(cut[i:], "</blockquote>") {
		body := stripTags(cut[i:])
		cut = cut[:i] + "<blockquote expandable>" + esc(short(body, 1500)) + "</blockquote>"
	}
	return cut
}

func fmtDuration(d time.Duration) string {
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func fmtTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}
