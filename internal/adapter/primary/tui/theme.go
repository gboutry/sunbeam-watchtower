// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package tui

import "github.com/charmbracelet/lipgloss"

type theme struct {
	header       lipgloss.Style
	subtle       lipgloss.Style
	key          lipgloss.Style
	tab          lipgloss.Style
	tabActive    lipgloss.Style
	success      lipgloss.Style
	pending      lipgloss.Style
	errorText    lipgloss.Style
	info         lipgloss.Style
	project      lipgloss.Style
	metadata     lipgloss.Style
	panel        lipgloss.Style
	panelTitle   lipgloss.Style
	selectedRow  lipgloss.Style
	statusBar    lipgloss.Style
	statusLeft   lipgloss.Style
	statusRight  lipgloss.Style
	badge        lipgloss.Style
	input        lipgloss.Style
	inputFocused lipgloss.Style
}

func newTheme() theme {
	return theme{
		header:       lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Bold(true),
		subtle:       lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")),
		key:          lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC")).Bold(true),
		tab:          lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Padding(0, 1),
		tabActive:    lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Background(lipgloss.Color("#334155")).Bold(true).Padding(0, 1),
		success:      lipgloss.NewStyle().Foreground(lipgloss.Color("#86EFAC")).Bold(true),
		pending:      lipgloss.NewStyle().Foreground(lipgloss.Color("#C4B5FD")).Bold(true),
		errorText:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5")).Bold(true),
		info:         lipgloss.NewStyle().Foreground(lipgloss.Color("#7DD3FC")),
		project:      lipgloss.NewStyle().Foreground(lipgloss.Color("#99F6E4")),
		metadata:     lipgloss.NewStyle().Foreground(lipgloss.Color("#A5B4FC")),
		panel:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#334155")).Padding(0, 1),
		panelTitle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Bold(true),
		selectedRow:  lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Background(lipgloss.Color("#1E293B")),
		statusBar:    lipgloss.NewStyle().Background(lipgloss.Color("#0F172A")).Foreground(lipgloss.Color("#CBD5E1")).Padding(0, 1),
		statusLeft:   lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")),
		statusRight:  lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")),
		badge:        lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Background(lipgloss.Color("#334155")).Padding(0, 1),
		input:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#334155")).Padding(0, 1),
		inputFocused: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#7DD3FC")).Padding(0, 1),
	}
}

func (t theme) semantic(text string) string {
	switch text {
	case "authenticated", "succeeded", "running local", "remote":
		return t.success.Render(text)
	case "queued", "running", "building", "embedded", "daemon":
		return t.pending.Render(text)
	case "failed", "cancelled", "error", "not authenticated":
		return t.errorText.Render(text)
	default:
		return t.info.Render(text)
	}
}
