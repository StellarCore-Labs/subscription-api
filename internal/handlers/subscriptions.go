package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"stellarbill-backend/internal/requestparams"
	"stellarbill-backend/internal/service"
	"stellarbill-backend/internal/subscriptions"
)

type Subscription struct {
	ID          string `json:"id"`
	PlanID      string `json:"plan_id"`
	Customer    string `json:"customer"`
	Status      string `json:"status"`
	Amount      string `json:"amount"`
	Interval    string `json:"interval"`
	NextBilling string `json:"next_billing,omitempty"`
}

// subscriptionDetailer is the part of the subscription service needed to build a detail response.
type subscriptionDetailer interface {
	GetDetail(ctx context.Context, tenantID, callerID, subscriptionID string) (*service.SubscriptionDetail, []string, error)
}

func (h *Handler) ListSubscriptions(c *gin.Context) {
	// Delegate to the injected service/repo. Keep behavior minimal and compatible with tests.
	subs, err := h.Subscriptions.ListSubscriptions(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscriptions": subs})
}

// ListSubscriptions serves the subscription list without an injected Handler.
func ListSubscriptions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"subscriptions": []Subscription{}})
}

// GetSubscription returns a placeholder subscription keyed by the :id path param.
func GetSubscription(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subscription id required"})
		return
	}
	c.JSON(http.StatusOK, Subscription{
		ID:       id,
		PlanID:   "plan_placeholder",
		Customer: "customer_placeholder",
		Status:   "placeholder",
		Amount:   "0",
		Interval: "monthly",
	})
}

// GetSubscription returns the full subscription detail through the injected service.
func (h *Handler) GetSubscription(c *gin.Context) {
	respondWithSubscriptionDetail(c, h.Subscriptions)
}

// NewGetSubscriptionHandler returns a gin.HandlerFunc that retrieves a full
// subscription detail using the provided SubscriptionService.
func NewGetSubscriptionHandler(svc service.SubscriptionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		respondWithSubscriptionDetail(c, svc)
	}
}

// respondWithSubscriptionDetail validates the request, calls the service, and
// renders either a standard error envelope or the detail envelope.
func respondWithSubscriptionDetail(c *gin.Context, svc subscriptionDetailer) {
	callerID, exists := c.Get("callerID")
	if !exists {
		RespondWithAuthError(c, "authentication required")
		return
	}
	caller, ok := callerID.(string)
	if !ok || caller == "" {
		RespondWithAuthError(c, "authentication required")
		return
	}

	tenantID, _ := c.Get("tenantID")
	tenant, _ := tenantID.(string)

	// Reject unexpected query parameters (e.g. ?expand=plan).
	if _, err := requestparams.SanitizeQuery(c.Request.URL.Query(), requestparams.QueryRules{}); err != nil {
		RespondWithValidationError(c, "invalid query parameters", nil)
		return
	}

	id, err := requestparams.NormalizePathID("id", c.Param("id"))
	if err != nil {
		RespondWithValidationError(c, "invalid subscription id", map[string]interface{}{"field": "id"})
		return
	}

	detail, warnings, err := svc.GetDetail(c.Request.Context(), tenant, caller, id)
	if err != nil {
		status, code, message := MapServiceErrorToResponse(err)
		RespondWithError(c, status, code, message)
		return
	}

	resp := service.ResponseEnvelope{
		APIVersion: "1",
		Data:       detail,
		Warnings:   warnings,
	}

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.JSON(http.StatusOK, resp)
}

// UpdateSubscriptionStatus handles status updates with validation
func UpdateSubscriptionStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subscription id required"})
		return
	}

	var payload struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: fetch current subscription from DB
	currentStatus := "active" // placeholder

	if err := subscriptions.CanTransition(currentStatus, payload.Status); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": err.Error(),
		})
		return
	}

	// TODO: persist update

	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"status": payload.Status,
	})
}
