package routes

import (
	"os"
	"strings"

	"stellarbill-backend/internal/auth"
	"stellarbill-backend/internal/config"
	"stellarbill-backend/internal/cors"
	"stellarbill-backend/internal/handlers"
	"stellarbill-backend/internal/idempotency"
	"stellarbill-backend/internal/logger"
	"stellarbill-backend/internal/middleware"
	"stellarbill-backend/internal/reconciliation"
	"stellarbill-backend/internal/repository"
	"stellarbill-backend/internal/service"
	"stellarbill-backend/internal/startup"
	"stellarbill-backend/internal/tracing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Register(r *gin.Engine) {
	cfg, err := config.Load()
	if err != nil {
		// Routes must stay registrable in tests and local runs that do not set
		// secrets; fall back to development defaults and log the problem.
		logger.Log.Warnf("config validation failed, using development defaults: %v", err)
		cfg = config.DevelopmentDefaults()
	}

	// Initialize tracing
	if cfg.TracingExporter != "none" {
		if _, err := tracing.InitTracer(cfg.TracingServiceName); err != nil {
			// Log error but continue
			logger.Log.Errorf("Failed to initialize tracer: %v", err)
		}
	}

	// Add OpenTelemetry middleware
	r.Use(otelgin.Middleware(cfg.TracingServiceName))
	// Add TraceID middleware to bridge OTEL trace ID to response headers
	r.Use(middleware.TraceIDMiddleware())

	corsProfile := cors.ProfileForEnv(cfg.Env, strings.Join(cfg.AllowedOrigins, ","))

	// Apply rate limiting middleware
	rateLimitConfig := middleware.RateLimiterConfig{
		Enabled:        cfg.RateLimitEnabled,
		Mode:           middleware.RateLimitMode(cfg.RateLimitMode),
		RequestsPerSec: int64(cfg.RateLimitRPS),
		BurstSize:      int64(cfg.RateLimitBurst),
		WhitelistPaths: cfg.RateLimitWhitelist,
	}
	r.Use(middleware.RateLimitMiddleware(rateLimitConfig))

	r.Use(cors.Middleware(corsProfile))

	store := idempotency.NewStore(idempotency.DefaultTTL)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-secret"
	}

	subRepo := repository.NewMockSubscriptionRepo()
	planRepo := repository.NewMockPlanRepo()
	svc := service.NewSubscriptionService(subRepo, planRepo)

	// Statement service wiring (in-memory mock for test/dev)
	stmtRepo := repository.NewMockStatementRepo()
	stmtSvc := service.NewStatementService(subRepo, stmtRepo)

	// Admin handler (token from env or default)
	adminToken := os.Getenv("ADMIN_TOKEN")
	adminHandler := handlers.NewAdminHandler(adminToken)
	// wire planRepo into handlers for list/detail endpoints and optional caching
	handlers.SetPlanRepository(planRepo)

	// Define the API version/group
	api := r.Group("/api")
	v1 := api.Group("/v1")

	dep := middleware.DeprecationHeaders()

	api.Use(idempotency.Middleware(store))

	// Public endpoints
	api.GET("/health", dep, handlers.Health)
	v1.GET("/health", handlers.Health)

	// Authenticated endpoints
	authenticated := api.Group("")
	authenticated.Use(auth.AuthMiddleware(jwtSecret))
	v1Authenticated := v1.Group("")
	v1Authenticated.Use(auth.AuthMiddleware(jwtSecret))

	authenticated.GET("/plans", dep, middleware.FeatureFlagWithDefault("plans_enabled", true), handlers.ListPlans)
	v1Authenticated.GET("/plans", middleware.FeatureFlagWithDefault("plans_enabled", true), handlers.ListPlans)

	subscriptionReaders := auth.AuthzMiddleware(auth.RoleAdmin, auth.RoleMerchant)

	authenticated.GET("/subscriptions", dep,
		middleware.FeatureFlagWithDefault("subscriptions_enabled", true),
		subscriptionReaders,
		handlers.ListSubscriptions,
	)
	v1Authenticated.GET("/subscriptions",
		middleware.FeatureFlagWithDefault("subscriptions_enabled", true),
		subscriptionReaders,
		handlers.ListSubscriptions,
	)

	authenticated.GET("/subscriptions/:id", dep,
		middleware.FeatureFlagWithDefault("subscriptions_enabled", true),
		subscriptionReaders,
		handlers.NewGetSubscriptionHandler(svc),
	)
	v1Authenticated.GET("/subscriptions/:id",
		middleware.FeatureFlagWithDefault("subscriptions_enabled", true),
		subscriptionReaders,
		handlers.NewGetSubscriptionHandler(svc),
	)

	authenticated.GET("/statements", dep, handlers.NewListStatementsHandler(stmtSvc))
	authenticated.GET("/statements/:id", dep, handlers.NewGetStatementHandler(stmtSvc))

	admin := api.Group("/admin")
	{
		admin.POST("/purge", adminHandler.PurgeCache)
		// Diagnostics endpoint — re-runs startup checks for live triage
		diagHandler := startup.NewDiagnosticsHandler(cfg, nil, nil)
		admin.GET("/diagnostics", auth.AuthzMiddleware(auth.RoleAdmin), diagHandler.Handle)

		// Reconciliation endpoint (admin-only) - accepts backend subscription list.
		// Choose adapter implementation via env var CONTRACT_SNAPSHOT_URL. If set, use HTTPAdapter.
		contractURL := os.Getenv("CONTRACT_SNAPSHOT_URL")
		var adapter reconciliation.Adapter
		if contractURL != "" {
			// Optional auth header via CONTRACT_SNAPSHOT_AUTH (e.g. "Bearer <token>")
			authHeader := os.Getenv("CONTRACT_SNAPSHOT_AUTH")
			adapter = reconciliation.NewHTTPAdapter(contractURL, authHeader)
		} else {
			// Default to in-memory adapter (empty) — replace or seed as needed in dev.
			adapter = reconciliation.NewMemoryAdapter()
		}
		// Wire in-memory store for persistence by default; can be swapped for DB-backed store.
		reconStore := reconciliation.NewMemoryStore()
		admin.POST("/reconcile", auth.AuthzMiddleware(auth.RoleAdmin), handlers.NewReconcileHandler(adapter, reconStore))
		// List persisted reports
		admin.GET("/reports", auth.AuthzMiddleware(auth.RoleAdmin), func(c *gin.Context) {
			reports, err := reconStore.ListReports()
			if err != nil {
				c.JSON(500, gin.H{"error": "failed to load reports"})
				return
			}
			c.JSON(200, gin.H{"reports": reports})
		})
	}
}
