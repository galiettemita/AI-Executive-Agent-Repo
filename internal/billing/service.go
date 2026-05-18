package billing

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SubscriptionPlan defines a billing plan.
type SubscriptionPlan struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"` // free, pro, business, enterprise
	PriceCents     int64    `json:"price_cents"`
	Interval       string   `json:"interval"` // monthly, annual
	Features       []string `json:"features"`
	LLMBudgetCents int64    `json:"llm_budget_cents"`
}

// WorkspaceSubscription tracks a workspace's subscription state.
type WorkspaceSubscription struct {
	ID                   string    `json:"id"`
	WorkspaceID          string    `json:"workspace_id"`
	PlanID               string    `json:"plan_id"`
	Status               string    `json:"status"` // active, past_due, cancelled
	CurrentPeriodStart   time.Time `json:"current_period_start"`
	CurrentPeriodEnd     time.Time `json:"current_period_end"`
	StripeSubscriptionID string    `json:"stripe_subscription_id"`
}

// BillingService manages subscriptions and billing.
type BillingService struct {
	mu            sync.Mutex
	plans         map[string]SubscriptionPlan
	subscriptions map[string]WorkspaceSubscription // keyed by workspace ID
	now           func() time.Time
}

// NewBillingService creates a new BillingService with default plans.
func NewBillingService() *BillingService {
	svc := &BillingService{
		plans:         map[string]SubscriptionPlan{},
		subscriptions: map[string]WorkspaceSubscription{},
		now:           func() time.Time { return time.Now().UTC() },
	}
	for _, p := range DefaultPlans() {
		svc.plans[p.ID] = p
	}
	return svc
}

// DefaultPlans returns the standard set of subscription plans.
func DefaultPlans() []SubscriptionPlan {
	return []SubscriptionPlan{
		{
			ID:             "plan_free",
			Name:           "free",
			PriceCents:     0,
			Interval:       "monthly",
			Features:       []string{"basic_chat", "5_connectors"},
			LLMBudgetCents: 500,
		},
		{
			ID:             "plan_pro",
			Name:           "pro",
			PriceCents:     2900,
			Interval:       "monthly",
			Features:       []string{"basic_chat", "25_connectors", "priority_support", "advanced_analytics"},
			LLMBudgetCents: 5000,
		},
		{
			ID:             "plan_business",
			Name:           "business",
			PriceCents:     9900,
			Interval:       "monthly",
			Features:       []string{"basic_chat", "unlimited_connectors", "sso", "audit_log", "custom_branding"},
			LLMBudgetCents: 25000,
		},
		{
			ID:             "plan_enterprise",
			Name:           "enterprise",
			PriceCents:     0, // custom pricing
			Interval:       "annual",
			Features:       []string{"basic_chat", "unlimited_connectors", "sso", "audit_log", "custom_branding", "dedicated_support", "sla"},
			LLMBudgetCents: 100000,
		},
	}
}

// Subscribe creates a subscription for a workspace.
func (s *BillingService) Subscribe(workspaceID, planID string) (*WorkspaceSubscription, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.plans[planID]; !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if existing, ok := s.subscriptions[workspaceID]; ok && existing.Status == "active" {
		return nil, fmt.Errorf("workspace %s already has an active subscription", workspaceID)
	}

	now := s.now()
	sub := WorkspaceSubscription{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:        workspaceID,
		PlanID:             planID,
		Status:             "active",
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	s.subscriptions[workspaceID] = sub
	return &sub, nil
}

// ChangePlan changes a workspace's subscription plan.
func (s *BillingService) ChangePlan(workspaceID, newPlanID string) (*WorkspaceSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.plans[newPlanID]; !ok {
		return nil, fmt.Errorf("plan not found: %s", newPlanID)
	}

	sub, ok := s.subscriptions[workspaceID]
	if !ok {
		return nil, fmt.Errorf("no subscription for workspace: %s", workspaceID)
	}
	if sub.Status != "active" {
		return nil, fmt.Errorf("subscription is not active: %s", sub.Status)
	}

	sub.PlanID = newPlanID
	s.subscriptions[workspaceID] = sub
	return &sub, nil
}

// CancelSubscription cancels a workspace's subscription.
func (s *BillingService) CancelSubscription(workspaceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[workspaceID]
	if !ok {
		return fmt.Errorf("no subscription for workspace: %s", workspaceID)
	}
	if sub.Status == "cancelled" {
		return fmt.Errorf("subscription is already cancelled")
	}

	sub.Status = "cancelled"
	s.subscriptions[workspaceID] = sub
	return nil
}

// GetSubscription returns the current subscription for a workspace.
func (s *BillingService) GetSubscription(workspaceID string) (*WorkspaceSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[workspaceID]
	if !ok {
		return nil, fmt.Errorf("no subscription for workspace: %s", workspaceID)
	}
	return &sub, nil
}

// HandleStripeWebhook processes Stripe webhook events.
func (s *BillingService) HandleStripeWebhook(eventType string, payload map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch eventType {
	case "invoice.paid":
		wsID, ok := payload["workspace_id"].(string)
		if !ok || wsID == "" {
			return fmt.Errorf("missing workspace_id in payload")
		}
		sub, exists := s.subscriptions[wsID]
		if !exists {
			return fmt.Errorf("no subscription for workspace: %s", wsID)
		}
		sub.Status = "active"
		now := s.now()
		sub.CurrentPeriodStart = now
		sub.CurrentPeriodEnd = now.AddDate(0, 1, 0)
		s.subscriptions[wsID] = sub
		return nil

	case "invoice.payment_failed":
		wsID, ok := payload["workspace_id"].(string)
		if !ok || wsID == "" {
			return fmt.Errorf("missing workspace_id in payload")
		}
		sub, exists := s.subscriptions[wsID]
		if !exists {
			return fmt.Errorf("no subscription for workspace: %s", wsID)
		}
		sub.Status = "past_due"
		s.subscriptions[wsID] = sub
		return nil

	case "customer.subscription.deleted":
		wsID, ok := payload["workspace_id"].(string)
		if !ok || wsID == "" {
			return fmt.Errorf("missing workspace_id in payload")
		}
		sub, exists := s.subscriptions[wsID]
		if !exists {
			return fmt.Errorf("no subscription for workspace: %s", wsID)
		}
		sub.Status = "cancelled"
		s.subscriptions[wsID] = sub
		return nil

	default:
		return fmt.Errorf("unhandled event type: %s", eventType)
	}
}
