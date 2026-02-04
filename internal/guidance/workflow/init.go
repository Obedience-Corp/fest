package workflow

import "github.com/Obedience-Corp/fest/internal/guidance"

func init() {
	guidance.RegisterNavigator(guidance.ModeWorkflow, func(gctx *guidance.GuidanceContext) (guidance.Navigator, error) {
		return NewNavigator(gctx, guidance.ModeWorkflow)
	})
}
