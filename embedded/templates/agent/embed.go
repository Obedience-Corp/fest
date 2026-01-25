// Package agent provides embedded templates for agent-facing CLI output.
// All agent instruction templates live here for centralized maintainability.
package agent

import "embed"

//go:embed next/*.tmpl
//go:embed execute/*.tmpl
//go:embed validate/*.tmpl
var Templates embed.FS
