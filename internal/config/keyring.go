package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

// KeyringMarker in an account env value means the real secret lives in the OS
// keyring (service "troopa", user "account/VAR"). The marker is what stays in
// config.toml; the secret is never written there.
const KeyringMarker = "keyring:"

const keyringService = "troopa"

func keyringUser(account, envVar string) string {
	return account + "/" + envVar
}

// resolveEnvValue expands ~ or, for KeyringMarker, loads the secret from the
// keyring. It never logs the secret.
func resolveEnvValue(account, envVar, value string) (string, error) {
	if value != KeyringMarker {
		return expandHome(value), nil
	}
	secret, err := keyring.Get(keyringService, keyringUser(account, envVar))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("keyring has no secret for account %q variable %s; run: troopa account key set %s %s", account, envVar, account, envVar)
		}
		return "", fmt.Errorf("keyring: %w", err)
	}
	return secret, nil
}

// SetAccountKey stores secret in the OS keyring and sets env VAR = "keyring:"
// on [accounts.account] in config.toml.
func SetAccountKey(configPath, account, envVar, secret string) error {
	account = strings.TrimSpace(account)
	envVar = strings.TrimSpace(envVar)
	if account == "" || envVar == "" {
		return fmt.Errorf("account and variable are required")
	}
	if strings.ContainsAny(envVar, " \t=\"'{}") {
		return fmt.Errorf("invalid env variable %q", envVar)
	}
	if secret == "" {
		return fmt.Errorf("empty key")
	}
	cfg, err := Load(configPath)
	if err != nil {
		return err
	}
	if _, ok := cfg.Launch.Accounts[account]; !ok {
		return fmt.Errorf("account %q is not in config.toml", account)
	}
	if err := keyring.Set(keyringService, keyringUser(account, envVar), secret); err != nil {
		return fmt.Errorf("keyring: %w", err)
	}
	return patchAccountEnv(configPath, account, envVar, KeyringMarker, false)
}

// RemoveAccountKey deletes the secret from the keyring and removes VAR from
// the account's env in config.toml.
func RemoveAccountKey(configPath, account, envVar string) error {
	account = strings.TrimSpace(account)
	envVar = strings.TrimSpace(envVar)
	if account == "" || envVar == "" {
		return fmt.Errorf("account and variable are required")
	}
	cfg, err := Load(configPath)
	if err != nil {
		return err
	}
	if _, ok := cfg.Launch.Accounts[account]; !ok {
		return fmt.Errorf("account %q is not in config.toml", account)
	}
	if err := keyring.Delete(keyringService, keyringUser(account, envVar)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("keyring: %w", err)
	}
	return patchAccountEnv(configPath, account, envVar, "", true)
}

// ReadSecret reads a secret from r. When r is a terminal, input is not echoed.
func ReadSecret(r *os.File) (string, error) {
	if r == nil {
		return "", fmt.Errorf("no input")
	}
	fd := int(r.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func patchAccountEnv(path, account, envVar, value string, remove bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text, err := applyAccountEnvPatch(string(raw), account, envVar, value, remove)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func applyAccountEnvPatch(text, account, envVar, value string, remove bool) (string, error) {
	header := "[accounts." + account + "]"
	lines := strings.Split(text, "\n")
	start := -1
	end := len(lines)
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == header {
			start = i
			continue
		}
		if start >= 0 && i > start && strings.HasPrefix(trimmed, "[") {
			end = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("account %q is not in config.toml", account)
	}

	envLine := -1
	dottedLine := -1
	for i := start + 1; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "env") {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "env"))
			if strings.HasPrefix(rest, "=") {
				envLine = i
				break
			}
			if strings.HasPrefix(rest, ".") {
				name, _, ok := strings.Cut(strings.TrimPrefix(rest, "."), "=")
				if ok && strings.TrimSpace(name) == envVar {
					dottedLine = i
				}
			}
		}
	}

	if envLine >= 0 {
		trimmed := strings.TrimSpace(lines[envLine])
		_, rhs, ok := strings.Cut(trimmed, "=")
		if !ok {
			return "", fmt.Errorf("invalid env line for account %q", account)
		}
		rhs = strings.TrimSpace(rhs)
		m, err := parseInlineEnv(rhs)
		if err != nil {
			return "", err
		}
		if remove {
			delete(m, envVar)
		} else {
			m[envVar] = value
		}
		if len(m) == 0 {
			out := append([]string{}, lines[:envLine]...)
			out = append(out, lines[envLine+1:]...)
			return strings.Join(out, "\n"), nil
		}
		indent := lines[envLine][:len(lines[envLine])-len(strings.TrimLeft(lines[envLine], " \t"))]
		lines[envLine] = indent + "env = " + formatInlineEnv(m)
		return strings.Join(lines, "\n"), nil
	}

	if dottedLine >= 0 {
		if remove {
			out := append([]string{}, lines[:dottedLine]...)
			out = append(out, lines[dottedLine+1:]...)
			return strings.Join(out, "\n"), nil
		}
		indent := lines[dottedLine][:len(lines[dottedLine])-len(strings.TrimLeft(lines[dottedLine], " \t"))]
		lines[dottedLine] = indent + "env." + envVar + " = " + strconv.Quote(value)
		return strings.Join(lines, "\n"), nil
	}

	if remove {
		return text, nil
	}
	line := "env = " + formatInlineEnv(map[string]string{envVar: value})
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:start+1]...)
	out = append(out, line)
	out = append(out, lines[start+1:]...)
	return strings.Join(out, "\n"), nil
}

func parseInlineEnv(rhs string) (map[string]string, error) {
	type wrap struct {
		Env map[string]string `toml:"env"`
	}
	var w wrap
	if err := toml.Unmarshal([]byte("env = "+rhs), &w); err != nil {
		return nil, fmt.Errorf("parse account env: %w", err)
	}
	if w.Env == nil {
		w.Env = map[string]string{}
	}
	return w.Env, nil
}

func formatInlineEnv(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+" = "+strconv.Quote(m[k]))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}
