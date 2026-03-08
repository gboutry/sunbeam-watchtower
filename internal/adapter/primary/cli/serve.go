package cli

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/api"
	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	runtimeadapter "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/runtime"
	"github.com/spf13/cobra"
)

func newServeCmd(opts *Options) *cobra.Command {
	var listen string

	cmd := withActionID(&cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		Long:  "Start the Watchtower HTTP API server. The server provides a RESTful API for all watchtower operations and serves an OpenAPI spec at /openapi.json.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var serverOpts api.ServerOptions

			switch {
			case strings.HasPrefix(listen, "unix://"):
				serverOpts.UnixSocket = strings.TrimPrefix(listen, "unix://")
			case strings.HasPrefix(listen, "tcp://"):
				serverOpts.ListenAddr = strings.TrimPrefix(listen, "tcp://")
			default:
				serverOpts.ListenAddr = listen
			}

			srv := runtimeadapter.NewConfiguredServer(opts.Logger, opts.Application(), serverOpts)

			if err := srv.Start(); err != nil {
				return fmt.Errorf("starting server: %w", err)
			}

			addr := serverOpts.ListenAddr
			if addr == "" {
				addr = "127.0.0.1:8472"
			}
			if serverOpts.UnixSocket == "" {
				opts.Logger.Info("OpenAPI spec available", "url", "http://"+addr+"/openapi.json")
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			<-ctx.Done()

			opts.Logger.Info("shutting down gracefully")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			return srv.Shutdown(shutdownCtx)
		},
	}, frontend.ActionServeStart)

	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:8472", "listen address (tcp://host:port, unix:///path, or host:port)")
	return cmd
}
