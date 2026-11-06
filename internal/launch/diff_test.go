package launch

import "testing"

func TestParseDiffLineNumbersFromHunks(t *testing.T) {
	raw := `diff --git a/main.go b/main.go
index 111..222 100644
--- a/main.go
+++ b/main.go
@@ -10,4 +10,5 @@ func Run() {
 	keep()
-	old()
+	added()
+	also()
 	tail()
@@ -20 +21 @@ func Other() {
-	gone()
+	here()
`
	files := ParseDiff(raw)
	if len(files) != 1 || files[0].Path != "main.go" {
		t.Fatalf("%+v", files)
	}
	want := []struct {
		kind byte
		old  int
		new  int
	}{
		{'h', 0, 0},
		{' ', 0, 0}, // index
		{' ', 0, 0}, // ---
		{' ', 0, 0}, // +++
		{'@', 0, 0},
		{' ', 10, 10},
		{'-', 11, 0},
		{'+', 0, 11},
		{'+', 0, 12},
		{' ', 12, 13},
		{'@', 0, 0},
		{'-', 20, 0},
		{'+', 0, 21},
	}
	if len(files[0].Lines) != len(want) {
		t.Fatalf("lines=%d want %d\n%+v", len(files[0].Lines), len(want), files[0].Lines)
	}
	for i, w := range want {
		got := files[0].Lines[i]
		if got.Kind != w.kind || got.Old != w.old || got.New != w.new {
			t.Errorf("line %d %+v want kind=%c old=%d new=%d text=%q", i, got, w.kind, w.old, w.new, got.Text)
		}
	}
	line, side := files[0].Lines[7].CommentTarget()
	if line != 11 || side != "+" {
		t.Fatalf("added target %d %q", line, side)
	}
	line, side = files[0].Lines[6].CommentTarget()
	if line != 11 || side != "-" {
		t.Fatalf("deleted target %d %q", line, side)
	}
}

func TestHunkStartsZeroOldFile(t *testing.T) {
	oldN, newN, ok := hunkStarts("@@ -0,0 +1,3 @@")
	if !ok || oldN != 0 || newN != 1 {
		t.Fatalf("%d %d %v", oldN, newN, ok)
	}
}
