package festival_test

import (
	"context"
	"io"

	"github.com/Obedience-Corp/fest/pkg/festgif"
	"github.com/Obedience-Corp/fest/pkg/festgif/festival"
)

// Render a festival's replay as a GIF from another Go program.
func ExampleLoad() {
	ctx := context.Background()
	in, err := festival.Load(ctx, "festivals/active/my-festival-MF0001")
	if err != nil {
		return
	}
	// Write to an *os.File in real use.
	_, _ = festgif.Render(ctx, io.Discard, festgif.Plan(in, festgif.DefaultTiming))
}
