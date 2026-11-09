package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"stellarbill-backend/internal/auth"
	"stellarbill-backend/internal/repository"
)

func testJWTSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "dev-secret"
}

func TestRegister_HealthAndCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine)

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin: got %q want %q", got, "*")
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("payload.status: got %v want %q", payload["status"], "ok")
	}
	if payload["service"] != "stellarbill-backend" {
		t.Fatalf("payload.service: got %v want %q", payload["service"], "stellarbill-backend")
	}
}

func TestRegister_CORSPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine)

	req := httptest.NewRequest(http.MethodOptions, "http://localhost:8080/api/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Methods to be set")
	}
}

func TestRegister_GetSubscriptionShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	subRepo := repository.NewMockSubscriptionRepo(&repository.SubscriptionRow{
		ID:         "sub_123",
		PlanID:     "plan_1",
		CustomerID: "admin-user",
		Status:     "active",
		Amount:     "1000",
		Currency:   "USD",
		Interval:   "monthly",
	})
	RegisterWithDependencies(engine, Dependencies{Subscriptions: subRepo})

	token, err := auth.NewTokenGenerator(testJWTSecret()).GenerateAdminToken("admin-user", "admin@example.com")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/subscriptions/sub_123", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload.data to be an object, got %T", payload["data"])
	}
	if data["id"] != "sub_123" {
		t.Fatalf("payload.data.id: got %v want %q", data["id"], "sub_123")
	}
	if _, ok := data["plan_id"]; !ok {
		t.Fatalf("expected payload.data.plan_id to be present")
	}
	if _, ok := data["customer"]; !ok {
		t.Fatalf("expected payload.data.customer to be present")
	}
}
