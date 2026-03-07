// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"log/slog"
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

func discardTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}
