package cli

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/api"
	"github.com/spf13/cobra"
)

func TestCommandNeedsPersistentServer(t *testing.T) {
	t.Run("auth commands require persistent server", func(t *testing.T) {
		auth := &cobra.Command{Use: "auth"}
		cmd := &cobra.Command{Use: "status"}
		cmd.SetArgs(nil)
		auth.AddCommand(cmd)

		if !commandNeedsPersistentServer(cmd) {
			t.Fatal("auth status should require persistent server")
		}
	})

	t.Run("build trigger only requires persistent server when async", func(t *testing.T) {
		build := &cobra.Command{Use: "build"}
		cmd := &cobra.Command{Use: "trigger"}
		cmd.Flags().Bool("async", false, "")
		build.AddCommand(cmd)

		if commandNeedsPersistentServer(cmd) {
			t.Fatal("sync build trigger should not require persistent server")
		}
		if err := cmd.Flags().Set("async", "true"); err != nil {
			t.Fatalf("Set(async) error = %v", err)
		}
		if !commandNeedsPersistentServer(cmd) {
			t.Fatal("async build trigger should require persistent server")
		}
	})

	t.Run("project sync only requires persistent server when async", func(t *testing.T) {
		project := &cobra.Command{Use: "project"}
		cmd := &cobra.Command{Use: "sync"}
		cmd.Flags().Bool("async", false, "")
		project.AddCommand(cmd)

		if commandNeedsPersistentServer(cmd) {
			t.Fatal("sync project command should not require persistent server")
		}
		if err := cmd.Flags().Set("async", "true"); err != nil {
			t.Fatalf("Set(async) error = %v", err)
		}
		if !commandNeedsPersistentServer(cmd) {
			t.Fatal("async project sync should require persistent server")
		}
	})
}

func TestClientTargetModeForCommand(t *testing.T) {
	auth := &cobra.Command{Use: "auth"}
	authStatus := &cobra.Command{Use: "status"}
	auth.AddCommand(authStatus)

	review := &cobra.Command{Use: "review"}
	reviewList := &cobra.Command{Use: "list"}
	review.AddCommand(reviewList)

	if got := clientTargetModeForCommand(authStatus, "http://example", false); got != clientTargetExplicit {
		t.Fatalf("explicit target mode = %v, want %v", got, clientTargetExplicit)
	}
	if got := clientTargetModeForCommand(reviewList, "", true); got != clientTargetDaemon {
		t.Fatalf("daemon target mode = %v, want %v", got, clientTargetDaemon)
	}
	if got := clientTargetModeForCommand(authStatus, "", false); got != clientTargetEnsureDaemon {
		t.Fatalf("auth target mode = %v, want %v", got, clientTargetEnsureDaemon)
	}
	if got := clientTargetModeForCommand(reviewList, "", false); got != clientTargetEmbedded {
		t.Fatalf("review target mode = %v, want %v", got, clientTargetEmbedded)
	}
}

func TestLocalServerManagerStatusRunning(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	manager, err := newLocalServerManager(&Options{Logger: logger})
	if err != nil {
		t.Fatalf("newLocalServerManager() error = %v", err)
	}
	if err := os.MkdirAll(manager.paths.Dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	srv := api.NewServer(logger, api.ServerOptions{UnixSocket: manager.paths.Socket})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer srv.Shutdown(context.Background())

	if err := os.WriteFile(manager.paths.PIDFile, []byte("1234"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	status, err := manager.status(context.Background())
	if err != nil {
		t.Fatalf("status() error = %v", err)
	}
	if !status.Running {
		t.Fatal("status.Running = false, want true")
	}
	if status.PID != 1234 {
		t.Fatalf("status.PID = %d, want 1234", status.PID)
	}
	if status.Address != "unix://"+manager.paths.Socket {
		t.Fatalf("status.Address = %q, want %q", status.Address, "unix://"+manager.paths.Socket)
	}
}

func TestServerLifecycleCommandsDoNotRequireConfig(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"server", "status", "--config", filepath.Join(t.TempDir(), "missing.yaml")})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Local server not running.") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}
