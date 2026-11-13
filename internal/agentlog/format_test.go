package agentlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatClaudeCode(t *testing.T) {
	raw := readFixture(t, "claude-code.ndjson")
	lines := Format(raw, Options{})
	text := LinesText(lines)
	if strings.Contains(text, `"type"`) {
		t.Fatalf("raw JSON leaked:\n%s", text)
	}
	if !strings.Contains(text, "claude-sonnet-4") {
		t.Fatalf("missing system/model:\n%s", text)
	}
	if !strings.Contains(text, "▸ thinking ·") {
		t.Fatalf("missing collapsed thinking:\n%s", text)
	}
	if strings.Contains(text, "▾ thinking") {
		t.Fatalf("thinking should stay folded by default:\n%s", text)
	}
	if !strings.Contains(text, "I will read TASK.md and run the tests.") {
		t.Fatalf("missing assistant text:\n%s", text)
	}
	if !strings.Contains(text, "✓ Read TASK.md") {
		t.Fatalf("missing Read tool:\n%s", text)
	}
	if !strings.Contains(text, "✓ Bash: go test ./...") {
		t.Fatalf("missing Bash tool:\n%s", text)
	}
	if !strings.Contains(text, "✓ Edit internal/x.go") {
		t.Fatalf("missing Edit tool:\n%s", text)
	}
	if !strings.Contains(text, "■ done") || !strings.Contains(text, "$0.0123") {
		t.Fatalf("missing result summary:\n%s", text)
	}
	if !strings.Contains(text, "plain status line") {
		t.Fatalf("non-JSON line dropped:\n%s", text)
	}
}

func TestFormatClaudeThinkingExpanded(t *testing.T) {
	raw := readFixture(t, "claude-code.ndjson")
	text := LinesText(Format(raw, Options{ExpandThinking: true}))
	if !strings.Contains(text, "I should read the task file first.") {
		t.Fatalf("expanded thinking missing body:\n%s", text)
	}
	if !strings.Contains(text, "▾ thinking") {
		t.Fatalf("missing expand header:\n%s", text)
	}
}

func TestFormatCursor(t *testing.T) {
	raw := readFixture(t, "cursor.ndjson")
	lines := Format(raw, Options{})
	text := LinesText(lines)
	if strings.Contains(text, `"type"`) {
		t.Fatalf("raw JSON leaked:\n%s", text)
	}
	if !strings.Contains(text, "▸ thinking") {
		t.Fatalf("thinking deltas not joined:\n%s", text)
	}
	// Deltas joined; duplicate flush skipped.
	if strings.Count(text, "I'll read TASK.md") != 1 {
		t.Fatalf("assistant text not deduped:\n%s", text)
	}
	if !strings.Contains(text, "I'll read TASK.md now.") {
		t.Fatalf("missing joined assistant text:\n%s", text)
	}
	if !strings.Contains(text, "✓ Read TASK.md") {
		t.Fatalf("missing Read:\n%s", text)
	}
	if !strings.Contains(text, "✗ Bash: go test ./...") && !strings.Contains(text, "✗ Shell: go test ./...") {
		// shellToolCall → Shell
		if !strings.Contains(text, "✗") || !strings.Contains(text, "go test") {
			t.Fatalf("missing failed shell tool:\n%s", text)
		}
	}
	if !strings.Contains(text, "■ done") {
		t.Fatalf("missing result:\n%s", text)
	}
	if !strings.Contains(text, "· mystery_event") {
		t.Fatalf("unknown type not summarized:\n%s", text)
	}
}

func TestFormatStreamDeltasClaude(t *testing.T) {
	raw := strings.Join([]string{
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"Hmm "}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"yes."}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello "}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"world"}}}`,
	}, "\n")
	text := LinesText(Format(raw, Options{ExpandThinking: true}))
	if !strings.Contains(text, "Hmm yes.") {
		t.Fatalf("thinking deltas:\n%s", text)
	}
	if !strings.Contains(text, "Hello world") {
		t.Fatalf("text deltas:\n%s", text)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
