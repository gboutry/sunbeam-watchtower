// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	"github.com/gboutry/sunbeam-watchtower/pkg/client"
)

func TestOptionsFrontendCachesPerClientAndApp(t *testing.T) {
	opts := &Options{
		Client: client.NewClient("http://example.invalid"),
	}

	first := opts.Frontend()
	second := opts.Frontend()
	if first != second {
		t.Fatal("Frontend() should cache the facade when client/app are unchanged")
	}

	opts.Client = client.NewClient("http://example-2.invalid")
	third := opts.Frontend()
	if third == second {
		t.Fatal("Frontend() should rebuild the facade when the client changes")
	}

	opts.App = app.NewApp(&config.Config{}, discardTestLogger())
	fourth := opts.Frontend()
	if fourth == third {
		t.Fatal("Frontend() should rebuild the facade when the app changes")
	}
}

func TestNewRootCmd_HelpGroupsCommands(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{
		Out:    &out,
		ErrOut: &out,
	}

	cmd := NewRootCmd(opts)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	help := out.String()
	for _, want := range []string{
		"Workflows",
		"Meta Commands",
		"review",
		"build",
		"releases",
		"auth",
		"cache",
		"serve",
		"server",
		"version",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q:\n%s", want, help)
		}
	}
}

func discardTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}
