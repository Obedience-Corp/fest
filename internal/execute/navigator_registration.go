package execute

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/execution"
)

func init() {
	// Register the execution navigator with the guidance factory.
	// This uses the NavigatorFactoryFunc helper to avoid import cycles.
	guidance.RegisterNavigator(guidance.ModeExecute, execution.NavigatorFactoryFunc(newPlanBuilderAdapter))
}

// newPlanBuilderAdapter creates a PlanBuilder that implements the execution.PlanBuilder interface.
// This wraps our existing PlanBuilder to bridge between packages.
func newPlanBuilderAdapter(festivalPath string) execution.PlanBuilder {
	return &planBuilderAdapter{
		builder: NewPlanBuilder(festivalPath),
	}
}

// planBuilderAdapter adapts the execute.PlanBuilder to the execution.PlanBuilder interface.
type planBuilderAdapter struct {
	builder *PlanBuilder
}

// BuildPlan implements execution.PlanBuilder.
func (a *planBuilderAdapter) BuildPlan(ctx context.Context) (*execution.ExecutionPlan, error) {
	return a.builder.BuildPlan(ctx)
}
