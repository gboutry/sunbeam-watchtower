// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/api"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
)

func TestRuntimeHelperProcess(t *testing.T) {
	if os.Getenv("WATCHTOWER_RUNTIME_HELPER_PROCESS") != "1" {
		return
	}

	args := helperProcessArgs(os.Args)
	listen := "127.0.0.1:8472"
	for i := 0; i < len(args); i++ {
		if args[i] == "--listen" && i+1 < len(args) {
			listen = args[i+1]
			i++
		}
	}

	var serverOpts api.ServerOptions
	switch {
	case strings.HasPrefix(listen, "unix://"):
		serverOpts.UnixSocket = strings.TrimPrefix(listen, "unix://")
	case strings.HasPrefix(listen, "tcp://"):
		serverOpts.ListenAddr = strings.TrimPrefix(listen, "tcp://")
	default:
		serverOpts.ListenAddr = listen
	}

	application := app.NewAppWithOptions(&config.Config{}, NewLogger(false, os.Stderr), app.Options{
		RuntimeMode: app.RuntimeModePersistent,
	})
	srv := NewConfiguredServer(application.Logger, application, serverOpts)
	if err := srv.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	select {}
}

func TestNewSession_DefaultsToRemoteWhenServerAddrProvided(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	session, err := NewSession(context.Background(), Options{
		ServerAddr:   "http://127.0.0.1:9999",
		LogWriter:    &bytes.Buffer{},
		TargetPolicy: TargetPolicyPreferExistingDaemon,
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer session.Close()

	if got := session.Target().Kind; got != TargetKindRemote {
		t.Fatalf("Target().Kind = %q, want %q", got, TargetKindRemote)
	}
	if got := session.Target().Address; got != "http://127.0.0.1:9999" {
		t.Fatalf("Target().Address = %q", got)
	}
}

func TestNewSession_PreferEmbeddedFallsBackToEmbedded(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	session, err := NewSession(context.Background(), Options{
		LogWriter:    &bytes.Buffer{},
		TargetPolicy: TargetPolicyPreferEmbedded,
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer session.Close()

	if got := session.Target().Kind; got != TargetKindEmbedded {
		t.Fatalf("Target().Kind = %q, want %q", got, TargetKindEmbedded)
	}
	if session.Client == nil {
		t.Fatal("session.Client = nil")
	}
	if session.Frontend == nil {
		t.Fatal("session.Frontend = nil")
	}
}

func TestNewSession_PreferExistingDaemonReusesDaemon(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	manager, err := NewLocalServerManager(Options{LogWriter: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("NewLocalServerManager() error = %v", err)
	}
	if err := os.MkdirAll(manager.Paths().Dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	srv := api.NewServer(NewLogger(false, &bytes.Buffer{}), api.ServerOptions{UnixSocket: manager.Paths().Socket})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer srv.Shutdown(context.Background())

	if err := os.WriteFile(manager.Paths().PIDFile, []byte("1234"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := WriteLocalServerMetadata(manager.Paths().Metadata, LocalServerMetadata{
		PID:     1234,
		Address: manager.Paths().SocketURI,
		LogFile: manager.Paths().LogFile,
	}); err != nil {
		t.Fatalf("WriteLocalServerMetadata() error = %v", err)
	}

	session, err := NewSession(context.Background(), Options{
		LogWriter:    &bytes.Buffer{},
		TargetPolicy: TargetPolicyPreferExistingDaemon,
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer session.Close()

	if got := session.Target().Kind; got != TargetKindDaemon {
		t.Fatalf("Target().Kind = %q, want %q", got, TargetKindDaemon)
	}
	if got := session.Target().Address; got != manager.Paths().SocketURI {
		t.Fatalf("Target().Address = %q, want %q", got, manager.Paths().SocketURI)
	}
}

func TestNewSession_RequirePersistentStartsDaemon(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	wrapper := writeRuntimeHelperWrapper(t)
	manager, err := NewLocalServerManager(Options{
		LogWriter:      &bytes.Buffer{},
		ExecutablePath: wrapper,
	})
	if err != nil {
		t.Fatalf("NewLocalServerManager() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = manager.Stop(context.Background())
	})

	session, err := NewSession(context.Background(), Options{
		LogWriter:      &bytes.Buffer{},
		ExecutablePath: wrapper,
		TargetPolicy:   TargetPolicyRequirePersistent,
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer session.Close()

	if got := session.Target().Kind; got != TargetKindDaemon {
		t.Fatalf("Target().Kind = %q, want %q", got, TargetKindDaemon)
	}

	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Running {
		t.Fatalf("Status() = %+v, want running daemon", status)
	}
}

func helperProcessArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return nil
}

func writeRuntimeHelperWrapper(t *testing.T) string {
	t.Helper()

	testBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	wrapper := filepath.Join(t.TempDir(), "watchtower-runtime-helper")
	content := fmt.Sprintf(
		"#!/bin/sh\nexport WATCHTOWER_RUNTIME_HELPER_PROCESS=1\nexec %q -test.run=TestRuntimeHelperProcess -- \"$@\"\n",
		testBinary,
	)
	if err := os.WriteFile(wrapper, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", wrapper, err)
	}
	return wrapper
}
