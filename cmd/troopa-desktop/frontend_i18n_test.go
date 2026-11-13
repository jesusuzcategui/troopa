package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestDesktopSpanishCoversTCalls fails when a desktop UI string is used via
// t(...) without a Spanish entry in desktop/frontend/src/i18n.ts.
func TestDesktopSpanishCoversTCalls(t *testing.T) {
	_, this, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(this), "..", ".."))
	srcDir := filepath.Join(root, "desktop", "frontend", "src")
	catalogPath := filepath.Join(srcDir, "i18n.ts")

	es, err := loadDesktopESCatalog(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	var empty []string
	for key, val := range es {
		if strings.TrimSpace(val) == "" {
			empty = append(empty, key)
		}
	}
	if len(empty) > 0 {
		sort.Strings(empty)
		t.Fatalf("desktop Spanish catalog has empty translations for EN keys:\n  %s", strings.Join(empty, "\n  "))
	}

	var missing []string
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == "i18n.ts" || (!strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".vue")) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, key := range desktopTCallKeys(string(body)) {
			if _, have := es[key]; !have {
				missing = append(missing, rel+": "+key)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("desktop Spanish catalog missing keys:\n  %s", strings.Join(missing, "\n  "))
	}
}

var (
	desktopTCall    = regexp.MustCompile(`\bt\(\s*(?:lang|props\.lang)\s*,\s*(?:"((?:\\.|[^"\\])*)"|'((?:\\.|[^'\\])*)')`)
	desktopTTernary = regexp.MustCompile(`\bt\(\s*(?:lang|props\.lang)\s*,\s*[^\n,?]+?\?\s*["']((?:\\.|[^"'\\])*)["']\s*:\s*["']((?:\\.|[^"'\\])*)["']`)
)

func loadDesktopESCatalog(path string) (map[string]string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(body)
	start := strings.Index(text, "const es:")
	if start < 0 {
		start = strings.Index(text, "const es =")
	}
	if start < 0 {
		return nil, errString("es catalog not found in " + path)
	}
	brace := strings.Index(text[start:], "{")
	if brace < 0 {
		return nil, errString("es catalog brace not found")
	}
	from := start + brace
	end := strings.Index(text[from:], "\n};")
	if end < 0 {
		return nil, errString("es catalog end not found")
	}
	block := text[from : from+end]
	out := map[string]string{}
	entryFull := regexp.MustCompile(`(?m)^\s*"((?:\\.|[^"\\])*)"\s*:\s*"((?:\\.|[^"\\])*)"`)
	for _, m := range entryFull.FindAllStringSubmatch(block, -1) {
		out[unquoteTS(m[1])] = unquoteTS(m[2])
	}
	if len(out) == 0 {
		return nil, errString("empty es catalog")
	}
	return out, nil
}

func desktopTCallKeys(src string) []string {
	seen := map[string]struct{}{}
	var keys []string
	add := func(k string) {
		k = unquoteTS(k)
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for _, m := range desktopTTernary.FindAllStringSubmatch(src, -1) {
		add(m[1])
		add(m[2])
	}
	for _, m := range desktopTCall.FindAllStringSubmatch(src, -1) {
		if m[1] != "" {
			add(m[1])
		}
		if m[2] != "" {
			add(m[2])
		}
	}
	return keys
}

func unquoteTS(s string) string {
	return strings.ReplaceAll(s, `\"`, `"`)
}

type errString string

func (e errString) Error() string { return string(e) }
