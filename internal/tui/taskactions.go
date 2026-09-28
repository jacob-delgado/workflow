// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// tasksKeys offers refreshing while there is a Taskwarrior to ask. Moving
// through the list is a global affordance, shown in the help rather than the
// footer, as the other list panes have it.
func (m Model) tasksKeys() []key.Binding {
	if m.deps.Tasks.Install == nil {
		return nil
	}

	return []key.Binding{m.keys.refresh}
}

// handleTasksKey answers the Tasks pane's own keys.
func (m Model) handleTasksKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.up, m.keys.down):
		return m.moveTaskSelection(msg), nil
	case key.Matches(msg, m.keys.refresh):
		return m, m.loadTasks()
	}

	return m, nil
}
