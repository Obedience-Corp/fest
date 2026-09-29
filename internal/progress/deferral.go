package progress

// DeferralAudit carries the operator guard's findings from the command layer
// into the deferral event. It lives here rather than in internal/commands/task
// so the manager can record it without importing the command package.
type DeferralAudit struct {
	Actor        string
	TTY          bool
	AgentMarkers []string
	Ancestry     []string
	DeferredBy   string
}
