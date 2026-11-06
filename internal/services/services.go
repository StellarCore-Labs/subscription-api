package services

import (
	"context"

	"stellarbill-backend/internal/handlers"
	"stellarbill-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type planService struct{}

func NewPlanService() handlers.PlanService {
	return &planService{}
}

func (s *planService) ListPlans(c *gin.Context) ([]handlers.Plan, error) {
	return []handlers.Plan{}, nil
}

type subscriptionService struct{}

func NewSubscriptionService() handlers.SubscriptionService {
	return &subscriptionService{}
}

func (s *subscriptionService) ListSubscriptions(c *gin.Context) ([]handlers.Subscription, error) {
	return []handlers.Subscription{}, nil
}

func (s *subscriptionService) GetDetail(_ context.Context, tenantID, callerID, id string) (*service.SubscriptionDetail, []string, error) {
	return &service.SubscriptionDetail{
		ID:       id,
		Customer: callerID,
		Status:   "placeholder",
	}, nil, nil
}
