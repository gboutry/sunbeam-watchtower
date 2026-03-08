package cli

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/api"
	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	runtimeadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/runtime"
	"github.com/gboutry/sunbeam-watchtower/internal/app"
	"github.com/gboutry/sunbeam-watchtower/internal/config"
	"github.com/gboutry/sunbeam-watchtower/pkg/client"
	"github.com/spf13/cobra"
)

// Options holds resolved CLI state shared across commands.
type Options struct {
	Config         *config.Config
	ConfigPath     string
	Verbose        bool
	Output         string // "table", "json", "yaml"
	NoColor        bool
	Logger         *slog.Logger
	Out            io.Writer
	ErrOut         io.Writer
	App            *app.App
	Client         *client.Client
	ServerAddr     string // external server address (--server / WATCHTOWER_SERVER)
	ExecutablePath string

	embeddedSrv    *api.Server // auto-started embedded server
	frontendFacade *frontend.ClientFacade
	frontendClient *client.Client
	frontendApp    *app.App
}

// Frontend returns the shared client-side frontend facade for the current command execution.
func (o *Options) Frontend() *frontend.ClientFacade {
	if o.frontendFacade == nil || o.frontendClient != o.Client || o.frontendApp != o.App {
		o.frontendFacade = frontend.NewClientFacade(frontend.NewClientTransport(o.Client), o.App)
		o.frontendClient = o.Client
		o.frontendApp = o.App
	}
	return o.frontendFacade
}

// NewRootCmd creates the root watchtower command with all subcommands.
func NewRootCmd(opts *Options) *cobra.Command {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.ErrOut == nil {
		opts.ErrOut = os.Stderr
	}

	root := &cobra.Command{
		Use:           "watchtower",
		Short:         "Unified dashboard for Sunbeam across GitHub, Launchpad, and Gerrit",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Apply env var defaults for flags not explicitly set.
			runtimeOpts := runtimeadapter.Options{
				ConfigPath: opts.ConfigPath,
				ServerAddr: opts.ServerAddr,
				Verbose:    opts.Verbose,
			}
			runtimeadapter.ApplyEnvDefaults(&runtimeOpts)
			opts.ConfigPath = runtimeOpts.ConfigPath
			opts.ServerAddr = runtimeOpts.ServerAddr
			opts.Verbose = runtimeOpts.Verbose
			if v := os.Getenv("WATCHTOWER_CONFIG"); v != "" && opts.ConfigPath == "" {
				opts.ConfigPath = v
			}
			if v := os.Getenv("WATCHTOWER_VERBOSE"); v != "" && !opts.Verbose {
				opts.Verbose = v == "1" || v == "true" || v == "TRUE" || v == "True"
			}
			if v := os.Getenv("WATCHTOWER_OUTPUT"); v != "" && opts.Output == "table" {
				opts.Output = v
			}
			if v := os.Getenv("WATCHTOWER_NO_COLOR"); v != "" && !opts.NoColor {
				opts.NoColor = v == "1" || v == "true" || v == "TRUE" || v == "True"
			}
			if v := os.Getenv("WATCHTOWER_SERVER"); v != "" && opts.ServerAddr == "" {
				opts.ServerAddr = v
			}

			opts.Logger = runtimeadapter.NewLogger(opts.Verbose, opts.ErrOut)

			if commandNeedsConfig(cmd) {
				cfg, err := config.Load(opts.ConfigPath)
				if err != nil {
					return err
				}
				opts.Config = cfg
			}

			if commandNeedsApp(cmd) {
				opts.App = app.NewAppWithOptions(opts.Config, opts.Logger, app.Options{
					RuntimeMode: runtimeModeForCommand(cmd, opts),
				})
			}

			if commandNeedsClient(cmd) {
				manager, err := newLocalServerManager(opts)
				if err != nil {
					return err
				}
				daemonStatus, err := manager.Status(cmd.Context())
				if err != nil {
					return err
				}

				switch clientTargetModeForCommand(cmd, opts.ServerAddr, daemonStatus.Running) {
				case clientTargetExplicit:
					opts.Client = client.NewClient(opts.ServerAddr)
				case clientTargetDaemon:
					opts.ServerAddr = daemonStatus.Address
					opts.Client = client.NewClient(daemonStatus.Address)
				case clientTargetEnsureDaemon:
					status, started, err := manager.EnsureRunning(cmd.Context())
					if err != nil {
						return err
					}
					if started {
						opts.Logger.Info("started local watchtower server", "address", status.Address, "pid", status.PID, "log_file", status.LogFile)
					}
					opts.ServerAddr = status.Address
					opts.Client = client.NewClient(status.Address)
				case clientTargetEmbedded:
					srv := runtimeadapter.NewConfiguredServer(opts.Logger, opts.App, api.ServerOptions{ListenAddr: "127.0.0.1:0"})
					if err := srv.Start(); err != nil {
						return err
					}
					opts.embeddedSrv = srv
					opts.Client = client.NewClient("http://" + srv.Addr())
				}
			}
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if opts.embeddedSrv != nil {
				err = opts.embeddedSrv.Shutdown(context.Background())
			}
			if opts.App != nil {
				err = errors.Join(err, opts.App.Close())
			}
			opts.frontendFacade = nil
			opts.frontendClient = nil
			opts.frontendApp = nil
			return err
		},
	}

	root.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	root.PersistentFlags().BoolVar(&opts.Verbose, "verbose", false, "enable debug logging")
	root.PersistentFlags().StringVarP(&opts.Output, "output", "o", "table", "output format: table, json, yaml")
	root.PersistentFlags().BoolVar(&opts.NoColor, "no-color", false, "disable colored output")
	root.PersistentFlags().StringVar(&opts.ServerAddr, "server", "", "server address (http://host:port or unix:///path)")

	root.AddGroup(
		&cobra.Group{ID: "workflow", Title: "Workflows"},
		&cobra.Group{ID: "meta", Title: "Meta Commands"},
	)
	root.SetHelpCommandGroupID("meta")
	root.SetCompletionCommandGroupID("meta")

	root.AddCommand(
		withGroupID(newReviewCmd(opts), "workflow"),
		withGroupID(newCommitCmd(opts), "workflow"),
		withGroupID(newBugCmd(opts), "workflow"),
		withGroupID(newBuildCmd(opts), "workflow"),
		withGroupID(newReleasesCmd(opts), "workflow"),
		withGroupID(newProjectCmd(opts), "workflow"),
		withGroupID(newPackagesCmd(opts), "workflow"),
	)
	root.AddCommand(
		withGroupID(newVersionCmd(opts), "meta"),
		withGroupID(newConfigCmd(opts), "meta"),
		withGroupID(newAuthCmd(opts), "meta"),
		withGroupID(newOperationCmd(opts), "meta"),
		withGroupID(newCacheCmd(opts), "meta"),
		withGroupID(newServeCmd(opts), "meta"),
		withGroupID(newServerCmd(opts), "meta"),
	)

	return root
}

func withGroupID(cmd *cobra.Command, groupID string) *cobra.Command {
	cmd.GroupID = groupID
	return cmd
}
