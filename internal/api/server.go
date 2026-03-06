package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// schemaNamer is a custom schema namer that checks for a SchemaName() method
// on the type before falling back to huma's DefaultSchemaNamer. This avoids
// "duplicate name" panics when different packages define types with the same
// short Go name (e.g. bugsync.SyncAction vs project.SyncAction).
func schemaNamer(t reflect.Type, hint string) string {
	t = derefType(t)
	v := reflect.New(t)
	if namer, ok := v.Interface().(interface{ SchemaName() string }); ok {
		if name := namer.SchemaName(); name != "" {
			return name
		}
	}
	return huma.DefaultSchemaNamer(t, hint)
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

const Version = "0.1.0"
const defaultReadHeaderTimeout = 5 * time.Second

// ServerOptions configures the HTTP server.
type ServerOptions struct {
	// ListenAddr is the TCP address to listen on (e.g. "127.0.0.1:8472").
	ListenAddr string
	// UnixSocket, if set, makes the server listen on a unix domain socket
	// instead of TCP.
	UnixSocket string
}

// Server is the HTTP server for Sunbeam Watchtower.
type Server struct {
	router   chi.Router
	api      huma.API
	logger   *slog.Logger
	listener net.Listener
	httpSrv  *http.Server
	opts     ServerOptions
}

// NewServer creates a new Server with a chi router and huma API.
func NewServer(logger *slog.Logger, opts ServerOptions) *Server {
	router := chi.NewMux()
	cfg := huma.DefaultConfig("Sunbeam Watchtower API", Version)
	cfg.Components.Schemas = huma.NewMapRegistry("#/components/schemas/", schemaNamer)
	api := humachi.New(router, cfg)

	s := &Server{
		router: router,
		api:    api,
		logger: logger,
		opts:   opts,
	}
	s.registerHealth()
	return s
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok"`
	}
}

func (s *Server) registerHealth() {
	huma.Register(s.api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/api/v1/health",
		Summary:     "Health check",
	}, func(_ context.Context, _ *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}

// Start creates a net.Listener and begins serving HTTP requests.
func (s *Server) Start() error {
	var (
		ln  net.Listener
		err error
	)

	if s.opts.UnixSocket != "" {
		// Remove stale socket file if it exists.
		if err = os.Remove(s.opts.UnixSocket); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale socket: %w", err)
		}
		ln, err = net.Listen("unix", s.opts.UnixSocket)
		if err != nil {
			return fmt.Errorf("listen unix %s: %w", s.opts.UnixSocket, err)
		}
		s.logger.Info("listening on unix socket", "path", s.opts.UnixSocket)
	} else {
		addr := s.opts.ListenAddr
		if addr == "" {
			addr = "127.0.0.1:8472"
		}
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("listen tcp %s: %w", addr, err)
		}
		s.logger.Info("listening on TCP", "addr", ln.Addr().String())
	}

	s.listener = ln
	s.httpSrv = &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("http server error", "err", err)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the HTTP server and cleans up resources.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	err := s.httpSrv.Shutdown(ctx)

	// Clean up unix socket file.
	if s.opts.UnixSocket != "" {
		if removeErr := os.Remove(s.opts.UnixSocket); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warn("failed to remove unix socket", "path", s.opts.UnixSocket, "err", removeErr)
		}
	}
	return err
}

// API returns the huma.API so handlers can be registered externally.
func (s *Server) API() huma.API {
	return s.api
}

// Addr returns the listener address after Start has been called.
// For TCP listeners this is "host:port"; for unix sockets it is the socket path.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	if s.opts.UnixSocket != "" {
		return s.opts.UnixSocket
	}
	return s.listener.Addr().String()
}
