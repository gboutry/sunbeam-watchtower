// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	runtimeadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/runtime"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Options controls the TUI process.
type Options struct {
	ConfigPath     string
	ServerAddr     string
	Verbose        bool
	NoColor        bool
	In             io.Reader
	Out            io.Writer
	ErrOut         io.Writer
	ExecutablePath string
}

// Run starts the TUI.
func Run(ctx context.Context, opts Options) error {
	session, err := runtimeadapter.NewSession(ctx, runtimeadapter.Options{
		ConfigPath:     opts.ConfigPath,
		ServerAddr:     opts.ServerAddr,
		Verbose:        opts.Verbose,
		LogWriter:      opts.ErrOut,
		ExecutablePath: opts.ExecutablePath,
		TargetPolicy:   runtimeadapter.TargetPolicyPreferEmbedded,
		AccessMode:     runtimeadapter.AccessModeFull,
	})
	if err != nil {
		return err
	}
	defer session.Close()

	model := newRootModel(session, opts.NoColor)
	programOpts := []tea.ProgramOption{tea.WithAltScreen()}
	if opts.In != nil {
		programOpts = append(programOpts, tea.WithInput(opts.In))
	}
	if opts.Out != nil {
		programOpts = append(programOpts, tea.WithOutput(opts.Out))
	}
	if _, err := tea.NewProgram(model, programOpts...).Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	return nil
}
