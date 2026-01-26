package execute

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/implementation"
)

func init() {
	// Register the execution navigator with the guidance factory.
	// This uses the NavigatorFactoryFunc helper to avoid import cycles.
	guidance.RegisterNavigator(guidance.ModeImplementation, implementation.NavigatorFactoryFunc(newPlanBuilderAdapter))
}

// newPlanBuilderAdapter creates a PlanBuilder that implements the implementation.PlanBuilder interface.
// This wraps our existing PlanBuilder to bridge between packages.
func newPlanBuilderAdapter(festivalPath string) implementation.PlanBuilder {
	return &planBuilderAdapter{
		builder: NewPlanBuilder(festivalPath),
	}
}

// planBuilderAdapter adapts the execute.PlanBuilder to the implementation.PlanBuilder interface.
type planBuilderAdapter struct {
	builder *PlanBuilder
}

// BuildPlan implements implementation.PlanBuilder.
func (a *planBuilderAdapter) BuildPlan(ctx context.Context) (*implementation.ExecutionPlan, error) {
	return a.builder.BuildPlan(ctx)
}
