package localstore

// Manifest is .workflow/workflow.yaml. Schema defined in LOCAL_RUN_STATE.md.
type Manifest struct {
	Version     int         `yaml:"version"`
	Kind        string      `yaml:"kind"`
	WorkflowID  string      `yaml:"workflow_id"`
	WorkitemID  string      `yaml:"workitem_id"`
	DocPath     string      `yaml:"doc_path,omitempty"`
	DocHash     string      `yaml:"doc_hash,omitempty"`
	ActiveRunID string      `yaml:"active_run_id,omitempty"`
	Runs        []RunRecord `yaml:"runs,omitempty"`
}

// RunRecord is one entry in Manifest.Runs.
type RunRecord struct {
	RunID       string `yaml:"run_id"`
	Status      string `yaml:"status"`
	Path        string `yaml:"path"`
	StartedAt   string `yaml:"started_at,omitempty"`
	CompletedAt string `yaml:"completed_at,omitempty"`
}

// ManifestVersion is the only manifest schema version supported.
const ManifestVersion = 1

// ManifestKind is the expected kind value in workflow.yaml.
const ManifestKind = "workflow-runtime"
