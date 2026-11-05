package launch

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/jesusuzcategui/troopa/internal/config"
)

type Usage struct {
	Input          *int64
	Output         *int64
	Cache          *int64
	InitialContext *int64
	Turns          int
}

type usageHit struct {
	Input  *int64
	Output *int64
	Cache  *int64
	Kind   string
	Turns  int
}

func ParseUsage(log string) Usage {
	hits := usageFromJSON(log)
	u := Usage{}
	var requestHits []usageHit
	var resultHits []usageHit
	for _, h := range hits {
		if h.Turns > u.Turns {
			u.Turns = h.Turns
		}
		switch h.Kind {
		case "result":
			resultHits = append(resultHits, h)
		default:
			requestHits = append(requestHits, h)
		}
	}
	if len(requestHits) > 0 {
		sumHits(requestHits, &u)
		if requestHits[0].Input != nil {
			v := *requestHits[0].Input
			u.InitialContext = &v
		}
	} else if len(resultHits) == 1 {
		sumHits(resultHits, &u)
		if resultHits[0].Turns <= 1 && resultHits[0].Input != nil {
			v := *resultHits[0].Input
			u.InitialContext = &v
		}
	}
	if u.Input == nil && u.Output == nil && u.Cache == nil {
		textUsage(log, &u)
	}
	return u
}

func EstimateCost(u Usage, p config.TokenPrice) (float64, bool) {
	if u.Input == nil && u.Output == nil && u.Cache == nil {
		return 0, false
	}
	var in, out, cache float64
	if u.Input != nil {
		in = float64(*u.Input)
	}
	if u.Output != nil {
		out = float64(*u.Output)
	}
	if u.Cache != nil {
		cache = float64(*u.Cache)
	}
	return (in*p.Input + out*p.Output + cache*p.Cache) / 1_000_000, true
}

func sumHits(hits []usageHit, u *Usage) {
	var in, out, cache int64
	var hasIn, hasOut, hasCache bool
	for _, h := range hits {
		if h.Input != nil {
			in += *h.Input
			hasIn = true
		}
		if h.Output != nil {
			out += *h.Output
			hasOut = true
		}
		if h.Cache != nil {
			cache += *h.Cache
			hasCache = true
		}
	}
	if hasIn {
		u.Input = &in
	}
	if hasOut {
		u.Output = &out
	}
	if hasCache {
		u.Cache = &cache
	}
}

func usageFromJSON(log string) []usageHit {
	var hits []usageHit
	for _, v := range jsonBlobs(log) {
		collectUsage(v, "", &hits)
	}
	return hits
}

func jsonBlobs(s string) []any {
	var blobs []any
	i := 0
	for i < len(s) {
		if s[i] != '{' && s[i] != '[' {
			i++
			continue
		}
		dec := json.NewDecoder(strings.NewReader(s[i:]))
		var v any
		if err := dec.Decode(&v); err != nil {
			i++
			continue
		}
		blobs = append(blobs, v)
		n := int(dec.InputOffset())
		if n <= 0 {
			i++
			continue
		}
		i += n
	}
	return blobs
}

func collectUsage(v any, parentKind string, hits *[]usageHit) {
	m, ok := v.(map[string]any)
	if !ok {
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				collectUsage(item, parentKind, hits)
			}
		}
		return
	}
	kind := parentKind
	if t, ok := asString(m["type"]); ok {
		kind = t
	}
	if t, ok := asString(m["subtype"]); ok && kind == "" {
		kind = t
	}
	turns := 0
	if n, ok := intFromMap(m, "num_turns"); ok {
		turns = int(n)
	}
	if h, ok := hitFromMap(m, kind, turns); ok {
		*hits = append(*hits, h)
	}
	for _, key := range []string{"usage", "token_usage", "tokenUsage", "tokens"} {
		if child, ok := m[key].(map[string]any); ok {
			if h, ok := hitFromMap(child, kind, turns); ok {
				*hits = append(*hits, h)
			}
		}
	}
	if msg, ok := m["message"].(map[string]any); ok {
		if usage, ok := msg["usage"].(map[string]any); ok {
			if h, ok := hitFromMap(usage, kind, turns); ok {
				*hits = append(*hits, h)
			}
		}
	}
	if mu, ok := m["modelUsage"].(map[string]any); ok {
		for _, child := range mu {
			if cm, ok := child.(map[string]any); ok {
				if h, ok := hitFromMap(cm, kind, turns); ok {
					*hits = append(*hits, h)
				}
			}
		}
	}
}

func hitFromMap(m map[string]any, kind string, turns int) (usageHit, bool) {
	in := firstInt(m, "input_tokens", "inputTokens", "prompt_tokens", "promptTokens", "input")
	out := firstInt(m, "output_tokens", "outputTokens", "completion_tokens", "completionTokens", "output")
	var cache *int64
	for _, key := range []string{
		"cache_read_input_tokens", "cache_creation_input_tokens",
		"cacheReadInputTokens", "cacheCreationInputTokens",
		"cache_read_tokens", "cache_creation_tokens", "cache",
	} {
		if n, ok := intFromMap(m, key); ok {
			if cache == nil {
				z := int64(0)
				cache = &z
			}
			*cache += n
		}
	}
	if in == nil && out == nil && cache == nil {
		return usageHit{}, false
	}
	if turns == 0 {
		if n, ok := intFromMap(m, "num_turns"); ok {
			turns = int(n)
		}
	}
	return usageHit{Input: in, Output: out, Cache: cache, Kind: kind, Turns: turns}, true
}

func firstInt(m map[string]any, keys ...string) *int64 {
	for _, k := range keys {
		if n, ok := intFromMap(m, k); ok {
			v := n
			return &v
		}
	}
	return nil
}

func intFromMap(m map[string]any, key string) (int64, bool) {
	return asInt(m[key])
}

func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case int:
		return int64(n), true
	case int64:
		return n, true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

var (
	reInput  = regexp.MustCompile(`(?i)(?:input|prompt)[_\s-]*tokens?[:\s]+([0-9]+)`)
	reOutput = regexp.MustCompile(`(?i)(?:output|completion)[_\s-]*tokens?[:\s]+([0-9]+)`)
	reCache  = regexp.MustCompile(`(?i)cache[_\s-]*(?:read[_\s-]*)?tokens?[:\s]+([0-9]+)`)
)

func textUsage(log string, u *Usage) {
	if m := reInput.FindStringSubmatch(log); len(m) == 2 {
		if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			u.Input = &n
			u.InitialContext = &n
		}
	}
	if m := reOutput.FindStringSubmatch(log); len(m) == 2 {
		if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			u.Output = &n
		}
	}
	if m := reCache.FindStringSubmatch(log); len(m) == 2 {
		if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			u.Cache = &n
		}
	}
	if u.Turns > 1 {
		u.InitialContext = nil
	}
}
