package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/i18n"
)

func PidPath(dataDir string) string { return filepath.Join(dataDir, "daemon.pid") }
func LogPath(dataDir string) string { return filepath.Join(dataDir, "daemon.log") }

// Acquire writes a pid file so only one daemon runs. The returned function
// removes it if this process still owns it.
func Acquire(dataDir string) (func() error, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	path := PidPath(dataDir)
	for range 2 {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() error { return release(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		pid, alive := readPid(path)
		if alive {
			return nil, fmt.Errorf("%s", i18n.T("Troopa daemon is already running (pid %d)", pid))
		}
		_ = os.Remove(path)
	}
	return nil, fmt.Errorf("%s", i18n.T("Troopa daemon is already running (pid %d)", os.Getpid()))
}

func Running(dataDir string) (pid int, ok bool) {
	pid, ok = readPid(PidPath(dataDir))
	return pid, ok
}

func readPid(path string) (int, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, false
	}
	return pid, processAlive(pid)
}

func release(path string) error {
	pid, _ := readPid(path)
	if pid != os.Getpid() {
		return nil
	}
	return os.Remove(path)
}
