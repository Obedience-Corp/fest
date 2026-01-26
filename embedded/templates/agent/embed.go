// Package agent provides embedded templates for agent-facing CLI output.
// All agent instruction templates live here for centralized maintainability.
package agent

import "embed"

//go:embed next/*.tmpl
//go:embed implementation/*.tmpl
//go:embed plan/*.tmpl
//go:embed research/*.tmpl
//go:embed review/*.tmpl
//go:embed action/*.tmpl
//go:embed ingest/*.tmpl
//go:embed validate/*.tmpl
var Templates embed.FS
