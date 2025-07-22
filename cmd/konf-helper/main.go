package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	i "github.com/gerolf-vent/konfig-sidecar/internal"
	"go.uber.org/zap"
)

var supportedSignals = map[string]syscall.Signal{
	"HUP":  syscall.SIGHUP,
	"INT":  syscall.SIGINT,
	"TERM": syscall.SIGTERM,
	"USR1": syscall.SIGUSR1,
	"USR2": syscall.SIGUSR2,
}

func main() {
	delay := flag.Duration("delay", 2*time.Second, "Debounce delay for path updates (default: 2s)")
	listenAddress := flag.String("address", ":9952", "Address to listen for HTTP health and ready probes (default: :9952)")
	processName := flag.String("process", "", "Name of process to signal on configuration updates (optional)")
	processSignalName := flag.String("signal", "", "Signal to send to the process (default: HUP)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Initialize logger
	var logger *zap.Logger
	var err error
	if debug != nil && *debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Validate and parse flags
	if flag.NArg() < 1 {
		logger.Fatal("at least one path must be specified")
	}

	// Parse path configurations
	pathConfigs := make([]*i.PathConfig, 0, flag.NArg())
	for _, pathSpec := range flag.Args() {
		pathConfig, err := i.ParsePathConfig(pathSpec)
		if err != nil {
			logger.Fatal("failed to parse path configuration", zap.String("spec", pathSpec), zap.Error(err))
		}
		pathConfigs = append(pathConfigs, pathConfig)
	}

	// Initialize paths sync service
	pss, err := i.NewPathsSyncService(*delay)
	if err != nil {
		logger.Fatal("failed to initialize paths sync service", zap.Error(err))
	}

	// Set path configurations
	for _, pathConfig := range pathConfigs {
		if err := pss.SetPathConfig(pathConfig); err != nil {
			logger.Fatal("failed to set path configuration", zap.String("path", pathConfig.SrcPath()), zap.Error(err))
		}
	}

	// Set process signaling options
	if *processName != "" {
		signal, ok := supportedSignals[*processSignalName]
		if !ok {
			logger.Fatal("unsupported signal", zap.String("signal", *processSignalName))
		}
		processNotifier := i.NewProcessNotifier(*processName, signal)
		pss.SetNotifier(processNotifier)
	}

	// Start http endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if pss.IsStarted() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Ready\n"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Not Ready\n"))
		}
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pss.CheckHealth(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Unhealthy: " + err.Error() + "\n"))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Healthy\n"))
		}
	})
	server := &http.Server{
		Addr:    *listenAddress,
		Handler: mux,
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server", zap.String("address", *listenAddress))
		serverErr <- server.ListenAndServe()
	}()

	// Start paths sync service
	if err := pss.Start(); err != nil {
		logger.Fatal("failed to start paths sync service", zap.Error(err))
	}

	// Listen for os signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("HTTP server error", zap.Error(err))
	case sig := <-sigChan:
		logger.Info("received signal, shutting down", zap.String("signal", sig.String()))

		pss.Stop()

		// Graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("failed to shutdown HTTP server", zap.Error(err))
		} else {
			logger.Info("HTTP server stopped")
		}
	}
}
