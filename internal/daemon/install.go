package daemon

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/jesusuzcategui/troopa/internal/i18n"
)

const (
	linuxUnitName = "troopa.service"
	darwinLabel   = "dev.troopa"
	darwinPlist   = "dev.troopa.plist"
)

// Install writes a systemd user unit or a launchd plist. enable runs
// systemctl or launchctl; tests pass false so they never call them.
func Install(w io.Writer, enable bool) error {
	return install(runtime.GOOS, w, enable)
}

// Uninstall removes the unit or plist. enable runs the matching disable command.
func Uninstall(w io.Writer, enable bool) error {
	return uninstall(runtime.GOOS, w, enable)
}

func install(goos string, w io.Writer, enable bool) error {
	if goos == "windows" {
		fmt.Fprintln(w, i18n.T("The Troopa daemon is not supported on Windows yet."))
		return fmt.Errorf("%s", i18n.T("The Troopa daemon is not supported on Windows yet."))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path, contents, err := unitFile(goos, home, exe)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(w, i18n.T("Wrote %s", path))
	if enable {
		if err := enableService(goos, path); err != nil {
			fmt.Fprintln(w, i18n.T("Could not enable the service: %s", err.Error()))
			printCheck(w, goos, path)
			return err
		}
		fmt.Fprintln(w, i18n.T("Service enabled"))
	}
	printCheck(w, goos, path)
	return nil
}

func uninstall(goos string, w io.Writer, enable bool) error {
	if goos == "windows" {
		fmt.Fprintln(w, i18n.T("The Troopa daemon is not supported on Windows yet."))
		return fmt.Errorf("%s", i18n.T("The Troopa daemon is not supported on Windows yet."))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path, err := unitPath(goos, home)
	if err != nil {
		return err
	}
	if enable {
		_ = disableService(goos, path)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintln(w, i18n.T("Removed %s", path))
	printUnload(w, goos, path)
	return nil
}

func unitPath(goos, home string) (string, error) {
	switch goos {
	case "linux":
		return filepath.Join(home, ".config", "systemd", "user", linuxUnitName), nil
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", darwinPlist), nil
	default:
		return "", fmt.Errorf("%s", i18n.T("The Troopa daemon is not supported on Windows yet."))
	}
}

func unitFile(goos, home, exe string) (string, string, error) {
	path, err := unitPath(goos, home)
	if err != nil {
		return "", "", err
	}
	switch goos {
	case "linux":
		return path, linuxUnit(exe), nil
	case "darwin":
		return path, darwinUnit(exe), nil
	default:
		return "", "", fmt.Errorf("%s", i18n.T("The Troopa daemon is not supported on Windows yet."))
	}
}

func linuxUnit(exe string) string {
	return `[Unit]
Description=Troopa daemon
After=default.target

[Service]
Type=simple
ExecStart=` + exe + ` daemon
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`
}

func darwinUnit(exe string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>` + darwinLabel + `</string>
  <key>ProgramArguments</key>
  <array>
    <string>` + exe + `</string>
    <string>daemon</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
</dict>
</plist>
`
}

func enableService(goos, unitPath string) error {
	switch goos {
	case "linux":
		if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
			return err
		}
		return exec.Command("systemctl", "--user", "enable", "--now", "troopa").Run()
	case "darwin":
		return exec.Command("launchctl", "load", unitPath).Run()
	default:
		return nil
	}
}

func disableService(goos, unitPath string) error {
	switch goos {
	case "linux":
		_ = exec.Command("systemctl", "--user", "disable", "--now", "troopa").Run()
		return exec.Command("systemctl", "--user", "daemon-reload").Run()
	case "darwin":
		return exec.Command("launchctl", "unload", unitPath).Run()
	default:
		return nil
	}
}

func printCheck(w io.Writer, goos, path string) {
	switch goos {
	case "linux":
		fmt.Fprintln(w, i18n.T("Enable and start with: systemctl --user enable --now troopa"))
		fmt.Fprintln(w, i18n.T("Check with: systemctl --user status troopa"))
	case "darwin":
		fmt.Fprintln(w, i18n.T("Load with: launchctl load %s", path))
		fmt.Fprintln(w, i18n.T("Check with: launchctl list %s", darwinLabel))
	}
}

func printUnload(w io.Writer, goos, path string) {
	switch goos {
	case "linux":
		fmt.Fprintln(w, i18n.T("Disable with: systemctl --user disable --now troopa"))
	case "darwin":
		fmt.Fprintln(w, i18n.T("Unload with: launchctl unload %s", path))
	}
}
