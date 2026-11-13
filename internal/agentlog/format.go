// Package agentlog turns agent stream-json logs (Claude Code, Cursor, and
// similar) into readable lines for the TUI and the desktop app. It never
// writes agent.log.
package agentlog

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Kind classifies a formatted line.
const (
	KindText     = "text"
	KindThinking = "thinking"
	KindTool     = "tool"
	KindSystem   = "system"
	KindResult   = "result"
	KindRaw      = "raw"
	KindUnknown  = "unknown"
	KindMeta     = "meta" // fold headers, separators
)

// Line is one display row after formatting.
type Line struct {
	Kind    string
	Text    string
	Dim     bool   // muted style (thinking, unknown, meta)
	FoldKey string // non-empty: thinking block id for fold/expand
}

// Options control Format.
type Options struct {
	// ExpandThinking shows full thinking text; otherwise a one-line summary.
	ExpandThinking bool
	// MaxToolOut caps optional tool output lines (0 = omit output).
	MaxToolOut int
}

// Format turns raw agent.log bytes into display lines. Non-JSON lines pass
// through; JSON events are grouped (thinking deltas, text deltas, tools).
func Format(raw string, opt Options) []Line {
	if opt.MaxToolOut < 0 {
		opt.MaxToolOut = 0
	}
	f := &formatter{opt: opt, tools: map[string]*toolState{}}
	for _, line := range strings.Split(raw, "\n") {
		f.feed(line)
	}
	f.flushText()
	f.flushThinking()
	f.flushOpenTools()
	return f.out
}

type toolState struct {
	label  string
	done   bool
	ok     bool
	output string
	line   int // index in out when started
}

type formatter struct {
	opt      Options
	out      []Line
	thinking strings.Builder
	text     strings.Builder
	thinkID  int
	tools    map[string]*toolState
	// cursorAssist tracks last assistant text for stream-partial dedupe
	cursorAssist string
	seenAssist   map[string]bool
}

func (f *formatter) feed(line string) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		f.flushText()
		f.flushThinking()
		return
	}
	if line[0] != '{' {
		f.flushText()
		f.flushThinking()
		f.out = append(f.out, Line{Kind: KindRaw, Text: line})
		return
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		f.flushText()
		f.flushThinking()
		f.out = append(f.out, Line{Kind: KindRaw, Text: line})
		return
	}
	typ, _ := m["type"].(string)
	sub, _ := m["subtype"].(string)
	switch typ {
	case "system":
		f.flushText()
		f.flushThinking()
		f.handleSystem(m, sub)
	case "result":
		f.flushText()
		f.flushThinking()
		f.handleResult(m, sub)
	case "assistant":
		f.handleAssistant(m)
	case "user":
		f.handleUser(m)
	case "thinking":
		f.handleThinkingEvent(m, sub)
	case "tool_call":
		f.flushText()
		f.flushThinking()
		f.handleCursorTool(m, sub)
	case "stream_event":
		f.handleStreamEvent(m)
	case "content_block_delta", "content_block_start", "content_block_stop", "message_start", "message_delta", "message_stop":
		f.handleAnthropicEvent(typ, m)
	case "tool_use":
		f.flushText()
		f.flushThinking()
		f.startTool(str(m["id"]), toolLabel(str(m["name"]), asMap(m["input"])))
	case "tool_result":
		f.completeTool(str(m["tool_use_id"]), !truthy(m["is_error"]), stringify(m["content"]))
	default:
		if typ == "" {
			f.flushText()
			f.flushThinking()
			f.out = append(f.out, Line{Kind: KindRaw, Text: line})
			return
		}
		f.flushText()
		f.flushThinking()
		f.out = append(f.out, Line{Kind: KindUnknown, Text: "· " + typ, Dim: true})
	}
}

func (f *formatter) handleSystem(m map[string]any, sub string) {
	if sub != "init" && sub != "" {
		f.out = append(f.out, Line{Kind: KindUnknown, Text: "· system/" + sub, Dim: true})
		return
	}
	model := str(m["model"])
	parts := []string{"session"}
	if model != "" {
		parts = []string{model}
	}
	if cwd := str(m["cwd"]); cwd != "" {
		parts = append(parts, filepath.Base(cwd))
	}
	f.out = append(f.out, Line{Kind: KindSystem, Text: "● " + strings.Join(parts, " · ")})
}

func (f *formatter) handleResult(m map[string]any, sub string) {
	parts := []string{"done"}
	if sub != "" && sub != "success" {
		parts[0] = sub
	}
	if truthy(m["is_error"]) {
		parts[0] = "error"
	}
	if ms, ok := asFloat(m["duration_ms"]); ok && ms > 0 {
		parts = append(parts, formatDuration(ms))
	}
	if u := asMap(m["usage"]); u != nil {
		if tok := usageTokens(u); tok != "" {
			parts = append(parts, tok)
		}
	}
	if cost, ok := asFloat(m["total_cost_usd"]); ok && cost > 0 {
		parts = append(parts, fmt.Sprintf("$%.4f", cost))
	}
	if model := str(m["model"]); model != "" {
		parts = append(parts, model)
	}
	f.out = append(f.out, Line{Kind: KindResult, Text: "■ " + strings.Join(parts, " · ")})
}

func (f *formatter) handleAssistant(m map[string]any) {
	// Cursor stream-partial: skip duplicate flushes.
	_, hasTS := m["timestamp_ms"]
	_, hasCall := m["model_call_id"]
	if hasCall {
		return // buffered flush before tool
	}
	msg := asMap(m["message"])
	if msg == nil {
		return
	}
	content, _ := msg["content"].([]any)
	if content == nil {
		return
	}
	var textBits strings.Builder
	for _, c := range content {
		block, ok := c.(map[string]any)
		if !ok {
			continue
		}
		switch str(block["type"]) {
		case "thinking":
			f.flushText()
			t := str(block["thinking"])
			if t == "" {
				t = str(block["text"])
			}
			f.thinking.WriteString(t)
		case "text":
			f.flushThinking()
			textBits.WriteString(str(block["text"]))
		case "tool_use":
			f.flushThinking()
			if textBits.Len() > 0 {
				f.appendText(textBits.String())
				textBits.Reset()
			}
			f.flushText()
			f.startTool(str(block["id"]), toolLabel(str(block["name"]), asMap(block["input"])))
		}
	}
	text := textBits.String()
	if text == "" {
		return
	}
	// Final flush without timestamp_ms duplicates the turn.
	if !hasTS && f.cursorAssist != "" && (text == f.cursorAssist || strings.HasPrefix(f.cursorAssist, text) || strings.HasPrefix(text, f.cursorAssist)) {
		return
	}
	if hasTS {
		// Streaming delta: append fragment.
		f.appendText(text)
		f.cursorAssist += text
		return
	}
	// Complete assistant segment (no stream-partial).
	if f.seenAssist == nil {
		f.seenAssist = map[string]bool{}
	}
	if f.seenAssist[text] {
		return
	}
	f.seenAssist[text] = true
	f.appendText(text)
	f.flushText()
	f.cursorAssist = text
}

func (f *formatter) handleUser(m map[string]any) {
	msg := asMap(m["message"])
	if msg == nil {
		return
	}
	content, _ := msg["content"].([]any)
	for _, c := range content {
		block, ok := c.(map[string]any)
		if !ok {
			continue
		}
		switch str(block["type"]) {
		case "tool_result":
			f.completeTool(str(block["tool_use_id"]), !truthy(block["is_error"]), stringify(block["content"]))
		}
	}
}

func (f *formatter) handleThinkingEvent(m map[string]any, sub string) {
	switch sub {
	case "delta", "":
		f.flushText()
		f.thinking.WriteString(str(m["text"]))
	case "completed":
		f.flushThinking()
	default:
		// ignore unknown subtypes
	}
}

func (f *formatter) handleCursorTool(m map[string]any, sub string) {
	id := str(m["call_id"])
	if i := strings.IndexByte(id, '\n'); i >= 0 {
		id = id[:i]
	}
	tc := asMap(m["tool_call"])
	name, args, result, okResult := parseCursorTool(tc)
	switch sub {
	case "started":
		f.startTool(id, toolLabel(name, args))
	case "completed":
		out := ""
		success := true
		if result != nil {
			if _, err := result["error"]; err {
				success = false
				out = stringify(result["error"])
			} else if s := asMap(result["success"]); s != nil {
				out = stringify(s["content"])
				if out == "" {
					out = stringify(s)
				}
			} else {
				out = stringify(result)
			}
		}
		if !okResult && result == nil {
			success = true
		}
		f.completeTool(id, success, out)
	}
}

func (f *formatter) handleStreamEvent(m map[string]any) {
	ev := asMap(m["event"])
	if ev == nil {
		return
	}
	f.handleAnthropicEvent(str(ev["type"]), ev)
}

func (f *formatter) handleAnthropicEvent(typ string, m map[string]any) {
	switch typ {
	case "content_block_start":
		block := asMap(m["content_block"])
		if block == nil {
			return
		}
		switch str(block["type"]) {
		case "thinking":
			f.flushText()
		case "text":
			f.flushThinking()
		case "tool_use":
			f.flushText()
			f.flushThinking()
			f.startTool(str(block["id"]), toolLabel(str(block["name"]), asMap(block["input"])))
		}
	case "content_block_delta":
		delta := asMap(m["delta"])
		if delta == nil {
			return
		}
		switch str(delta["type"]) {
		case "thinking_delta":
			f.flushText()
			f.thinking.WriteString(str(delta["thinking"]))
		case "text_delta":
			f.flushThinking()
			f.appendText(str(delta["text"]))
		case "input_json_delta":
			// accumulate ignored; label comes from start/final assistant
		}
	case "content_block_stop":
		f.flushText()
		f.flushThinking()
	}
}

func (f *formatter) appendText(s string) {
	if s == "" {
		return
	}
	f.text.WriteString(s)
}

func (f *formatter) flushText() {
	s := strings.TrimRight(f.text.String(), "\n")
	f.text.Reset()
	if strings.TrimSpace(s) == "" {
		return
	}
	for _, line := range strings.Split(s, "\n") {
		f.out = append(f.out, Line{Kind: KindText, Text: line})
	}
}

func (f *formatter) flushThinking() {
	s := strings.TrimSpace(f.thinking.String())
	f.thinking.Reset()
	if s == "" {
		return
	}
	f.thinkID++
	key := fmt.Sprintf("t%d", f.thinkID)
	if !f.opt.ExpandThinking {
		preview := s
		if r := []rune(preview); len(r) > 72 {
			preview = string(r[:72]) + "…"
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		f.out = append(f.out, Line{
			Kind:    KindThinking,
			Text:    "▸ thinking · " + preview,
			Dim:     true,
			FoldKey: key,
		})
		return
	}
	f.out = append(f.out, Line{
		Kind:    KindMeta,
		Text:    "▾ thinking",
		Dim:     true,
		FoldKey: key,
	})
	for _, line := range strings.Split(s, "\n") {
		f.out = append(f.out, Line{Kind: KindThinking, Text: "  " + line, Dim: true, FoldKey: key})
	}
}

func (f *formatter) startTool(id, label string) {
	f.flushText()
	f.flushThinking()
	if id == "" {
		id = fmt.Sprintf("anon-%d", len(f.tools)+1)
	}
	if label == "" {
		label = "tool"
	}
	st := &toolState{label: label, line: len(f.out)}
	f.tools[id] = st
	f.out = append(f.out, Line{Kind: KindTool, Text: "▸ " + label})
}

func (f *formatter) completeTool(id string, ok bool, output string) {
	st := f.tools[id]
	if st == nil {
		// completion without start: still show
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		label := "tool"
		f.out = append(f.out, Line{Kind: KindTool, Text: mark + " " + label})
		f.emitToolOut(output)
		return
	}
	st.done = true
	st.ok = ok
	st.output = output
	mark := "✓"
	if !ok {
		mark = "✗"
	}
	if st.line >= 0 && st.line < len(f.out) {
		f.out[st.line].Text = mark + " " + st.label
	}
	f.emitToolOut(output)
}

func (f *formatter) emitToolOut(output string) {
	if f.opt.MaxToolOut <= 0 || strings.TrimSpace(output) == "" {
		return
	}
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) > f.opt.MaxToolOut {
		lines = append(lines[:f.opt.MaxToolOut], "…")
	}
	for _, line := range lines {
		f.out = append(f.out, Line{Kind: KindRaw, Text: "    " + truncateRunes(line, 120), Dim: true})
	}
}

func (f *formatter) flushOpenTools() {
	// leave unfinished tools as ▸
}

func toolLabel(name string, input map[string]any) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "tool"
	}
	// Cursor: readToolCall → Read
	if strings.HasSuffix(name, "ToolCall") {
		base := strings.TrimSuffix(name, "ToolCall")
		name = strings.ToUpper(base[:1]) + base[1:]
	}
	switch strings.ToLower(name) {
	case "read", "readfile", "read_file":
		if p := firstPath(input); p != "" {
			return "Read " + p
		}
	case "edit", "editfile", "edit_file", "strreplace", "search_replace":
		if p := firstPath(input); p != "" {
			return "Edit " + p
		}
	case "write", "writefile", "write_file", "createfile":
		if p := firstPath(input); p != "" {
			return "Write " + p
		}
	case "bash", "shell", "run_terminal_cmd", "shelltoolcall":
		if cmd := firstString(input, "command", "cmd", "shell_command"); cmd != "" {
			return "Bash: " + truncateRunes(cmd, 80)
		}
	case "grep", "search":
		if q := firstString(input, "pattern", "query", "regex"); q != "" {
			return name + ": " + truncateRunes(q, 60)
		}
	}
	if p := firstPath(input); p != "" {
		return name + " " + p
	}
	if cmd := firstString(input, "command", "cmd"); cmd != "" {
		return name + ": " + truncateRunes(cmd, 80)
	}
	return name
}

func parseCursorTool(tc map[string]any) (name string, args map[string]any, result map[string]any, ok bool) {
	if tc == nil {
		return "", nil, nil, false
	}
	if fn := asMap(tc["function"]); fn != nil {
		name = str(fn["name"])
		if a := str(fn["arguments"]); a != "" {
			var m map[string]any
			if json.Unmarshal([]byte(a), &m) == nil {
				args = m
			}
		} else {
			args = asMap(fn["arguments"])
		}
		return name, args, asMap(tc["result"]), true
	}
	for k, v := range tc {
		inner := asMap(v)
		if inner == nil {
			continue
		}
		name = k
		args = asMap(inner["args"])
		result = asMap(inner["result"])
		return name, args, result, true
	}
	return "", nil, nil, false
}

func firstPath(input map[string]any) string {
	return firstString(input, "file_path", "path", "file", "filename", "target_file", "filePath")
}

func firstString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if s := str(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func usageTokens(u map[string]any) string {
	in, _ := asFloat(u["input_tokens"])
	if in == 0 {
		in, _ = asFloat(u["inputTokens"])
	}
	out, _ := asFloat(u["output_tokens"])
	if out == 0 {
		out, _ = asFloat(u["outputTokens"])
	}
	if in == 0 && out == 0 {
		return ""
	}
	parts := []string{}
	if in > 0 {
		parts = append(parts, fmt.Sprintf("↑%s", formatCount(in)))
	}
	if out > 0 {
		parts = append(parts, fmt.Sprintf("↓%s", formatCount(out)))
	}
	return strings.Join(parts, " ")
}

func formatCount(n float64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", n/1_000_000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", n/1000)
	}
	return fmt.Sprintf("%.0f", n)
}

func formatDuration(ms float64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%v", t)
	case bool:
		return fmt.Sprintf("%v", t)
	default:
		return ""
	}
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	}
	return 0, false
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	}
	return false
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any:
		var b strings.Builder
		for i, x := range t {
			if i > 0 {
				b.WriteByte('\n')
			}
			if m, ok := x.(map[string]any); ok {
				if str(m["type"]) == "text" {
					b.WriteString(str(m["text"]))
					continue
				}
			}
			b.WriteString(stringify(x))
		}
		return b.String()
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(raw)
	}
}

// LinesText joins formatted lines for plain-string consumers.
func LinesText(lines []Line) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l.Text)
	}
	return b.String()
}
