// Package methodology embeds the festival methodology scaffold that 'fest
// init' copies into a new workspace.
//
// The scaffold normally arrives by syncing from GitHub, which makes a working
// network a hard prerequisite for ever running 'fest init' — including on
// machines that will never have one. Embedding the same tree the sync would
// fetch lets init work from the binary alone, and leaves sync as the way to
// pick up methodology changes rather than the way to obtain it at all.
//
// This is the same directory the default repository serves, so the bundled
// copy and a fresh sync of the default ref agree by construction at build
// time; they diverge only as the upstream methodology moves on.
package methodology

import "embed"

// Root is the directory within FS holding the scaffold.
const Root = "festivals"

// FS holds the methodology scaffold.
//
// The all: prefix matters: without it go:embed silently skips entries whose
// names begin with a dot, which here would drop .festival — the directory
// holding the templates and agent guides that make the scaffold useful — and
// .gitignore, leaving a workspace that looks complete and is not.
//
//go:embed all:festivals
var FS embed.FS
