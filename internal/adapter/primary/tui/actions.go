// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	runtimeadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/runtime"
)

type actionDeniedMsg struct {
	err error
}

func guardSessionAction(session *runtimeadapter.Session, actionID frontend.ActionID, run func() tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if err := runtimeadapter.CheckActionAccess(session.AccessMode(), actionID, false); err != nil {
			return actionDeniedMsg{err: err}
		}
		return run()
	}
}
