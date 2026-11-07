package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/jesusuzcategui/troopa/internal/cli"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/daemon"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	troopamcp "github.com/jesusuzcategui/troopa/internal/mcp"
	"github.com/jesusuzcategui/troopa/internal/store"
	"github.com/jesusuzcategui/troopa/internal/tui"
)

type jsonSuccess struct {
	SchemaVersion int `json:"schema_version"`
	Data          any `json:"data"`
}
type jsonFailure struct {
	SchemaVersion int `json:"schema_version"`
	Error         struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// version is set at build time (goreleaser: -X main.version=...).
var version = "dev"

const usage = `Troopa: your AI agent troop in the terminal.

Usage:
  troopa                              open the interactive TUI
  troopa setup                        run the first-run setup again
  troopa login <account> <agent>      sign an agent in to an account
  troopa extensions                   list troopa-* extensions on the PATH
  troopa <extension> [args]           run troopa-<extension>
  troopa daemon                       run in the background (no TUI)
  troopa daemon status                show whether it runs and which runs it watches
  troopa daemon install               install a user service (Linux/macOS)
  troopa daemon uninstall             remove the user service
  troopa mcp                          MCP server over stdio (read tools)
  troopa <resource> <command> --json  run a non-interactive command
  troopa help | -h | --help           show this help
  troopa version | -v | --version     show the version

Resources: project, client, task, run, plan, profile, brain.
Examples:
  troopa project list --json
  troopa task list --json --project 1
  troopa run rounds --json --id 1
  troopa task attachments --json --id 1
  troopa brain sync --json

Full command reference: docs/cli-json.md
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func buildVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (len(args) == 1 && args[0] == "setup") {
		if err := runTUI(len(args) == 1); err != nil {
			fmt.Fprintf(stderr, "troopa: %v\n", err)
			return 1
		}
		return 0
	}
	if len(args) == 1 {
		switch args[0] {
		case "help", "-h", "--help":
			fmt.Fprint(stdout, usage)
			return 0
		case "version", "-v", "--version":
			fmt.Fprintf(stdout, "troopa %s\n", buildVersion())
			return 0
		}
	}
	switch args[0] {
	case "login":
		return runLogin(args[1:], stdout, stderr)
	case "daemon":
		return runDaemon(args[1:], stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stdout, stderr)
	case "extensions":
		exts := cli.ListExtensions()
		if len(exts) == 0 {
			fmt.Fprintln(stdout, "No extensions. Put a program named troopa-<name> on your PATH.")
		}
		for _, e := range exts {
			fmt.Fprintln(stdout, e)
		}
		return 0
	}
	if path, ok := cli.FindExtension(args[0]); ok {
		return runExtension(path, args[1:], stderr)
	}
	if err := cli.ValidateJSON(args); err != nil {
		writeError(stdout, "invalid_argument", err.Error())
		return 2
	}
	_, _, dbPath, err := config.Paths()
	if err == nil {
		var s *store.Store
		s, err = store.Open(dbPath)
		if err == nil {
			defer s.Close()
			var result any
			result, err = cli.Execute(s, args)
			if err == nil {
				writeJSON(stdout, jsonSuccess{SchemaVersion: cli.SchemaVersion, Data: result})
				return 0
			}
		}
	}
	code := "internal"
	if e, ok := err.(*cli.Error); ok {
		code = e.Code
	}
	writeError(stdout, code, err.Error())
	if code == "invalid_argument" {
		return 2
	}
	if code == "not_found" {
		return 3
	}
	return 1
}
func applyConfigLang() {
	_, configPath, _, err := config.Paths()
	if err != nil {
		return
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return
	}
	i18n.Apply(cfg.Language)
}

func runDaemon(args []string, stdout, stderr io.Writer) int {
	applyConfigLang()
	if len(args) == 0 {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := daemon.Run(ctx); err != nil {
			fmt.Fprintf(stderr, "troopa: %v\n", err)
			return 1
		}
		return 0
	}
	switch args[0] {
	case "install":
		if err := daemon.Install(stdout, true); err != nil {
			fmt.Fprintf(stderr, "troopa: %v\n", err)
			return 1
		}
		return 0
	case "uninstall":
		if err := daemon.Uninstall(stdout, true); err != nil {
			fmt.Fprintf(stderr, "troopa: %v\n", err)
			return 1
		}
		return 0
	case "status":
		if err := daemon.Status(stdout); err != nil {
			fmt.Fprintf(stderr, "troopa: %v\n", err)
			return 1
		}
		return 0
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, i18n.T("usage: troopa daemon [install | uninstall | status]"))
		return 0
	default:
		fmt.Fprintln(stderr, i18n.T("usage: troopa daemon [install | uninstall | status]"))
		return 2
	}
}

func runMCP(args []string, stdout, stderr io.Writer) int {
	applyConfigLang()
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			fmt.Fprintln(stdout, i18n.T("usage: troopa mcp"))
			return 0
		default:
			fmt.Fprintln(stderr, i18n.T("usage: troopa mcp"))
			return 2
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := troopamcp.Run(ctx); err != nil {
		fmt.Fprintf(stderr, "troopa: %v\n", err)
		return 1
	}
	return 0
}

func runTUI(forceSetup bool) error {
	_, configPath, dbPath, err := config.Paths()
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	i18n.Apply(cfg.Language)
	return tui.Run(s, cfg, configPath, tui.Options{Version: buildVersion(), ForceSetup: forceSetup})
}

// runLogin opens an agent CLI with the logins of an account
// ([accounts.<name>] in config.toml) so the person signs in once.
func runLogin(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: troopa login <account> <agent>")
		return 2
	}
	_, configPath, _, err := config.Paths()
	if err != nil {
		fmt.Fprintf(stderr, "troopa: %v\n", err)
		return 1
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "troopa: %v\n", err)
		return 1
	}
	cmd, err := launch.LoginCommand(cfg.Launch, configPath, args[0], args[1])
	if err != nil {
		fmt.Fprintf(stderr, "troopa: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Opening %s with the %q account. Sign in, then exit the agent.\n", args[1], args[0])
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "troopa: %v\n", err)
		return 1
	}
	return 0
}

// runExtension runs troopa-<name> with the paths Troopa uses.
func runExtension(path string, args []string, stderr io.Writer) int {
	dataDir, configPath, dbPath, _ := config.Paths()
	self, _ := os.Executable()
	cmd := exec.Command(path, args...)
	cmd.Env = append(os.Environ(),
		"TROOPA_BIN="+self,
		"TROOPA_VERSION="+buildVersion(),
		"TROOPA_CONFIG="+configPath,
		"TROOPA_DATA="+dataDir,
		"TROOPA_DB="+dbPath,
	)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "troopa: %v\n", err)
		return 1
	}
	return 0
}

func writeJSON(w io.Writer, v any) { _ = json.NewEncoder(w).Encode(v) }
func writeError(w io.Writer, code, message string) {
	var r jsonFailure
	r.SchemaVersion = cli.SchemaVersion
	r.Error.Code = code
	r.Error.Message = message
	writeJSON(w, r)
}
