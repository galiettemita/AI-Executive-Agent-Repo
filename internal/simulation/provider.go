package simulation

import (
	"context"
	"time"

	"github.com/brevio/brevio/internal/billing"
	"github.com/brevio/brevio/internal/wallet"
)

// CalendarSnapshotProvider fetches existing calendar events.
type CalendarSnapshotProvider interface {
	FetchSnapshot(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time) (CalendarSnapshot, error)
}

// FinancialSnapshotProvider fetches the current financial state for a workspace.
type FinancialSnapshotProvider interface {
	FetchSnapshot(ctx context.Context, workspaceID string) (FinancialSnapshot, error)
}

// WalletFinancialProvider implements FinancialSnapshotProvider using wallet and billing services.
type WalletFinancialProvider struct {
	walletSvc  *wallet.WalletService
	billingSvc *billing.BillingService
}

func NewWalletFinancialProvider(walletSvc *wallet.WalletService, billingSvc *billing.BillingService) *WalletFinancialProvider {
	return &WalletFinancialProvider{walletSvc: walletSvc, billingSvc: billingSvc}
}

func (p *WalletFinancialProvider) FetchSnapshot(_ context.Context, workspaceID string) (FinancialSnapshot, error) {
	snap := FinancialSnapshot{WorkspaceID: workspaceID, FetchedAt: time.Now().UTC(), WalletStatus: "active"}

	w, err := p.walletSvc.GetWallet(workspaceID)
	if err == nil {
		snap.WalletBalanceCents = w.BalanceCents
		snap.WalletStatus = w.Status
	}

	sub, err := p.billingSvc.GetSubscription(workspaceID)
	if err == nil {
		plan, planErr := p.billingSvc.GetPlan(sub.PlanID)
		if planErr == nil {
			snap.MonthlyBudgetCents = plan.LLMBudgetCents
		}
		snap.MonthlySpentCents = p.billingSvc.GetMonthlyLLMSpend(workspaceID)
	}

	return snap, nil
}

// NoOpCalendarProvider returns an empty snapshot.
type NoOpCalendarProvider struct{}

func (p *NoOpCalendarProvider) FetchSnapshot(_ context.Context, workspaceID string, _, _ time.Time) (CalendarSnapshot, error) {
	return CalendarSnapshot{WorkspaceID: workspaceID, FetchedAt: time.Now().UTC()}, nil
}
