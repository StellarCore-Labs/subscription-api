package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"stellarbill-backend/internal/config"
	"stellarbill-backend/internal/middleware"
	"stellarbill-backend/internal/routes"
	"stellarbill-backend/internal/security"
	"stellarbill-backend/internal/shutdown"
	"stellarbill-backend/internal/startup"
)

var listenAndServe = func(srv *http.Server) error {
	return srv.ListenAndServe()
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		printConfigError(err)
		os.Exit(1)
	}

	// Init PII-safe logger
	var logger *zap.Logger
	switch cfg.Env {
	case "production":
		logger = security.ProductionLogger()
		gin.SetMode(gin.ReleaseMode)
	case "development":
		logger = security.DevLogger()
		gin.SetMode(gin.DebugMode)
	default:
		logger = security.ProductionLogger()
		gin.SetMode(gin.TestMode)
	}
	defer func() { _ = logger.Sync() }()
	logger.Info("Starting stellarbill backend", zap.String("env", cfg.Env))

	// Log config warnings
	if vResult := cfg.Validate(); len(vResult.Warnings) > 0 {
		logger.Warn("Configuration warnings",
			zap.Strings("warnings", vResult.Warnings))
	}

	// Run startup diagnostics — fail fast if critical checks fail
	startupResults := startup.RunChecks(cfg, nil, nil) // db/migrations wired below once available
	logger.Info("Startup checks completed:\n" + startup.FormatResults(startupResults))
	if startup.HasFailures(startupResults) {
		logger.Fatal("Startup checks failed — aborting")
	}

	// Create the router with configured middleware and routes.
	router := newRouter()

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Second,
	}

	logger.Info("Starting Stellarbill backend server", zap.String("addr", addr))
	logger.Info("Server timeouts",
		zap.Int("read", cfg.ReadTimeout),
		zap.Int("write", cfg.WriteTimeout),
		zap.Int("idle", cfg.IdleTimeout))

	// Initialize graceful shutdown orchestrator
	// Shutdown timeout: 30 seconds (total time for all cleanup)
	// Drain timeout: 20 seconds (time to wait for in-flight requests)
	gracefulShutdown := shutdown.NewGracefulShutdown(
		srv,
		30*time.Second,
		20*time.Second,
	)

	// Register cleanup callbacks in reverse order of initialization
	gracefulShutdown.OnShutdown(func(ctx context.Context) error {
		log.Println("Cleanup callback: Releasing resources...")
		// Add any additional cleanup logic here
		// e.g., database connection pools, caches, etc.
		return nil
	})

	// Start server in a background goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Println("Listening for connections...")
		if err := listenAndServe(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// Start signal listener (blocks until shutdown is triggered)
	go gracefulShutdown.ListenForShutdownSignals()

	// Wait for either a server error or shutdown completion
	shutdownDone := make(chan struct{})
	go func() {
		gracefulShutdown.Wait()
		close(shutdownDone)
	}()

	select {
	case err := <-serverErrors:
		log.Fatalf("Server error: %v", err)
	case <-shutdownDone:
		log.Println("Server shutdown completed successfully")
	}
}

func printConfigError(err error) {
	fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
}

func newRouter() *gin.Engine {
	router := gin.New()
	router.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.Logging(log.Default()),
		middleware.CORS("*"),
		middleware.RateLimit(middleware.NewRateLimiter(60, time.Minute)),
	)
	routes.Register(router)
	return router
}
