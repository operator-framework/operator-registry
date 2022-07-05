package serve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	endpoint "net/http/pprof"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/operator-framework/operator-registry/pkg/api"
	health "github.com/operator-framework/operator-registry/pkg/api/grpc_health_v1"
	"github.com/operator-framework/operator-registry/pkg/lib/dns"
	"github.com/operator-framework/operator-registry/pkg/lib/log"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/operator-framework/operator-registry/pkg/server"
)

type serve struct {
	configDir string

	port           string
	terminationLog string

	debug     bool
	pprofAddr string

	logger *logrus.Entry
}

const (
	defaultCpuStartupPath string = "/debug/pprof/startup/cpu"
)

func NewCmd() *cobra.Command {
	logger := logrus.New()
	s := serve{
		logger: logrus.NewEntry(logger),
	}
	cmd := &cobra.Command{
		Use:   "serve <source_path>",
		Short: "serve declarative configs",
		Long: `This command serves declarative configs via a GRPC server.

NOTE: The declarative config directory is loaded by the serve command at
startup. Changes made to the declarative config after the this command starts
will not be reflected in the served content.
`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			s.configDir = args[0]
			if s.debug {
				logger.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return s.run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&s.debug, "debug", false, "enable debug logging")
	cmd.Flags().StringVarP(&s.terminationLog, "termination-log", "t", "/dev/termination-log", "path to a container termination log file")
	cmd.Flags().StringVarP(&s.port, "port", "p", "50051", "port number to serve on")
	cmd.Flags().StringVar(&s.pprofAddr, "pprof-addr", "", "address of startup profiling endpoint (addr:port format)")
	return cmd
}

func (s *serve) run(ctx context.Context) error {
	p := newProfilerInterface(s.pprofAddr, s.logger)
	if err := p.startCpuProfileCache(); err != nil {
		return fmt.Errorf("could not start CPU profile: %v", err)
	}

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)

	// Immediately set up termination log
	err := log.AddDefaultWriterHooks(s.terminationLog)
	if err != nil {
		s.logger.WithError(err).Warn("unable to set termination log path")
	}

	// Ensure there is a default nsswitch config
	if err := dns.EnsureNsswitch(); err != nil {
		s.logger.WithError(err).Warn("unable to write default nsswitch config")
	}

	s.logger = s.logger.WithFields(logrus.Fields{"configs": s.configDir, "port": s.port})

	store, err := registry.NewQuerierFromFS(os.DirFS(s.configDir))
	if err != nil {
		return err
	}
	defer store.Close()

	lis, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		s.logger.Fatalf("failed to listen: %s", err)
	}

	grpcServer := grpc.NewServer()
	api.RegisterRegistryServer(grpcServer, server.NewRegistryServer(store))
	health.RegisterHealthServer(grpcServer, server.NewHealthServer())
	reflection.Register(grpcServer)

	eg.Go(func() error {
		// All this channel stuff is necessary so that we can return from
		// this function early when the context is cancelled. This is required
		// to get `eg.Wait()` to unblock, so that we can proceed to gracefully
		// shutting down.
		errChan := make(chan error)
		go func() {
			s.logger.Info("serving registry")
			errChan <- grpcServer.Serve(lis)
		}()
		select {
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	eg.Go(func() error {
		return p.listenAndServe(ctx)
	})
	eg.Go(func() (err error) {
		defer p.stopCpuProfileCache()
		if err := store.Wait(ctx); err != nil {
			return err
		}
		s.logger.Info("registry initialization complete")
		return nil
	})

	// wait until both errgroup goroutines return and then
	// return the first error that occurred (or nil)
	err = eg.Wait()

	// stop the servers prior to handling the error returned
	// from Wait().
	s.logger.Info("stopping grpc server")
	grpcServer.GracefulStop()
	if p.isEnabled() {
		s.logger.Info("stopping http pprof server")
		if err := p.shutdown(context.Background()); err != nil {
			return err
		}
	}

	if !errors.Is(err, context.Canceled) {
		return err
	}
	return nil

}

// manages an HTTP pprof endpoint served by `server`,
// including default pprof handlers and custom cpu pprof cache stored in `cache`.
// the cache is intended to sample CPU activity for a period and serve the data
// via a custom pprof path once collection is complete (e.g. over process initialization)
type profilerInterface struct {
	addr  string
	cache bytes.Buffer

	server http.Server

	cacheReady bool
	cacheLock  sync.RWMutex

	logger *logrus.Entry
}

func newProfilerInterface(a string, log *logrus.Entry) *profilerInterface {
	return &profilerInterface{
		addr:   a,
		logger: log.WithFields(logrus.Fields{"address": a}),
		cache:  bytes.Buffer{},
	}
}

func (p *profilerInterface) isEnabled() bool {
	return p.addr != ""
}

func (p *profilerInterface) listenAndServe(ctx context.Context) error {
	// short-circuit if not enabled
	if !p.isEnabled() {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", endpoint.Index)
	mux.HandleFunc("/debug/pprof/cmdline", endpoint.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", endpoint.Profile)
	mux.HandleFunc("/debug/pprof/symbol", endpoint.Symbol)
	mux.HandleFunc("/debug/pprof/trace", endpoint.Trace)
	mux.HandleFunc(defaultCpuStartupPath, p.httpHandler)

	p.server = http.Server{
		Addr:    p.addr,
		Handler: mux,
	}

	errChan := make(chan error)
	go func() {
		p.logger.Info("starting pprof endpoint")
		if err := p.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *profilerInterface) startCpuProfileCache() error {
	// short-circuit if not enabled
	if !p.isEnabled() {
		return nil
	}

	p.logger.Infof("start caching cpu profile data at %q", defaultCpuStartupPath)
	if err := pprof.StartCPUProfile(&p.cache); err != nil {
		return err
	}

	return nil
}

func (p *profilerInterface) stopCpuProfileCache() {
	// short-circuit if not enabled
	if !p.isEnabled() {
		return
	}
	pprof.StopCPUProfile()
	p.setCacheReady()
	p.logger.Info("stopped caching cpu profile data")
}

func (p *profilerInterface) httpHandler(w http.ResponseWriter, r *http.Request) {
	if !p.isCacheReady() {
		http.Error(w, "cpu profile cache is not yet ready", http.StatusServiceUnavailable)
	}
	w.Write(p.cache.Bytes())
}

func (p *profilerInterface) shutdown(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}

func (p *profilerInterface) isCacheReady() bool {
	p.cacheLock.RLock()
	isReady := p.cacheReady
	p.cacheLock.RUnlock()

	return isReady
}

func (p *profilerInterface) setCacheReady() {
	p.cacheLock.Lock()
	p.cacheReady = true
	p.cacheLock.Unlock()
}
