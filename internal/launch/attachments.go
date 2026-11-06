package launch

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/store"
)

const workAttachmentsDir = ".troopa/attachments"

func CopyAttachments(worktree string, atts []store.Attachment) error {
	destDir := filepath.Join(worktree, workAttachmentsDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	for _, a := range atts {
		src := a.Path
		if src == "" {
			continue
		}
		name := a.Name
		if name == "" {
			name = filepath.Base(src)
		}
		dest := filepath.Join(destDir, name)
		if err := copyFile(src, dest); err != nil {
			return fmt.Errorf("copy attachment %s: %w", name, err)
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dest)
		return err
	}
	return out.Close()
}

func StageTaskContext(worktree string, t store.Task, atts []store.Attachment, brainDir, slug string) error {
	if err := ExcludeWorkFiles(worktree); err != nil {
		return err
	}
	if err := CopyAttachments(worktree, atts); err != nil {
		return err
	}
	return writeTASK(worktree, t, atts, brain.TASKContext(brainDir, slug))
}

func MediaDuration(path string) string {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return ""
	}
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	sec, err := strconv.ParseFloat(s, 64)
	if err != nil || sec <= 0 {
		return ""
	}
	d := time.Duration(sec * float64(time.Second)).Round(time.Second)
	min := int(d.Minutes())
	secPart := int(d.Seconds()) % 60
	if min > 0 {
		return fmt.Sprintf("%dm%02ds", min, secPart)
	}
	return fmt.Sprintf("%ds", secPart)
}

func isAV(mime string) bool {
	mime = strings.ToLower(mime)
	return strings.HasPrefix(mime, "video/") || strings.HasPrefix(mime, "audio/")
}
