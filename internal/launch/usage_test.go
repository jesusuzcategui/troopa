package launch

import (
	"testing"

	"github.com/jesusuzcategui/troopa/internal/config"
)

func TestParseUsageStreamJSONFirstRequest(t *testing.T) {
	log := `
{"type":"system","subtype":"init"}
{"type":"assistant","message":{"usage":{"input_tokens":1200,"output_tokens":40,"cache_read_input_tokens":800}}}
{"type":"assistant","message":{"usage":{"input_tokens":90,"output_tokens":20}}}
{"type":"result","num_turns":2,"usage":{"input_tokens":90,"output_tokens":20},"total_cost_usd":0.01}
`
	u := ParseUsage(log)
	if u.InitialContext == nil || *u.InitialContext != 1200 {
		t.Fatalf("context=%v want 1200", ptr(u.InitialContext))
	}
	if u.Input == nil || *u.Input != 1290 {
		t.Fatalf("in=%v want 1290", ptr(u.Input))
	}
	if u.Output == nil || *u.Output != 60 {
		t.Fatalf("out=%v want 60", ptr(u.Output))
	}
	if u.Cache == nil || *u.Cache != 800 {
		t.Fatalf("cache=%v want 800", ptr(u.Cache))
	}
}

func TestParseUsageSingleTurnResultIsContext(t *testing.T) {
	log := `{"type":"result","num_turns":1,"usage":{"input_tokens":50,"output_tokens":10}}`
	u := ParseUsage(log)
	if u.InitialContext == nil || *u.InitialContext != 50 {
		t.Fatalf("context=%v", ptr(u.InitialContext))
	}
}

func TestParseUsageMultiTurnResultHidesContext(t *testing.T) {
	log := `{"type":"result","num_turns":3,"usage":{"input_tokens":50,"output_tokens":10}}`
	u := ParseUsage(log)
	if u.InitialContext != nil {
		t.Fatalf("invented context %d", *u.InitialContext)
	}
	if u.Input == nil || *u.Input != 50 {
		t.Fatalf("in=%v", ptr(u.Input))
	}
}

func TestParseUsageMissingIsEmpty(t *testing.T) {
	u := ParseUsage("working...\ndone")
	if u.Input != nil || u.Output != nil || u.InitialContext != nil {
		t.Fatalf("expected empty usage %+v", u)
	}
}

func TestParseUsageTextFallback(t *testing.T) {
	u := ParseUsage("Input tokens: 321\nOutput tokens: 44\ncache_read_tokens: 12")
	if u.InitialContext == nil || *u.InitialContext != 321 {
		t.Fatalf("context=%v", ptr(u.InitialContext))
	}
	if u.Output == nil || *u.Output != 44 {
		t.Fatalf("out=%v", ptr(u.Output))
	}
}

func TestEstimateCost(t *testing.T) {
	in, out, cache := int64(1_000_000), int64(1_000_000), int64(1_000_000)
	u := Usage{Input: &in, Output: &out, Cache: &cache}
	got, ok := EstimateCost(u, config.TokenPrice{Input: 3, Output: 15, Cache: 0.3})
	if !ok {
		t.Fatal("expected cost")
	}
	if got != 18.3 {
		t.Fatalf("cost=%v want 18.3", got)
	}
	if _, ok := EstimateCost(Usage{}, config.DefaultPrice()); ok {
		t.Fatal("empty usage should be n/d")
	}
}

func ptr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
