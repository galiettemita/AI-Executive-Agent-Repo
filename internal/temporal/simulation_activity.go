package temporal

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/brevio/brevio/internal/simulation"
)

// SimulatePlanActivity runs the world-model constraint-satisfaction engine
// between GeneratePlanActivity and AuthorizePlanActivity.
func (a *Activities) SimulatePlanActivity(ctx context.Context, in simulation.SimulationInput) (simulation.SimulationResult, error) {
	logger := activity.GetLogger(ctx)

	if a.simulator == nil {
		return simulation.SimulationResult{
			PlanID: in.PlanID, Domain: simulation.DomainNone,
			Passed: true, SimulatedAt: time.Now().UTC(),
		}, nil
	}

	logger.Info("SimulatePlanActivity: running constraint checks",
		"workspace_id", in.WorkspaceID, "intent_len", len(in.Intent))

	result, err := a.simulator.Simulate(ctx, in)
	if err != nil {
		logger.Error("SimulatePlanActivity: simulation error, passing through", "error", err)
		result.Passed = true
		return result, nil
	}

	if !result.Passed {
		logger.Warn("SimulatePlanActivity: plan BLOCKED",
			"violation_count", len(result.Violations), "domain", result.Domain)
	}

	return result, nil
}
