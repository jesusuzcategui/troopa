package launch

import (
	"strings"
	"testing"
)

func TestAppendFeedbackImages(t *testing.T) {
	got := AppendFeedbackImages("fix it", []string{"/wt/.troopa/attachments/a.png"})
	if got == "" || !strings.Contains(got, "a.png") || !strings.Contains(got, "fix it") {
		t.Fatalf("%q", got)
	}
	if AppendFeedbackImages("", nil) != "" {
		t.Fatal("empty")
	}
}
