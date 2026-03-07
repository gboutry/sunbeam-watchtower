package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newServerCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage the local persistent Watchtower server",
	}
	cmd.AddCommand(
		newServerStartCmd(opts),
		newServerStatusCmd(opts),
		newServerStopCmd(opts),
	)
	return cmd
}

func newServerStartCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the local persistent Watchtower server",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newLocalServerManager(opts)
			if err != nil {
				return err
			}
			status, started, err := manager.ensureRunning(cmd.Context())
			if err != nil {
				return err
			}

			if started {
				fmt.Fprintf(opts.Out, "Started local server at %s (pid %d)\n", status.Address, status.PID)
			} else {
				fmt.Fprintf(opts.Out, "Local server already running at %s (pid %d)\n", status.Address, status.PID)
			}
			fmt.Fprintf(opts.Out, "Log file: %s\n", status.LogFile)
			if status.ConfigPath != "" {
				fmt.Fprintf(opts.Out, "Config: %s\n", status.ConfigPath)
			}
			return nil
		},
	}
}

func newServerStatusCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local persistent server status",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newLocalServerManager(opts)
			if err != nil {
				return err
			}
			status, err := manager.status(cmd.Context())
			if err != nil {
				return err
			}
			if !status.Running {
				fmt.Fprintf(opts.Out, "Local server not running.\nExpected address: %s\n", status.Address)
				if status.StaleSocket {
					fmt.Fprintf(opts.Out, "Stale socket file detected at %s\n", manager.paths.Socket)
				}
				if status.StalePIDFile {
					fmt.Fprintf(opts.Out, "Stale pid file detected at %s\n", manager.paths.PIDFile)
				}
				return nil
			}
			fmt.Fprintf(opts.Out, "Local server running at %s (pid %d)\n", status.Address, status.PID)
			fmt.Fprintf(opts.Out, "Log file: %s\n", status.LogFile)
			if status.ConfigPath != "" {
				fmt.Fprintf(opts.Out, "Config: %s\n", status.ConfigPath)
			}
			if !status.StartedAt.IsZero() {
				fmt.Fprintf(opts.Out, "Started: %s\n", status.StartedAt.Format(time.RFC3339))
			}
			return nil
		},
	}
}

func newServerStopCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the local persistent Watchtower server",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := newLocalServerManager(opts)
			if err != nil {
				return err
			}
			stopped, err := manager.stop(cmd.Context())
			if err != nil {
				return err
			}
			if !stopped {
				fmt.Fprintln(opts.Out, "Local server is not running. Cleaned up any stale local server files.")
				return nil
			}
			fmt.Fprintln(opts.Out, "Stopped local server.")
			return nil
		},
	}
}
