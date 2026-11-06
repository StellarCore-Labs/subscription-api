package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// DevelopmentDefaults returns a fully-populated Config that does not require any
// secrets. It is used by callers that must stay operable when the environment is
// not fully configured, such as test routers.
func DevelopmentDefaults() Config {
	return Config{
		Env:                "development",
		Port:               DefaultPort,
		MaxHeaderBytes:     MaxHeaderBytes,
		ReadTimeout:        DefaultReadTimeout,
		WriteTimeout:       DefaultWriteTimeout,
		IdleTimeout:        DefaultIdleTimeout,
		TracingExporter:    "none",
		TracingServiceName: "stellabill-backend",
		AllowedOrigins:     []string{"*"},
		FeatureFlags:       loadFeatureFlags(),
		Outbox:             DefaultOutboxConfig(),
		SecurityFrameOpt:   "DENY",
		SecurityHSTSMaxAge: "31536000",
	}
}

// FeatureFlagsConfig holds the global feature-flag settings.
type FeatureFlagsConfig struct {
	// DefaultEnabled is the state assumed for flags that are not configured.
	DefaultEnabled bool
	// LogDisabled logs a line whenever a flag blocks a request.
	LogDisabled bool
	// ConfigFile optionally points at a feature-flag config file.
	ConfigFile string
}

// OutboxConfig holds transactional-outbox dispatcher settings.
type OutboxConfig struct {
	PollInterval       time.Duration
	BatchSize          int
	MaxRetries         int
	RetryBackoffFactor float64
	CleanupInterval    time.Duration
	CompletedEventTTL  time.Duration
	ProcessingTimeout  time.Duration
	PublisherType      string
	HTTPEndpoint       string
}

// DefaultOutboxConfig mirrors the outbox dispatcher defaults.
func DefaultOutboxConfig() OutboxConfig {
	return OutboxConfig{
		PollInterval:       5 * time.Second,
		BatchSize:          10,
		MaxRetries:         3,
		RetryBackoffFactor: 2.0,
		CleanupInterval:    time.Hour,
		CompletedEventTTL:  24 * time.Hour,
		ProcessingTimeout:  30 * time.Second,
		PublisherType:      "console",
	}
}

func (o OutboxConfig) GetPollInterval() time.Duration {
	if o.PollInterval <= 0 {
		return 5 * time.Second
	}
	return o.PollInterval
}

func (o OutboxConfig) GetCleanupInterval() time.Duration {
	if o.CleanupInterval <= 0 {
		return time.Hour
	}
	return o.CleanupInterval
}

func (o OutboxConfig) GetCompletedEventTTL() time.Duration {
	if o.CompletedEventTTL <= 0 {
		return 24 * time.Hour
	}
	return o.CompletedEventTTL
}

func (o OutboxConfig) GetProcessingTimeout() time.Duration {
	if o.ProcessingTimeout <= 0 {
		return 30 * time.Second
	}
	return o.ProcessingTimeout
}

// loadFeatureFlags reads the global feature-flag settings from the environment.
func loadFeatureFlags() FeatureFlagsConfig {
	return FeatureFlagsConfig{
		DefaultEnabled: getEnvBool("FF_DEFAULT_ENABLED", false),
		LogDisabled:    getEnvBool("FF_LOG_DISABLED", true),
		ConfigFile:     os.Getenv("FF_CONFIG_FILE"),
	}
}

// loadOutbox reads outbox dispatcher settings from the environment, falling back
// to the dispatcher defaults for anything that is unset or invalid.
func loadOutbox() OutboxConfig {
	cfg := DefaultOutboxConfig()

	if v := os.Getenv("OUTBOX_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.PollInterval = d
		}
	}
	if v := os.Getenv("OUTBOX_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.BatchSize = n
		}
	}
	if v := os.Getenv("OUTBOX_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MaxRetries = n
		}
	}
	if v := os.Getenv("OUTBOX_RETRY_BACKOFF_FACTOR"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.RetryBackoffFactor = f
		}
	}
	if v := os.Getenv("OUTBOX_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.CleanupInterval = d
		}
	}
	if v := os.Getenv("OUTBOX_COMPLETED_EVENT_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.CompletedEventTTL = d
		}
	}
	if v := os.Getenv("OUTBOX_PROCESSING_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.ProcessingTimeout = d
		}
	}
	if v := os.Getenv("OUTBOX_PUBLISHER_TYPE"); v != "" {
		cfg.PublisherType = v
	}
	cfg.HTTPEndpoint = os.Getenv("OUTBOX_HTTP_ENDPOINT")

	return cfg
}

// loadAllowedOrigins reads the CORS allow-list from CORS_ALLOWED_ORIGINS.
func loadAllowedOrigins() []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if strings.TrimSpace(raw) == "" {
		return []string{"*"}
	}

	origins := make([]string, 0, len(raw))
	for _, origin := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
}
