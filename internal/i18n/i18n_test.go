package i18n

import (
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		troopa, lcAll, lang string
		want                string
	}{
		{"", "", "es_CO.UTF-8", ES},
		{"", "", "en_US.UTF-8", EN},
		{"", "", "", EN},
		{"", "C", "es_ES.UTF-8", ES},
		{"en", "", "es_CO.UTF-8", EN},
		{"", "fr_FR.UTF-8", "es_CO.UTF-8", EN},
	}
	for _, c := range cases {
		t.Setenv("TROOPA_LANG", c.troopa)
		t.Setenv("LC_ALL", c.lcAll)
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", c.lang)
		if got := Detect(); got != c.want {
			t.Fatalf("%+v: got %s", c, got)
		}
	}
}

func TestT(t *testing.T) {
	defer SetLang(lang)
	SetLang(ES)
	if got := T("New task"); got != "Nueva tarea" {
		t.Fatalf("es: %q", got)
	}
	if got := T("Archive «%s»?", "API"); got != "¿Archivar «API»?" {
		t.Fatalf("es format: %q", got)
	}
	if got := T("not in the catalog"); got != "not in the catalog" {
		t.Fatalf("fallback: %q", got)
	}
	SetLang(EN)
	if got := T("New task"); got != "New task" {
		t.Fatalf("en: %q", got)
	}
	SetLang("fr")
	if Lang() != EN {
		t.Fatalf("unknown language should fall back to en, got %s", Lang())
	}
}

func TestSpanishCoversCloseTaskKeys(t *testing.T) {
	keys := []string{
		"Merged %s into %s (%s) as %s",
		"Worktree removed, branch %s kept",
		"Discard? Approved commits on this local branch have not been merged and will be lost.",
		"Merge instead",
		"Clean the worktree and keep the branch?",
		"Merge %s into %s (%s)?",
		"g merge  c clean  d discard  n/p file  j/k scroll  o editor  esc board",
		"No",
		"Commits",
	}
	for _, k := range keys {
		if _, ok := es[k]; !ok {
			t.Errorf("missing Spanish for %q", k)
		}
	}
}

// Every Spanish entry must keep the same format verbs as its English key.
func TestSpanishKeepsVerbs(t *testing.T) {
	for en, tr := range es {
		if strings.Count(en, "%") != strings.Count(tr, "%") {
			t.Errorf("format verbs differ:\n en: %q\n es: %q", en, tr)
		}
		if strings.TrimSpace(tr) == "" {
			t.Errorf("empty translation for %q", en)
		}
	}
}
