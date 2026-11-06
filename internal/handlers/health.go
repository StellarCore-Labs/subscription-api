package handlers

import (
	"context"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"stellarbill-backend/internal/outbox"
)

// ServiceName identifies this service in health responses.
const ServiceName = "stellarbill-backend"

// Overall health states returned by the readiness endpoint.
const (
	StatusReady       = "ready"
	StatusDegraded    = "degraded"
	StatusUnavailable = "unavailable"
)

// Database health-check tunables.
const (
	// MaxRetries is the number of attempts made before reporting the database down.
	MaxRetries = 3
	// MaxDatabaseTimeout bounds each individual ping attempt.
	MaxDatabaseTimeout = 2 * time.Second
	// InitialBackoff is the base delay for exponential backoff between pings.
	InitialBackoff = 100 * time.Millisecond
)

// DBPinger is the minimal database surface needed by the readiness probe.
type DBPinger interface {
	PingContext(ctx context.Context) error
}

// HealthResponse is the payload returned by the readiness endpoint.
type HealthResponse struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Timestamp    string            `json:"timestamp"`
	Dependencies map[string]string `json:"dependencies"`
}

var globalOutboxManager *outbox.Manager

// SetOutboxManager wires the outbox manager used by the health endpoints.
func SetOutboxManager(m *outbox.Manager) {
	globalOutboxManager = m
}

// healthStatus builds the health payload, including outbox state when wired.
func healthStatus() gin.H {
	status := gin.H{
		"status":  "ok",
		"service": ServiceName,
	}

	if globalOutboxManager == nil {
		return status
	}

	if err := globalOutboxManager.Health(); err != nil {
		status["status"] = "degraded"
		status["outbox"] = gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		return status
	}

	if stats, err := globalOutboxManager.GetStats(); err == nil {
		status["outbox"] = stats
	}
	return status
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, healthStatus())
}

// Health reports service health for callers that do not use a Handler instance.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, healthStatus())
}

// OutboxStats returns detailed outbox statistics
func OutboxStats(c *gin.Context) {
	if globalOutboxManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Outbox manager not available",
		})
		return
	}

	stats, err := globalOutboxManager.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// PublishTestEvent publishes a test event for development/testing
func PublishTestEvent(c *gin.Context) {
	if globalOutboxManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Outbox manager not available",
		})
		return
	}

	// Get event type from query parameter
	eventType := c.Query("type")
	if eventType == "" {
		eventType = "test.event"
	}

	// Create test event data
	eventData := gin.H{
		"message":    "This is a test event",
		"timestamp":  gin.H{"$date": gin.H{"$numberLong": strconv.FormatInt(time.Now().UnixMilli(), 10)}},
		"request_id": c.GetHeader("X-Request-ID"),
		"user_agent": c.GetHeader("User-Agent"),
		"ip_address": c.ClientIP(),
	}

	// Publish the event
	service := globalOutboxManager.GetService()
	err := service.PublishEvent(c.Request.Context(), eventType, eventData, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Test event published successfully",
		"event_type": eventType,
	})
}

// ReadinessHandler checks if the service is ready
func ReadinessHandler(db DBPinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		deps := make(map[string]string)

		dbStatus := checkDatabase(db)
		deps["database"] = dbStatus

		overallStatus := deriveOverallStatus(deps)

		resp := HealthResponse{
			Status:       overallStatus,
			Service:      ServiceName,
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
			Dependencies: deps,
		}

		statusCode := http.StatusOK
		if overallStatus == StatusDegraded || overallStatus == StatusUnavailable {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, resp)
	}
}

// checkDatabase implements Timeout and Bounded Retry policies
func checkDatabase(db DBPinger) string {
	if os.Getenv("DATABASE_URL") == "" {
		return "not_configured"
	}
	if db == nil {
		return "down"
	}

	var lastErr error

	// IMPLEMENTATION: Bounded Retry Loop
	for i := 0; i < MaxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), MaxDatabaseTimeout)

		lastErr = db.PingContext(ctx)
		cancel() // Release context resources immediately

		if lastErr == nil {
			return "up"
		}

		// If not the last attempt, wait before retrying (Exponential Backoff)
		if i < MaxRetries-1 {
			backoff := time.Duration(math.Pow(2, float64(i))) * InitialBackoff
			time.Sleep(backoff)
		}
	}

	// Determine final failure state
	if lastErr == context.DeadlineExceeded {
		return "timeout"
	}
	return "down"
}

func deriveOverallStatus(deps map[string]string) string {
	for _, status := range deps {
		if status == "down" || status == "timeout" {
			return StatusDegraded
		}
	}
	return StatusReady
}
