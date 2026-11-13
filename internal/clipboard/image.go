// Package clipboard reads images from the system clipboard (Wayland/X11).
package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ImageFromClipboard saves a clipboard image to a temp PNG and returns its path.
// Prefer Wayland (wl-paste), then X11 (xclip). Clear error if neither works.
func ImageFromClipboard() (path string, err error) {
	if _, err := exec.LookPath("wl-paste"); err == nil {
		return fromWlPaste()
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return fromXclip()
	}
	return "", fmt.Errorf("no clipboard tool: install wl-paste (Wayland) or xclip (X11)")
}

func fromWlPaste() (string, error) {
	out, err := exec.Command("wl-paste", "--list-types").Output()
	if err != nil {
		return "", fmt.Errorf("clipboard: %w", err)
	}
	types := string(out)
	mime := ""
	for _, t := range []string{"image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif"} {
		if strings.Contains(types, t) {
			mime = t
			break
		}
	}
	if mime == "" {
		return "", fmt.Errorf("clipboard has no image (types: %s)", strings.TrimSpace(strings.ReplaceAll(types, "\n", ", ")))
	}
	ext := ".png"
	switch mime {
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	}
	f, err := os.CreateTemp("", "troopa-clip-*"+ext)
	if err != nil {
		return "", err
	}
	defer f.Close()
	cmd := exec.Command("wl-paste", "-t", mime)
	cmd.Stdout = f
	if err := cmd.Run(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("wl-paste: %w", err)
	}
	st, err := f.Stat()
	if err != nil || st.Size() == 0 {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("clipboard image is empty")
	}
	return f.Name(), nil
}

func fromXclip() (string, error) {
	out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
	if err != nil {
		return "", fmt.Errorf("clipboard: %w", err)
	}
	types := string(out)
	mime := ""
	for _, t := range []string{"image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif"} {
		if strings.Contains(types, t) {
			mime = t
			break
		}
	}
	if mime == "" {
		return "", fmt.Errorf("clipboard has no image")
	}
	ext := ".png"
	switch mime {
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	}
	f, err := os.CreateTemp("", "troopa-clip-*"+ext)
	if err != nil {
		return "", err
	}
	defer f.Close()
	cmd := exec.Command("xclip", "-selection", "clipboard", "-t", mime, "-o")
	cmd.Stdout = f
	if err := cmd.Run(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("xclip: %w", err)
	}
	st, err := f.Stat()
	if err != nil || st.Size() == 0 {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("clipboard image is empty")
	}
	return f.Name(), nil
}
