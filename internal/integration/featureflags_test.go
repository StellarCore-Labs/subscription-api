package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"stellarbill-backend/internal/auth"
	"stellarbill-backend/internal/config"
	"stellarbill-backend/internal/featureflags"
	"stellarbill-backend/internal/routes"
)

// reloadFeatureFlags resets flags to their defaults and re-applies environment
// overrides. The flag manager is a process-wide singleton, so tests that
// change the environment must reload it.
func reloadFeatureFlags() {
	m := featureflags.GetInstance()
	m.LoadDefaultFlags()
	m.ReloadFromEnvironment()
}

func setupIntegrationTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	reloadFeatureFlags()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.Register(router)
	return router
}

// testAuthHeader returns a bearer token accepted by the router's JWT middleware.
func testAuthHeader(t *testing.T) string {
	t.Helper()
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}
	token, err := auth.NewTokenGenerator(secret).GenerateAdminToken("test-admin", "admin@example.com")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return "Bearer " + token
}

func TestHealthEndpoint_NoFeatureFlags(t *testing.T) {
	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["status"] != "ok" {
		t.Error("Health endpoint should work without feature flags")
	}
}

func TestSubscriptionsEndpoint_FlagEnabled(t *testing.T) {
	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/subscriptions", nil)
	req.Header.Set("Authorization", testAuthHeader(t))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSubscriptionsEndpoint_FlagDisabled(t *testing.T) {
	t.Setenv("FF_SUBSCRIPTIONS_ENABLED", "false")

	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/subscriptions", nil)
	req.Header.Set("Authorization", testAuthHeader(t))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["error"] != "feature_unavailable" {
		t.Error("Expected feature_unavailable error")
	}
}

func TestPlansEndpoint_FlagEnabled(t *testing.T) {
	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/plans", nil)
	req.Header.Set("Authorization", testAuthHeader(t))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestPlansEndpoint_FlagDisabled(t *testing.T) {
	t.Setenv("FF_PLANS_ENABLED", "false")

	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/plans", nil)
	req.Header.Set("Authorization", testAuthHeader(t))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestNewBillingFlowEndpoint_FlagDisabled(t *testing.T) {
	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/billing/new-flow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestNewBillingFlowEndpoint_FlagEnabled(t *testing.T) {
	t.Setenv("FF_NEW_BILLING_FLOW", "true")

	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/billing/new-flow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["message"] != "New billing flow is enabled" {
		t.Error("Expected new billing flow message")
	}
}

func TestAdvancedAnalyticsEndpoint_FlagsDisabled(t *testing.T) {
	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/analytics/advanced", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestAdvancedAnalyticsEndpoint_OneFlagEnabled(t *testing.T) {
	t.Setenv("FF_ADVANCED_ANALYTICS", "true")
	t.Setenv("FF_SUBSCRIPTIONS_ENABLED", "false")

	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/analytics/advanced", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestAdvancedAnalyticsEndpoint_AllFlagsEnabled(t *testing.T) {
	t.Setenv("FF_ADVANCED_ANALYTICS", "true")
	t.Setenv("FF_SUBSCRIPTIONS_ENABLED", "true")

	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/analytics/advanced", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["message"] != "Advanced analytics available" {
		t.Error("Expected advanced analytics message")
	}
}

func TestJSONFeatureFlags(t *testing.T) {
	t.Setenv("FEATURE_FLAGS", `{"subscriptions_enabled": false, "plans_enabled": false}`)

	router := setupIntegrationTestRouter(t)
	token := testAuthHeader(t)

	req1, _ := http.NewRequest("GET", "/api/subscriptions", nil)
	req1.Header.Set("Authorization", token)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != 503 {
		t.Errorf("Expected subscriptions to be disabled, got status %d", w1.Code)
	}

	req2, _ := http.NewRequest("GET", "/api/plans", nil)
	req2.Header.Set("Authorization", token)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != 503 {
		t.Errorf("Expected plans to be disabled, got status %d", w2.Code)
	}
}

func TestFeatureFlagPriority(t *testing.T) {
	t.Setenv("FEATURE_FLAGS", `{"new_billing_flow": false}`)
	t.Setenv("FF_NEW_BILLING_FLOW", "true")

	router := setupIntegrationTestRouter(t)

	req, _ := http.NewRequest("GET", "/api/billing/new-flow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200 (FF_ prefix should override JSON), got %d", w.Code)
	}
}

func TestConfigIntegration(t *testing.T) {
	os.Setenv("FF_DEFAULT_ENABLED", "true")
	os.Setenv("FF_LOG_DISABLED", "false")
	defer func() {
		os.Unsetenv("FF_DEFAULT_ENABLED")
		os.Unsetenv("FF_LOG_DISABLED")
	}()

	cfg, err := config.Load()
	if err != nil {
		// Secrets are not required for this assertion; fall back to defaults.
		cfg = config.DevelopmentDefaults()
	}

	if !cfg.FeatureFlags.DefaultEnabled {
		t.Error("Default enabled should be true")
	}

	if cfg.FeatureFlags.LogDisabled {
		t.Error("Log disabled should be false")
	}
}
