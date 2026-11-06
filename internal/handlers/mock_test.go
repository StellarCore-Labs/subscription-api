package handlers

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"

	"stellarbill-backend/internal/service"
)

type MockPlanService struct {
	mock.Mock
}

func (m *MockPlanService) ListPlans(c *gin.Context) ([]Plan, error) {
	args := m.Called(c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Plan), args.Error(1)
}

type MockSubscriptionService struct {
	mock.Mock
}

func (m *MockSubscriptionService) ListSubscriptions(c *gin.Context) ([]Subscription, error) {
	args := m.Called(c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Subscription), args.Error(1)
}

func (m *MockSubscriptionService) GetDetail(ctx context.Context, tenantID, callerID, id string) (*service.SubscriptionDetail, []string, error) {
	args := m.Called(ctx, tenantID, callerID, id)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*service.SubscriptionDetail), args.Get(1).([]string), args.Error(2)
}
