package cli

import (
	"github.com/spf13/cobra"

	runtimeadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/runtime"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
)

type clientTargetMode int

const (
	clientTargetNone clientTargetMode = iota
	clientTargetExplicit
	clientTargetDaemon
	clientTargetEnsureDaemon
	clientTargetEmbedded
)

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

func newLocalServerManager(opts *Options) (*runtimeadapter.LocalServerManager, error) {
	return runtimeadapter.NewLocalServerManager(runtimeadapter.Options{
		ConfigPath:     opts.ConfigPath,
		ServerAddr:     opts.ServerAddr,
		Verbose:        opts.Verbose,
		Logger:         opts.Logger,
		ExecutablePath: opts.ExecutablePath,
	})
}
