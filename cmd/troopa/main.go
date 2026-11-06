package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/jesusuzcategui/troopa/internal/cli"
	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
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
  troopa <resource> <command> --json  run a non-interactive command
  troopa help | -h | --help           show this help
  troopa version | -v | --version     show the version

Resources: project, client, task, run, brain.
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
func writeJSON(w io.Writer, v any) { _ = json.NewEncoder(w).Encode(v) }
func writeError(w io.Writer, code, message string) {
	var r jsonFailure
	r.SchemaVersion = cli.SchemaVersion
	r.Error.Code = code
	r.Error.Message = message
	writeJSON(w, r)
}
