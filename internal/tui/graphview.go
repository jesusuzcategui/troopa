package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/i18n"
)

// The brain graph draws notes as nodes and links as lines on a braille
// canvas: every terminal cell holds 2×4 dots, enough for thin diagonal lines.
// The local view puts one note in the middle with its links around it; the
// full view shows the whole brain laid out by a force simulation.

type graphState struct {
	g      *brain.Graph
	local  bool
	center int // local view: the note in the middle
	depth  int // local view: 1 or 2 rings
	sel    int // selected node (graph index)
	zoom   float64
	panX   float64 // full view: point of the layout at the middle of the screen
	panY   float64
	shown  []shownNode // last frame, for keys and the mouse
	top    int         // first screen row of the canvas (mouse)
	left   int
}

type shownNode struct {
	idx    int
	cx, cy int // cell
	ring   int // local view: 0 center, 1, 2
	angle  float64
}

func (m *Model) openBrainGraph() {
	g, err := brain.BuildGraph(m.brainDir())
	if err != nil {
		m.err = err.Error()
		return
	}
	if len(g.Nodes) == 0 {
		m.status = i18n.T("The second brain has no notes yet")
		return
	}
	center := g.Index(m.brainRel)
	if center < 0 {
		center = g.Index("projects/INDEX.md")
	}
	if center < 0 {
		center = 0
	}
	m.brainGraph = &graphState{g: g, local: true, center: center, sel: center, depth: 1, zoom: 1, panX: 0.5, panY: 0.5}
	m.err = ""
	m.status = graphHelp(true)
}

func graphHelp(local bool) string {
	if local {
		return i18n.T("←/→ pick · enter center / open · 1/2 rings · v whole brain · r reload · esc back")
	}
	return i18n.T("arrows move · enter local view · +/- zoom · 0 fit · v local view · r reload · esc back")
}

func (m Model) updateBrainGraphKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	gs := m.brainGraph
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "G", "backspace":
		m.brainGraph = nil
		m.status = m.brainStatus()
		return m, nil
	case "v":
		gs.local = !gs.local
		if gs.local {
			gs.center = gs.sel
		} else {
			gs.zoom = math.Max(gs.zoom, 1)
			gs.panX, gs.panY = gs.g.Nodes[gs.sel].X, gs.g.Nodes[gs.sel].Y
			if gs.zoom == 1 {
				gs.panX, gs.panY = 0.5, 0.5
			}
		}
		m.status = graphHelp(gs.local)
	case "r":
		rel := gs.g.Nodes[gs.center].Rel
		local := gs.local
		m.openBrainGraph()
		if m.brainGraph != nil {
			if i := m.brainGraph.g.Index(rel); i >= 0 {
				m.brainGraph.center, m.brainGraph.sel = i, i
			}
			m.brainGraph.local = local
		}
	case "1":
		gs.depth = 1
	case "2":
		gs.depth = 2
	case "+", "=":
		if !gs.local {
			gs.zoom = math.Min(gs.zoom*1.5, 12)
			gs.panX, gs.panY = gs.g.Nodes[gs.sel].X, gs.g.Nodes[gs.sel].Y
		}
	case "-":
		if !gs.local {
			gs.zoom = math.Max(gs.zoom/1.5, 1)
			if gs.zoom == 1 {
				gs.panX, gs.panY = 0.5, 0.5
			}
		}
	case "0":
		gs.zoom, gs.panX, gs.panY = 1, 0.5, 0.5
	case "enter", " ":
		return m.graphEnter()
	case "o":
		return m.graphOpenNote(gs.sel)
	case "left", "h", "up", "k", "right", "l", "down", "j", "tab", "shift+tab":
		m.graphMove(msg.String())
	}
	if gs := m.brainGraph; gs != nil {
		m.brainRel = gs.g.Nodes[gs.sel].Rel
	}
	return m, nil
}

// graphEnter recenters the local view on the selected note, or opens it when
// it is already the center. In the full view it switches to the local view.
func (m Model) graphEnter() (tea.Model, tea.Cmd) {
	gs := m.brainGraph
	if !gs.local {
		gs.local = true
		gs.center = gs.sel
		m.status = graphHelp(true)
		return m, nil
	}
	if gs.sel == gs.center {
		return m.graphOpenNote(gs.sel)
	}
	gs.center = gs.sel
	return m, nil
}

func (m Model) graphOpenNote(i int) (tea.Model, tea.Cmd) {
	rel := m.brainGraph.g.Nodes[i].Rel
	m.brainGraph = nil
	m.loadBrainNote(rel, true)
	if !m.brainWide() {
		m.brainNotePane = true
	}
	m.status = m.brainStatus()
	return m, nil
}

// graphMove changes the selection: around the ring in the local view, to the
// nearest node in that direction in the full view.
func (m *Model) graphMove(key string) {
	gs := m.brainGraph
	if len(gs.shown) == 0 {
		return
	}
	if gs.local {
		// center first, then the rings in angle order
		order := make([]int, 0, len(gs.shown))
		for _, s := range gs.shown {
			order = append(order, s.idx)
		}
		pos := 0
		for i, idx := range order {
			if idx == gs.sel {
				pos = i
			}
		}
		step := 1
		switch key {
		case "left", "h", "up", "k", "shift+tab":
			step = -1
		}
		gs.sel = order[(pos+step+len(order))%len(order)]
		return
	}
	var cur shownNode
	found := false
	for _, s := range gs.shown {
		if s.idx == gs.sel {
			cur, found = s, true
		}
	}
	if !found {
		gs.sel = gs.shown[0].idx
		return
	}
	dx, dy := 0, 0
	switch key {
	case "left", "h", "shift+tab":
		dx = -1
	case "right", "l", "tab":
		dx = 1
	case "up", "k":
		dy = -1
	case "down", "j":
		dy = 1
	}
	best, bestScore := -1, math.Inf(1)
	for _, s := range gs.shown {
		if s.idx == gs.sel {
			continue
		}
		ex, ey := float64(s.cx-cur.cx), float64(s.cy-cur.cy)*2 // cells are twice as tall
		along := ex*float64(dx) + ey*float64(dy)
		if along <= 0 {
			continue
		}
		across := math.Abs(ex*float64(dy)) + math.Abs(ey*float64(dx))
		if score := along + 2*across; score < bestScore {
			best, bestScore = s.idx, score
		}
	}
	if best >= 0 {
		gs.sel = best
	}
}

func (m Model) updateBrainGraphMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	gs := m.brainGraph
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if !gs.local {
			return m.updateBrainGraphKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if !gs.local {
			return m.updateBrainGraphKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
		}
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	x, y := msg.X-gs.left, msg.Y-gs.top
	best, bestD := -1, 9.0
	for _, s := range gs.shown {
		d := math.Abs(float64(s.cx-x)) + 2*math.Abs(float64(s.cy-y))
		if d < bestD {
			best, bestD = s.idx, d
		}
	}
	if best < 0 {
		return m, nil
	}
	if best == gs.sel {
		return m.graphEnter()
	}
	gs.sel = best
	m.brainRel = gs.g.Nodes[best].Rel
	return m, nil
}

// --- layout of a frame -----------------------------------------------------

type canvasNode struct {
	idx      int
	px, py   int // pixel (2×4 per cell)
	ring     int
	angle    float64
	priority int // lower gets its label first
}

func (m Model) viewBrainGraph() string {
	th := m.theme
	gs := m.brainGraph
	w := max(20, m.width-4)
	h := max(6, m.height-7)
	title := th.Title.Render(i18n.T("Brain graph"))
	mode := i18n.T("local")
	if !gs.local {
		mode = i18n.T("whole brain")
	}
	sel := gs.g.Nodes[gs.sel]
	title += th.Muted.Render(fmt.Sprintf("  ·  %s  ·  %d %s  ·  %s", mode, len(gs.g.Nodes), i18n.T("notes"), sel.Rel))
	var nodes []canvasNode
	if gs.local {
		nodes = gs.localLayout(w, h)
	} else {
		nodes = gs.globalLayout(w, h)
	}
	canvas, shown := renderGraph(th, gs.g, w, h, nodes, gs.sel, gs.center, gs.local)
	gs.shown = shown
	gs.top = 3 // title, blank line, then the canvas (App padding adds the rest)
	gs.left = 2
	if pad := th.App.GetPaddingTop(); pad > 0 {
		gs.top += pad
	}
	if pad := th.App.GetPaddingLeft(); pad > 0 {
		gs.left = pad
	}
	legend := th.Muted.Render("● " + i18n.T("project") + "  ◆ " + i18n.T("hub") + "  • " + i18n.T("task") + "  ◇ " + i18n.T("guide") + "  ○ " + i18n.T("note") +
		"   " + fmt.Sprintf(i18n.T("%d links"), len(gs.g.Neighbors(gs.sel))))
	return th.App.Render(title + "\n\n" + canvas + "\n" + legend)
}

// localLayout: the center note, its links on a ring around it and, with
// depth 2, their links on an outer ring next to them.
func (gs *graphState) localLayout(w, h int) []canvasNode {
	pw, ph := float64(w*2), float64(h*4)
	cx, cy := pw/2, ph/2
	r := math.Min(pw, ph) / 2
	r1, r2 := r*0.55, r*0.92
	if gs.depth == 1 {
		r1 = r * 0.8
	}
	out := []canvasNode{{idx: gs.center, px: int(cx), py: int(cy), priority: 0}}
	placed := map[int]bool{gs.center: true}
	first := gs.g.Neighbors(gs.center)
	sort.SliceStable(first, func(a, b int) bool {
		na, nb := gs.g.Nodes[first[a]], gs.g.Nodes[first[b]]
		if kindRank(na.Kind) != kindRank(nb.Kind) {
			return kindRank(na.Kind) < kindRank(nb.Kind)
		}
		return na.Label < nb.Label
	})
	var ring1 []int
	for _, j := range first {
		if !placed[j] {
			placed[j] = true
			ring1 = append(ring1, j)
		}
	}
	angleOf := map[int]float64{}
	for k, j := range ring1 {
		a := -math.Pi/2 + 2*math.Pi*float64(k)/float64(max(1, len(ring1)))
		angleOf[j] = a
		out = append(out, canvasNode{idx: j, px: int(cx + r1*math.Cos(a)), py: int(cy + r1*math.Sin(a)), ring: 1, angle: a, priority: 1})
	}
	if gs.depth >= 2 && len(ring1) > 0 {
		span := 2 * math.Pi / float64(len(ring1))
		count := 0
		for _, j := range ring1 {
			var kids []int
			for _, k := range gs.g.Neighbors(j) {
				if !placed[k] {
					placed[k] = true
					kids = append(kids, k)
				}
			}
			for n, k := range kids {
				if count >= 60 {
					break
				}
				a := angleOf[j]
				if len(kids) > 1 {
					a += span * 0.8 * (float64(n)/float64(len(kids)-1) - 0.5)
				}
				out = append(out, canvasNode{idx: k, px: int(cx + r2*math.Cos(a)), py: int(cy + r2*math.Sin(a)), ring: 2, angle: a, priority: 3})
				count++
			}
		}
	}
	return out
}

// globalLayout maps the force layout to the screen with zoom and pan.
func (gs *graphState) globalLayout(w, h int) []canvasNode {
	pw, ph := float64(w*2-4), float64(h*4-4)
	var out []canvasNode
	for i, n := range gs.g.Nodes {
		x := (n.X-gs.panX)*gs.zoom + 0.5
		y := (n.Y-gs.panY)*gs.zoom + 0.5
		if x < 0 || x > 1 || y < 0 || y > 1 {
			continue
		}
		pri := 4
		switch {
		case n.Kind == brain.KindProject || n.Kind == brain.KindIndex:
			pri = 1
		case n.Degree >= 4:
			pri = 2
		}
		out = append(out, canvasNode{idx: i, px: 2 + int(x*pw), py: 2 + int(y*ph), priority: pri})
	}
	return out
}

func kindRank(kind string) int {
	switch kind {
	case brain.KindProject:
		return 0
	case brain.KindIndex, brain.KindInbox, brain.KindToday, brain.KindReadme:
		return 1
	case brain.KindGuide:
		return 2
	case brain.KindTask:
		return 3
	}
	return 4
}

func nodeGlyph(kind string) rune {
	switch kind {
	case brain.KindProject:
		return '●'
	case brain.KindIndex, brain.KindInbox, brain.KindToday, brain.KindReadme:
		return '◆'
	case brain.KindTask:
		return '•'
	case brain.KindGuide:
		return '◇'
	}
	return '○'
}

// --- braille canvas --------------------------------------------------------

const (
	stNone = iota
	stEdge
	stEdgeHot
	stLabel
	stLabelHot
	stProject
	stHub
	stTask
	stGuide
	stNote
	stSelected
)

// brailleBit is the dot of pixel (x%2, y%4) inside a braille cell.
var brailleBit = [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

type graphCanvas struct {
	w, h  int
	dots  []rune // braille bits per cell
	text  []rune // a glyph or label rune overrides the dots
	style []int
}

func newGraphCanvas(w, h int) *graphCanvas {
	return &graphCanvas{w: w, h: h, dots: make([]rune, w*h), text: make([]rune, w*h), style: make([]int, w*h)}
}

func (c *graphCanvas) plot(x, y, st int) {
	if x < 0 || y < 0 || x >= c.w*2 || y >= c.h*4 {
		return
	}
	i := (y/4)*c.w + x/2
	c.dots[i] |= brailleBit[y%4][x%2]
	if c.text[i] == 0 && st > c.style[i] {
		c.style[i] = st
	}
}

// line draws a line with Bresenham's algorithm, leaving a gap near the ends
// so the node glyphs stay readable.
func (c *graphCanvas) line(x0, y0, x1, y1, st int) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	x, y := x0, y0
	for {
		if (abs(x-x0) > 1 || abs(y-y0) > 2) && (abs(x-x1) > 1 || abs(y-y1) > 2) {
			c.plot(x, y, st)
		}
		if x == x1 && y == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
	}
}

func (c *graphCanvas) free(cx, cy, n int) bool {
	if cy < 0 || cy >= c.h || cx < 0 || cx+n > c.w {
		return false
	}
	for k := 0; k < n; k++ {
		if c.text[cy*c.w+cx+k] != 0 {
			return false
		}
	}
	return true
}

func (c *graphCanvas) put(cx, cy int, s string, st int) {
	for k, r := range []rune(s) {
		if cx+k < 0 || cx+k >= c.w || cy < 0 || cy >= c.h {
			continue
		}
		i := cy*c.w + cx + k
		c.text[i] = r
		c.style[i] = st
	}
}

// gap clears the dots of one cell so a label never touches a line.
func (c *graphCanvas) gap(cx, cy int) {
	if cx < 0 || cy < 0 || cx >= c.w || cy >= c.h {
		return
	}
	i := cy*c.w + cx
	if c.text[i] == 0 {
		c.text[i] = ' '
		c.style[i] = stNone
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func renderGraph(th Theme, g *brain.Graph, w, h int, nodes []canvasNode, sel, center int, local bool) (string, []shownNode) {
	c := newGraphCanvas(w, h)
	pos := map[int]canvasNode{}
	for _, n := range nodes {
		pos[n.idx] = n
	}
	// edges first, the ones touching the selection last so they win
	for pass := 0; pass < 2; pass++ {
		for _, e := range g.Edges {
			a, okA := pos[e.From]
			b, okB := pos[e.To]
			if !okA || !okB {
				continue
			}
			hot := e.From == sel || e.To == sel
			if local && !hot && a.ring+b.ring > 2 {
				// in the local view only draw lines that lead somewhere visible
				// from the center or the selection
				if a.idx != center && b.idx != center && a.ring != 1 && b.ring != 1 {
					continue
				}
			}
			if hot != (pass == 1) {
				continue
			}
			st := stEdge
			if hot {
				st = stEdgeHot
			}
			c.line(a.px, a.py, b.px, b.py, st)
		}
	}
	// glyphs
	var shown []shownNode
	for _, n := range nodes {
		cx, cy := min(n.px/2, w-1), min(n.py/4, h-1)
		kind := g.Nodes[n.idx].Kind
		st := stNote
		switch kind {
		case brain.KindProject:
			st = stProject
		case brain.KindIndex, brain.KindInbox, brain.KindToday, brain.KindReadme:
			st = stHub
		case brain.KindTask:
			st = stTask
		case brain.KindGuide:
			st = stGuide
		}
		if n.idx == sel {
			st = stSelected
		}
		c.put(cx, cy, string(nodeGlyph(kind)), st)
		shown = append(shown, shownNode{idx: n.idx, cx: cx, cy: cy, ring: n.ring, angle: n.angle})
	}
	// labels by priority: selection, center, its neighbors, then the rest
	hotSet := map[int]bool{}
	for _, j := range g.Neighbors(sel) {
		hotSet[j] = true
	}
	order := append([]canvasNode(nil), nodes...)
	rank := func(n canvasNode) int {
		switch {
		case n.idx == sel:
			return -3
		case n.idx == center && local:
			return -2
		case hotSet[n.idx]:
			return -1
		}
		return n.priority
	}
	sort.SliceStable(order, func(a, b int) bool {
		ra, rb := rank(order[a]), rank(order[b])
		if ra != rb {
			return ra < rb
		}
		return g.Nodes[order[a].idx].Degree > g.Nodes[order[b].idx].Degree
	})
	for _, n := range order {
		label := g.Nodes[n.idx].Label
		maxLen := 22
		if n.idx == sel {
			maxLen = 40
		}
		label = truncate(label, maxLen)
		lw := lipgloss.Width(label)
		cx, cy := min(n.px/2, w-1), min(n.py/4, h-1)
		st := stLabel
		if n.idx == sel || hotSet[n.idx] || (local && n.idx == center) {
			st = stLabelHot
		}
		if n.idx == sel {
			st = stSelected
		}
		// right of the glyph, else left of it
		leftSide := local && n.ring > 0 && math.Cos(n.angle) < -0.2
		tries := []int{cx + 2, cx - 1 - lw}
		if leftSide {
			tries = []int{cx - 1 - lw, cx + 2}
		}
		for _, x := range tries {
			if c.free(x-1, cy, lw+2) || (x-1 < 0 && c.free(x, cy, lw+1)) {
				c.gap(x-1, cy)
				c.gap(x+lw, cy)
				c.put(x, cy, label, st)
				break
			}
		}
	}
	return c.render(th), shown
}

func (c *graphCanvas) render(th Theme) string {
	fg := func(s lipgloss.Style) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(s.GetForeground()).Background(th.App.GetBackground())
	}
	styles := map[int]lipgloss.Style{
		stNone:     fg(th.Muted),
		stEdge:     fg(th.Muted).Faint(true),
		stEdgeHot:  fg(th.Border),
		stLabel:    fg(th.Muted),
		stLabelHot: fg(th.Normal),
		stProject:  fg(th.Title).Bold(true),
		stHub:      fg(th.Border).Bold(true),
		stTask:     fg(th.Normal),
		stGuide:    fg(th.DiffAdd),
		stNote:     fg(th.Label),
		stSelected: th.Selected.UnsetPadding().UnsetMargins().Bold(true),
	}
	var b strings.Builder
	for y := 0; y < c.h; y++ {
		var run strings.Builder
		cur := -1
		flush := func() {
			if run.Len() > 0 {
				b.WriteString(styles[max(cur, 0)].Render(run.String()))
				run.Reset()
			}
		}
		for x := 0; x < c.w; x++ {
			i := y*c.w + x
			r := c.text[i]
			st := c.style[i]
			if r == 0 {
				if c.dots[i] == 0 {
					r, st = ' ', stNone
				} else {
					r = 0x2800 + c.dots[i]
				}
			}
			if st != cur {
				flush()
				cur = st
			}
			run.WriteRune(r)
		}
		flush()
		if y < c.h-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
