package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jesusuzcategui/troopa/internal/cli"
	"github.com/jesusuzcategui/troopa/internal/config"
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

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if err := runTUI(); err != nil {
			fmt.Fprintf(stderr, "troopa: %v\n", err)
			return 1
		}
		return 0
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
func runTUI() error {
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
	return tui.Run(s, cfg, configPath)
}
func writeJSON(w io.Writer, v any) { _ = json.NewEncoder(w).Encode(v) }
func writeError(w io.Writer, code, message string) {
	var r jsonFailure
	r.SchemaVersion = cli.SchemaVersion
	r.Error.Code = code
	r.Error.Message = message
	writeJSON(w, r)
}
