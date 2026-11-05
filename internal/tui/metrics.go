package tui

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jesusuzcategui/troopa/internal/config"
	"github.com/jesusuzcategui/troopa/internal/i18n"
	"github.com/jesusuzcategui/troopa/internal/launch"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func (m Model) openMetrics() (tea.Model, tea.Cmd) {
	m.helpFrom = m.screen
	m.screen = screenMetrics
	m.status = i18n.T("esc back")
	m.err = ""
	return m, nil
}

func (m Model) updateMetricsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "b", "M":
		if m.helpFrom == screenBoard {
			m.screen = screenBoard
			m.status = i18n.T("n new  e edit  [ ] move  m menu  / search  f/p filter  M metrics  esc projects")
		} else {
			m.screen = screenList
			m.status = listStatus()
		}
	case "?":
		m.helpFrom = screenMetrics
		m.screen = screenHelp
	}
	return m, nil
}

func (m Model) viewMetrics() string {
	th := m.theme
	agents, errA := m.store.MetricsByAgent()
	projects, errP := m.store.MetricsByProject()
	var b strings.Builder
	b.WriteString(th.Title.Render(i18n.T("Metrics")))
	b.WriteString(th.Muted.Render(i18n.T("  ·  by agent and by project")))
	b.WriteString("\n\n")
	if errA != nil || errP != nil {
		err := errA
		if err == nil {
			err = errP
		}
		b.WriteString(th.Error.Render(err.Error()))
		return th.App.Render(b.String())
	}
	if len(agents) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("No runs yet. Launch an agent (yolo or pair) and come back here.")))
		b.WriteString("\n\n")
	} else {
		b.WriteString(th.Muted.Render(i18n.T("By agent")))
		b.WriteString("\n")
		b.WriteString(renderMetricTable(agents, m.width))
		b.WriteString("\n")
		b.WriteString(th.Muted.Render(i18n.T("Bars (cost; time when there is no cost)")))
		b.WriteString("\n")
		b.WriteString(renderBars(agents, max(20, m.width-24)))
		b.WriteString("\n")
	}
	b.WriteString(th.Muted.Render(i18n.T("By project")))
	b.WriteString("\n")
	if len(projects) == 0 {
		b.WriteString(th.Muted.Render(i18n.T("n/a")))
		b.WriteString("\n")
	} else {
		b.WriteString(renderMetricTable(projects, m.width))
	}
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("ctx start = input tokens of the first request (n/a if the CLI does not report it)")))
	b.WriteString("\n")
	b.WriteString(th.Muted.Render(i18n.T("esc back")))
	return th.App.Render(b.String())
}

func renderMetricTable(rows []store.MetricRow, width int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %-12s %8s %8s %18s %9s %8s\n", i18n.T("name"), i18n.T("ctx start"), i18n.T("time"), i18n.T("in/out/cache"), i18n.T("cost"), "% ok"))
	for _, r := range rows {
		line := fmt.Sprintf("  %-12s %8s %8s %18s %9s %8s",
			truncate(r.Key, 12),
			fmtAvg(r.AvgContext),
			fmtAvgDuration(r.AvgDurationMS),
			fmtTokens(r.SumIn, r.SumOut, r.SumCache),
			fmtCost(r.SumCost),
			fmtPct(r.Approved, r.Runs),
		)
		b.WriteString(truncate(line, max(1, width-1)))
		b.WriteString("\n")
	}
	return b.String()
}

func renderBars(rows []store.MetricRow, width int) string {
	useCost := false
	maxV := 0.0
	for _, r := range rows {
		if r.SumCost.Valid && r.SumCost.Float64 > 0 {
			useCost = true
		}
	}
	vals := make([]float64, len(rows))
	for i, r := range rows {
		if useCost {
			if r.SumCost.Valid {
				vals[i] = r.SumCost.Float64
			}
		} else if r.AvgDurationMS.Valid {
			vals[i] = r.AvgDurationMS.Float64
		}
		if vals[i] > maxV {
			maxV = vals[i]
		}
	}
	if maxV <= 0 {
		return "  n/d\n"
	}
	barW := max(8, min(32, width))
	var b strings.Builder
	for i, r := range rows {
		n := 0
		if maxV > 0 {
			n = int(math.Round(vals[i] / maxV * float64(barW)))
		}
		if n < 0 {
			n = 0
		}
		if n > barW {
			n = barW
		}
		label := fmtCost(r.SumCost)
		if !useCost {
			label = fmtAvgDuration(r.AvgDurationMS)
		}
		b.WriteString(fmt.Sprintf("  %-10s %s %s\n", truncate(r.Key, 10), strings.Repeat("█", n)+strings.Repeat("░", barW-n), label))
	}
	return b.String()
}

func fmtAvg(n sql.NullFloat64) string {
	if !n.Valid {
		return i18n.T("n/a")
	}
	return fmtCount(int64(math.Round(n.Float64)))
}

func fmtAvgDuration(n sql.NullFloat64) string {
	if !n.Valid {
		return i18n.T("n/a")
	}
	return fmtDuration(int64(math.Round(n.Float64)))
}

func fmtTokens(in, out, cache sql.NullInt64) string {
	return fmt.Sprintf("%s/%s/%s", fmtNullInt(in), fmtNullInt(out), fmtNullInt(cache))
}

func fmtNullInt(n sql.NullInt64) string {
	if !n.Valid {
		return i18n.T("n/a")
	}
	return fmtCount(n.Int64)
}

func fmtCost(n sql.NullFloat64) string {
	if !n.Valid {
		return i18n.T("n/a")
	}
	return fmt.Sprintf("$%.4f", n.Float64)
}

func fmtPct(approved, runs int) string {
	if runs == 0 {
		return i18n.T("n/a")
	}
	return fmt.Sprintf("%d%%", approved*100/runs)
}

func fmtCount(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 10_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func fmtDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	sec := float64(ms) / 1000
	if sec < 60 {
		return fmt.Sprintf("%.1fs", sec)
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

func applyRunMetrics(run *store.Run, logPath, agent string, price config.TokenPrice) {
	if run.Agent == "" {
		run.Agent = agent
	}
	if !run.CreatedAt.IsZero() {
		ms := time.Since(run.CreatedAt).Milliseconds()
		if ms < 0 {
			ms = 0
		}
		run.DurationMS = sql.NullInt64{Int64: ms, Valid: true}
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		raw = nil
	}
	u := launch.ParseUsage(string(raw))
	run.TokensIn = nullI64(u.Input)
	run.TokensOut = nullI64(u.Output)
	run.TokensCache = nullI64(u.Cache)
	run.TokensContext = nullI64(u.InitialContext)
	if cost, ok := launch.EstimateCost(u, price); ok {
		run.CostUSD = sql.NullFloat64{Float64: cost, Valid: true}
	}
}

func nullI64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}
