package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/api"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	"github.com/spf13/cobra"
)

type clientTargetMode int

const (
	clientTargetNone clientTargetMode = iota
	clientTargetExplicit
	clientTargetDaemon
	clientTargetEnsureDaemon
	clientTargetEmbedded
)

type localServerPaths struct {
	Dir       string
	Socket    string
	PIDFile   string
	LogFile   string
	SocketURI string
}

type localServerStatus struct {
	Address string
	PID     int
	Running bool
	LogFile string
}

type localServerManager struct {
	logger         *slog.Logger
	configPath     string
	verbose        bool
	executablePath string
	paths          localServerPaths
}

func newConfiguredServer(logger *slog.Logger, application *app.App, serverOpts api.ServerOptions) *api.Server {
	srv := api.NewServer(logger, serverOpts)
	api.RegisterAuthAPI(srv.API(), application)
	api.RegisterPackagesAPI(srv.API(), application)
	api.RegisterBugsAPI(srv.API(), application)
	api.RegisterCacheAPI(srv.API(), application)
	api.RegisterConfigAPI(srv.API(), application)
	api.RegisterReviewsAPI(srv.API(), application)
	api.RegisterCommitsAPI(srv.API(), application)
	api.RegisterBuildsAPI(srv.API(), application)
	api.RegisterProjectsAPI(srv.API(), application)
	api.RegisterOperationsAPI(srv.API(), application)
	return srv
}

func commandNeedsConfig(cmd *cobra.Command) bool {
	return !commandSkipsConfig(cmd)
}

func commandSkipsConfig(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "version":
		return true
	case "status", "stop":
		return isServerLifecycleCommand(cmd)
	default:
		return false
	}
}

func commandNeedsClient(cmd *cobra.Command) bool {
	switch {
	case cmd.Name() == "version":
		return false
	case cmd.Name() == "serve":
		return false
	case isServerLifecycleCommand(cmd):
		return false
	default:
		return true
	}
}

func commandNeedsApp(cmd *cobra.Command) bool {
	switch {
	case cmd.Name() == "version":
		return false
	case isServerLifecycleCommand(cmd):
		return false
	default:
		return true
	}
}

func commandNeedsPersistentServer(cmd *cobra.Command) bool {
	parent := ""
	if p := cmd.Parent(); p != nil {
		parent = p.Name()
	}

	switch parent {
	case "auth":
		return true
	case "operation":
		return true
	case "build":
		if cmd.Name() == "trigger" {
			async, _ := cmd.Flags().GetBool("async")
			return async
		}
	case "project":
		if cmd.Name() == "sync" {
			async, _ := cmd.Flags().GetBool("async")
			return async
		}
	}

	return false
}

func isServerLifecycleCommand(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Parent() == nil {
		return false
	}
	return cmd.Parent().Name() == "server"
}

func runtimeModeForCommand(cmd *cobra.Command, opts *Options) app.RuntimeMode {
	switch {
	case cmd.Name() == "serve":
		return app.RuntimeModePersistent
	case opts.ServerAddr != "":
		return app.RuntimeModeEphemeral
	default:
		return app.RuntimeModeEphemeral
	}
}

func clientTargetModeForCommand(cmd *cobra.Command, explicitServer string, daemonRunning bool) clientTargetMode {
	switch {
	case !commandNeedsClient(cmd):
		return clientTargetNone
	case explicitServer != "":
		return clientTargetExplicit
	case daemonRunning:
		return clientTargetDaemon
	case commandNeedsPersistentServer(cmd):
		return clientTargetEnsureDaemon
	default:
		return clientTargetEmbedded
	}
}

func resolveLocalServerPaths() (localServerPaths, error) {
	var dir string
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		dir = filepath.Join(runtimeDir, "sunbeam-watchtower")
	} else {
		cacheDir, err := app.ResolveCacheDir()
		if err != nil {
			return localServerPaths{}, err
		}
		dir = filepath.Join(cacheDir, "run")
	}

	return localServerPaths{
		Dir:       dir,
		Socket:    filepath.Join(dir, "watchtower.sock"),
		PIDFile:   filepath.Join(dir, "watchtower.pid"),
		LogFile:   filepath.Join(dir, "watchtower.log"),
		SocketURI: "unix://" + filepath.Join(dir, "watchtower.sock"),
	}, nil
}

func newLocalServerManager(opts *Options) (*localServerManager, error) {
	paths, err := resolveLocalServerPaths()
	if err != nil {
		return nil, err
	}

	executablePath := opts.ExecutablePath
	if executablePath == "" {
		executablePath, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve executable path: %w", err)
		}
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &localServerManager{
		logger:         logger,
		configPath:     opts.ConfigPath,
		verbose:        opts.Verbose,
		executablePath: executablePath,
		paths:          paths,
	}, nil
}

func (m *localServerManager) status(ctx context.Context) (localServerStatus, error) {
	status := localServerStatus{
		Address: m.paths.SocketURI,
		LogFile: m.paths.LogFile,
	}

	if pid, err := readPIDFile(m.paths.PIDFile); err == nil {
		status.PID = pid
	}

	if healthy := client.NewClient(m.paths.SocketURI).Health(ctx) == nil; !healthy {
		return status, nil
	}

	status.Running = true
	return status, nil
}

func (m *localServerManager) ensureRunning(ctx context.Context) (localServerStatus, bool, error) {
	status, err := m.status(ctx)
	if err != nil {
		return localServerStatus{}, false, err
	}
	if status.Running {
		return status, false, nil
	}

	if err := os.MkdirAll(m.paths.Dir, 0o755); err != nil {
		return localServerStatus{}, false, fmt.Errorf("create runtime dir: %w", err)
	}

	_ = os.Remove(m.paths.Socket)

	logFile, err := os.OpenFile(m.paths.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return localServerStatus{}, false, fmt.Errorf("open server log: %w", err)
	}
	defer logFile.Close()

	args := []string{"serve", "--listen", m.paths.SocketURI}
	if m.configPath != "" {
		args = append(args, "--config", m.configPath)
	}
	if m.verbose {
		args = append(args, "--verbose")
	}

	cmd := exec.Command(m.executablePath, args...)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return localServerStatus{}, false, fmt.Errorf("start local server process: %w", err)
	}
	if err := os.WriteFile(m.paths.PIDFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		return localServerStatus{}, false, fmt.Errorf("write server pid file: %w", err)
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err = m.status(ctx)
		if err != nil {
			return localServerStatus{}, false, err
		}
		if status.Running {
			return status, true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return localServerStatus{}, false, fmt.Errorf("local server did not become healthy; see %s", m.paths.LogFile)
}

func (m *localServerManager) stop(ctx context.Context) (bool, error) {
	status, err := m.status(ctx)
	if err != nil {
		return false, err
	}
	if !status.Running {
		_ = os.Remove(m.paths.PIDFile)
		return false, nil
	}
	if status.PID == 0 {
		return false, fmt.Errorf("local server is running at %s but %s is missing", status.Address, m.paths.PIDFile)
	}

	proc, err := os.FindProcess(status.PID)
	if err != nil {
		return false, fmt.Errorf("find local server process: %w", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return false, fmt.Errorf("signal local server: %w", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status, err = m.status(ctx)
		if err != nil {
			return false, err
		}
		if !status.Running {
			_ = os.Remove(m.paths.PIDFile)
			return true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return false, fmt.Errorf("local server did not stop within timeout")
}

func readPIDFile(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return 0, fmt.Errorf("parse pid file %s: %w", path, err)
	}
	return pid, nil
}
