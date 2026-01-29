package workspace

// WorkspaceType indicates how the workspace was detected.
type WorkspaceType int

const (
	// WorkspaceTypeCampaign detected via .campaign/ directory.
	WorkspaceTypeCampaign WorkspaceType = iota

	// WorkspaceTypeStandalone detected via .workspace marker or nearest festivals/.
	WorkspaceTypeStandalone
)

// String returns a human-readable name for the workspace type.
func (t WorkspaceType) String() string {
	switch t {
	case WorkspaceTypeCampaign:
		return "campaign"
	case WorkspaceTypeStandalone:
		return "standalone"
	default:
		return "unknown"
	}
}

// WorkspaceInfo contains detected workspace information.
type WorkspaceInfo struct {
	// Root is the campaign root (if campaign) or festivals parent (if standalone).
	Root string

	// FestivalsPath is the absolute path to the festivals/ directory.
	FestivalsPath string

	// Type indicates how this workspace was detected.
	Type WorkspaceType
}
