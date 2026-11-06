package brain

import (
	"hash/fnv"
	"math"
	"path"
	"strings"
)

// Graph is the brain as nodes (notes) and edges (links between them). Notes
// inside a project folder are also tied to its project.md, so a project and
// its tasks and guides stay together even without explicit links.
type Graph struct {
	Nodes []GraphNode
	Edges []GraphEdge
	index map[string]int
}

type GraphNode struct {
	Rel    string
	Label  string
	Kind   string // inbox, today, index, readme, project, task, guide, note
	Degree int
	X, Y   float64 // layout in [0,1]
}

type GraphEdge struct {
	From, To int
	Folder   bool // implied by the folder, not written as a link
}

const (
	KindInbox   = "inbox"
	KindToday   = "today"
	KindIndex   = "index"
	KindReadme  = "readme"
	KindProject = "project"
	KindTask    = "task"
	KindGuide   = "guide"
	KindNote    = "note"
)

// NoteKind classifies a note by where it lives in the brain.
func NoteKind(rel string) string {
	rel = strings.ToLower(rel)
	parts := strings.Split(rel, "/")
	switch {
	case rel == "inbox.md":
		return KindInbox
	case rel == "today.md":
		return KindToday
	case rel == "readme.md":
		return KindReadme
	case strings.HasSuffix(rel, "index.md"):
		return KindIndex
	case len(parts) == 3 && parts[0] == "projects" && parts[2] == "project.md":
		return KindProject
	case len(parts) >= 4 && parts[0] == "projects" && parts[2] == "tasks":
		return KindTask
	case len(parts) >= 4 && parts[0] == "projects" && parts[2] == "guides":
		return KindGuide
	}
	return KindNote
}

// NoteLabel is a short name for a note: the project slug for project.md,
// otherwise the file name without .md.
func NoteLabel(rel string) string {
	if NoteKind(rel) == KindProject {
		return path.Base(path.Dir(rel))
	}
	return strings.TrimSuffix(path.Base(rel), path.Ext(rel))
}

// BuildGraph reads every note of the brain and its links.
func BuildGraph(dir string) (*Graph, error) {
	notes, err := ListNotes(dir)
	if err != nil {
		return nil, err
	}
	g := &Graph{index: map[string]int{}}
	for _, rel := range notes {
		g.index[rel] = len(g.Nodes)
		g.Nodes = append(g.Nodes, GraphNode{Rel: rel, Label: NoteLabel(rel), Kind: NoteKind(rel)})
	}
	seen := map[[2]int]bool{}
	add := func(a, b int, folder bool) {
		if a == b {
			return
		}
		key := [2]int{min(a, b), max(a, b)}
		if seen[key] {
			return
		}
		seen[key] = true
		g.Edges = append(g.Edges, GraphEdge{From: a, To: b, Folder: folder})
		g.Nodes[a].Degree++
		g.Nodes[b].Degree++
	}
	for i, rel := range notes {
		body, err := ReadNote(dir, rel)
		if err != nil {
			continue
		}
		for _, l := range NoteLinks(dir, rel, body) {
			if j, ok := g.index[l.Rel]; ok {
				add(i, j, false)
			}
		}
	}
	for i, n := range g.Nodes {
		if n.Kind != KindTask && n.Kind != KindGuide {
			continue
		}
		parts := strings.Split(n.Rel, "/")
		if j, ok := g.index[path.Join(parts[0], parts[1], "project.md")]; ok {
			add(i, j, true)
		}
	}
	g.Layout(0)
	return g, nil
}

// Index returns the node of a note, or -1.
func (g *Graph) Index(rel string) int {
	if i, ok := g.index[rel]; ok {
		return i
	}
	return -1
}

// Neighbors lists the nodes linked to i.
func (g *Graph) Neighbors(i int) []int {
	var out []int
	for _, e := range g.Edges {
		switch i {
		case e.From:
			out = append(out, e.To)
		case e.To:
			out = append(out, e.From)
		}
	}
	return out
}

// Layout places the nodes with a force simulation (Fruchterman-Reingold).
// Start positions come from a hash of each path, so the same brain always
// gets the same picture. iterations <= 0 picks a default.
func (g *Graph) Layout(iterations int) {
	n := len(g.Nodes)
	if n == 0 {
		return
	}
	if iterations <= 0 {
		iterations = 300
		if n > 400 {
			iterations = 120
		}
	}
	for i := range g.Nodes {
		h := fnv.New64a()
		_, _ = h.Write([]byte(g.Nodes[i].Rel))
		v := h.Sum64()
		g.Nodes[i].X = 0.1 + 0.8*float64(v&0xffff)/0xffff
		g.Nodes[i].Y = 0.1 + 0.8*float64((v>>16)&0xffff)/0xffff
	}
	if n == 1 {
		g.Nodes[0].X, g.Nodes[0].Y = 0.5, 0.5
		return
	}
	k := math.Sqrt(1.0 / float64(n))
	temp := 0.1
	dx := make([]float64, n)
	dy := make([]float64, n)
	for it := 0; it < iterations; it++ {
		for i := range dx {
			dx[i], dy[i] = 0, 0
		}
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				ddx := g.Nodes[i].X - g.Nodes[j].X
				ddy := g.Nodes[i].Y - g.Nodes[j].Y
				d2 := ddx*ddx + ddy*ddy
				if d2 < 1e-9 {
					ddx, ddy, d2 = 0.001*float64(i-j), 0.001, 1e-6
				}
				f := k * k / d2
				dx[i] += ddx * f
				dy[i] += ddy * f
				dx[j] -= ddx * f
				dy[j] -= ddy * f
			}
		}
		for _, e := range g.Edges {
			ddx := g.Nodes[e.From].X - g.Nodes[e.To].X
			ddy := g.Nodes[e.From].Y - g.Nodes[e.To].Y
			d := math.Sqrt(ddx*ddx+ddy*ddy) + 1e-9
			f := d / k
			dx[e.From] -= ddx * f
			dy[e.From] -= ddy * f
			dx[e.To] += ddx * f
			dy[e.To] += ddy * f
		}
		for i := 0; i < n; i++ {
			// a weak pull to the center keeps lone notes on screen
			dx[i] += (0.5 - g.Nodes[i].X) * 0.05 / k
			dy[i] += (0.5 - g.Nodes[i].Y) * 0.05 / k
			d := math.Sqrt(dx[i]*dx[i]+dy[i]*dy[i]) + 1e-9
			step := math.Min(d, temp)
			g.Nodes[i].X += dx[i] / d * step
			g.Nodes[i].Y += dy[i] / d * step
		}
		temp *= 0.985
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, nd := range g.Nodes {
		minX, maxX = math.Min(minX, nd.X), math.Max(maxX, nd.X)
		minY, maxY = math.Min(minY, nd.Y), math.Max(maxY, nd.Y)
	}
	w, h := math.Max(maxX-minX, 1e-6), math.Max(maxY-minY, 1e-6)
	for i := range g.Nodes {
		g.Nodes[i].X = (g.Nodes[i].X - minX) / w
		g.Nodes[i].Y = (g.Nodes[i].Y - minY) / h
	}
}
