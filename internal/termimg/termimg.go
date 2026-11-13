// Package termimg detects terminal graphics support (Kitty protocol) and
// builds escape sequences to show or clear an image in the terminal.
package termimg

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Supports reports whether this terminal can draw images with the Kitty
// graphics protocol. Inside tmux without Kitty passthrough, returns false.
func Supports() bool {
	return SupportsEnv(os.Getenv)
}

// SupportsEnv is Supports with an injectable getenv (for tests).
func SupportsEnv(getenv func(string) string) bool {
	if inTmux(getenv) && !tmuxPassthrough(getenv) {
		return false
	}
	if getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	switch strings.ToLower(getenv("TERM_PROGRAM")) {
	case "kitty", "ghostty", "wezterm":
		return true
	}
	term := strings.ToLower(getenv("TERM"))
	if strings.Contains(term, "kitty") || strings.Contains(term, "ghostty") {
		return true
	}
	// WezTerm often uses xterm-256color; TERM_PROGRAM covers it.
	return false
}

func inTmux(getenv func(string) string) bool {
	return getenv("TMUX") != "" || getenv("TMUX_PANE") != ""
}

// tmuxPassthrough is true when Kitty is the outer terminal (passthrough likely).
func tmuxPassthrough(getenv func(string) string) bool {
	return getenv("KITTY_WINDOW_ID") != "" || strings.Contains(strings.ToLower(getenv("TERM")), "kitty")
}

// IsImagePath reports common image extensions.
func IsImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	}
	return false
}

// IsImageMIME reports image MIME types.
func IsImageMIME(mime string) bool {
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	return strings.HasPrefix(mime, "image/")
}

// PlaceSequence returns Kitty graphics escapes to display path scaled to
// cols×rows cells. imageID must be a positive int used again in ClearSequence.
func PlaceSequence(path string, imageID, cols, rows int) (string, error) {
	if imageID < 1 {
		imageID = 1
	}
	if cols < 1 {
		cols = 40
	}
	if rows < 1 {
		rows = 12
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("not a file: %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(raw)
	fmtType := kittyFormat(path)
	// Transmit and place: a=T (transmit+display), f=format, i=id, c/cols r/rows.
	var b strings.Builder
	chunks := chunkBase64(b64, 4096)
	for i, chunk := range chunks {
		more := 1
		if i == len(chunks)-1 {
			more = 0
		}
		if i == 0 {
			fmt.Fprintf(&b, "\033_Ga=T,f=%d,i=%d,c=%d,r=%d,m=%d;%s\033\\", fmtType, imageID, cols, rows, more, chunk)
		} else {
			fmt.Fprintf(&b, "\033_Gm=%d;%s\033\\", more, chunk)
		}
	}
	return b.String(), nil
}

// ClearSequence deletes a previously placed image by id.
func ClearSequence(imageID int) string {
	if imageID < 1 {
		imageID = 1
	}
	return fmt.Sprintf("\033_Ga=d,d=i,i=%d\033\\", imageID)
}

func kittyFormat(path string) int {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return 100
	case ".jpg", ".jpeg":
		return 100 // kitty decodes via PNG payload preferably; 100=PNG, but jpeg often works as file
	case ".gif":
		return 100
	default:
		return 100
	}
}

func chunkBase64(s string, n int) []string {
	if n < 1 {
		n = 4096
	}
	var out []string
	for len(s) > 0 {
		if len(s) <= n {
			out = append(out, s)
			break
		}
		out = append(out, s[:n])
		s = s[n:]
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// ImageRef parses "[image: path]" / "[imagen: path]" markers.
func ImageRef(line string) (path string, ok bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return "", false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	lower := strings.ToLower(inner)
	for _, prefix := range []string{"image:", "imagen:"} {
		if strings.HasPrefix(lower, prefix) {
			p := strings.TrimSpace(inner[len(prefix):])
			if p != "" {
				return p, true
			}
		}
	}
	return "", false
}
