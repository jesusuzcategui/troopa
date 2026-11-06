// Package i18n translates Troopa's interface. English strings are the keys;
// other languages map them to their translation. A missing translation falls
// back to English.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

// Supported languages.
const (
	EN = "en"
	ES = "es"
)

var (
	lang     = initial()
	catalogs = map[string]map[string]string{ES: es}
)

// Detect picks the language from TROOPA_LANG, then LC_ALL, LC_MESSAGES and LANG.
// Anything that is not Spanish falls back to English.
func Detect() string {
	for _, key := range []string{"TROOPA_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if v == "" || v == "c" || v == "posix" {
			continue
		}
		if strings.HasPrefix(v, ES) {
			return ES
		}
		return EN
	}
	return EN
}

// initial is the language before the config is read: TROOPA_LANG, else English.
func initial() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("TROOPA_LANG"))); strings.HasPrefix(v, ES) {
		return ES
	}
	return EN
}

// Apply sets the language from the config value. TROOPA_LANG always wins;
// "auto" follows the system locale; anything else but "es" means English.
func Apply(pref string) {
	if v := strings.TrimSpace(os.Getenv("TROOPA_LANG")); v != "" {
		SetLang(initial())
		return
	}
	switch strings.ToLower(strings.TrimSpace(pref)) {
	case ES:
		SetLang(ES)
	case "auto":
		SetLang(Detect())
	default:
		SetLang(EN)
	}
}

// SetLang switches the active language. Unknown languages mean English.
func SetLang(l string) {
	if _, ok := catalogs[l]; ok {
		lang = l
		return
	}
	lang = EN
}

// Lang returns the active language.
func Lang() string { return lang }

// T translates msg. With args it formats the translation like fmt.Sprintf.
func T(msg string, args ...any) string {
	out := msg
	if c, ok := catalogs[lang]; ok {
		if tr, ok := c[msg]; ok && tr != "" {
			out = tr
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(out, args...)
	}
	return out
}
