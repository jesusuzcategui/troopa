package tui

import (
	"github.com/jesusuzcategui/troopa/internal/brain"
	"github.com/jesusuzcategui/troopa/internal/store"
)

func (m Model) brainSettings() brain.Settings {
	return brain.FromConfig(m.cfg)
}

func (m *Model) rememberProject(p store.Project) {
	projects, err := m.store.List(true)
	if err != nil {
		return
	}
	_ = brain.OnProject(m.brainSettings(), projects, p)
}

func (m *Model) rememberTask(t store.Task) {
	p, err := m.store.Get(t.ProjectID)
	if err != nil {
		if m.boardProject.ID != 0 {
			p = m.boardProject
		} else {
			return
		}
	}
	_ = brain.OnTask(m.brainSettings(), p, t)
}

func (m *Model) rememberTaskID(id int64) {
	t, err := m.store.GetTask(id)
	if err != nil {
		return
	}
	m.rememberTask(t)
}

func (m *Model) rememberApprove(p store.Project, t store.Task, run store.Run) {
	if fresh, err := m.store.GetTask(t.ID); err == nil {
		t = fresh
	}
	_ = brain.OnApprove(m.brainSettings(), p, t, run)
}

func (m *Model) rememberMerge(p store.Project, t store.Task, run store.Run) {
	if fresh, err := m.store.GetTask(t.ID); err == nil {
		t = fresh
	}
	_ = brain.OnMerge(m.brainSettings(), p, t, run)
}

func (m *Model) moveTask(id int64, status string) error {
	if err := m.store.MoveTask(id, status); err != nil {
		return err
	}
	m.rememberTaskID(id)
	return nil
}

func (m *Model) ensureBrain() {
	_ = brain.Ensure(m.brainSettings().Dir)
}
