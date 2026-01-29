package scope

import "context"

// Context keys are unexported to prevent external packages from
// directly manipulating context values.
type contextKey int

const (
	workspaceKey contextKey = iota
	festivalKey
)

// WorkspaceFrom extracts the resolved workspace from the context.
// Returns the workspace info and true if present, nil and false otherwise.
func WorkspaceFrom(ctx context.Context) (*WorkspaceInfo, bool) {
	ws, ok := ctx.Value(workspaceKey).(*WorkspaceInfo)
	return ws, ok
}

// FestivalFrom extracts the resolved festival path from the context.
// Returns the festival path and true if present, empty string and false otherwise.
func FestivalFrom(ctx context.Context) (string, bool) {
	path, ok := ctx.Value(festivalKey).(string)
	return path, ok
}

// WithWorkspace returns a new context with the workspace info set.
// This is used by the resolver to inject workspace context.
func WithWorkspace(ctx context.Context, ws *WorkspaceInfo) context.Context {
	return context.WithValue(ctx, workspaceKey, ws)
}

// WithFestival returns a new context with the festival path set.
// This is used by the resolver to inject festival context.
func WithFestival(ctx context.Context, festivalPath string) context.Context {
	return context.WithValue(ctx, festivalKey, festivalPath)
}
