package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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

func TestSpanishCoversTCalls(t *testing.T) {
	_, this, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(this), "..", ".."))
	fset := token.NewFileSet()
	var missing []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "T" || len(call.Args) == 0 {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "i18n" {
				return true
			}
			key := tCallKey(call.Args[0])
			if key == "" {
				return true
			}
			if _, have := es[key]; !have {
				rel, _ := filepath.Rel(root, path)
				missing = append(missing, rel+": "+key)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Fatalf("Spanish catalog missing keys:\n  %s", strings.Join(missing, "\n  "))
	}
}

func tCallKey(arg ast.Expr) string {
	switch v := arg.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				return ""
			}
			return s
		}
	case *ast.SelectorExpr:
		if pkg, ok := v.X.(*ast.Ident); ok && pkg.Name == "i18n" {
			return constValue(v.Sel.Name)
		}
	case *ast.Ident:
		return constValue(v.Name)
	}
	return ""
}

func constValue(name string) string {
	switch name {
	case "HelpText":
		return HelpText
	case "WelcomeText":
		return WelcomeText
	case "AboutCreator":
		return AboutCreator
	default:
		return name
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
		"1 pending comment",
		"%d pending comments",
		"y approve  x request changes  C comments  enter comment  e edit  D delete  d discard  r fold  [ ] round  n/p file  j/k line  o editor  esc board",
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

func TestApply(t *testing.T) {
	defer SetLang(lang)
	t.Setenv("TROOPA_LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "es_CO.UTF-8")
	Apply("")
	if Lang() != EN {
		t.Fatal("empty preference must default to English")
	}
	Apply("auto")
	if Lang() != ES {
		t.Fatal("auto should follow LANG")
	}
	Apply("es")
	if Lang() != ES {
		t.Fatal("es")
	}
	t.Setenv("TROOPA_LANG", "en")
	Apply("es")
	if Lang() != EN {
		t.Fatal("TROOPA_LANG must win")
	}
}
