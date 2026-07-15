package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// DungeonDir is the canonical, visible name of the festivals dungeon
	// directory (the terminal archive for completed/archived/someday
	// festivals).
	DungeonDir = "dungeon"

	// HiddenDungeonDir is the hidden spelling of the dungeon directory.
	// Campaigns scaffolded with dungeon_hidden enabled use this spelling.
	HiddenDungeonDir = ".dungeon"
)

// dungeonConflictWarned dedupes the both-spellings-exist warning per
// festivals root for the lifetime of the process, so a single command that
// resolves the dungeon many times (e.g. scanning several status
// directories) only prints the warning once.
var dungeonConflictWarned sync.Map

// ResolveDungeonDir returns the on-disk directory name to use for the
// dungeon under festivalsRoot: whichever of dungeon/ or .dungeon/ actually
// exists. If both exist, the visible spelling wins and a one-time warning is
// printed to stderr. If neither exists yet, it defaults to the visible
// spelling so callers creating a dungeon path get the current behavior.
func ResolveDungeonDir(festivalsRoot string) string {
	visible := isExistingDir(filepath.Join(festivalsRoot, DungeonDir))
	hidden := isExistingDir(filepath.Join(festivalsRoot, HiddenDungeonDir))

	switch {
	case visible && hidden:
		warnDungeonConflict(festivalsRoot)
		return DungeonDir
	case hidden:
		return HiddenDungeonDir
	default:
		return DungeonDir
	}
}

// IsDungeonDirName reports whether name is a recognized dungeon directory
// spelling (visible or hidden). Use this when comparing a real directory
// name discovered while walking the filesystem (e.g. a parent directory
// basename), as opposed to a logical status string.
func IsDungeonDirName(name string) bool {
	return name == DungeonDir || name == HiddenDungeonDir
}

// JoinDungeon resolves the dungeon directory name under festivalsRoot and
// joins any additional path segments onto it. Use this wherever a literal
// filepath.Join(festivalsRoot, "dungeon", ...) was previously hardcoded.
func JoinDungeon(festivalsRoot string, elem ...string) string {
	parts := append([]string{festivalsRoot, ResolveDungeonDir(festivalsRoot)}, elem...)
	return filepath.Join(parts...)
}

// JoinStatus resolves a logical status path (e.g. "active", "dungeon",
// "dungeon/completed") to a filesystem path under festivalsRoot. When status
// is "dungeon" or begins with "dungeon/", the dungeon segment is resolved to
// whichever spelling exists on disk; every other status passes through to a
// plain join. Use this as a drop-in replacement for
// filepath.Join(festivalsRoot, status) wherever status is a dynamic logical
// status string that may reference the dungeon.
func JoinStatus(festivalsRoot, status string) string {
	if status == DungeonDir {
		return JoinDungeon(festivalsRoot)
	}
	if rest, ok := strings.CutPrefix(status, DungeonDir+"/"); ok {
		return JoinDungeon(festivalsRoot, rest)
	}
	return filepath.Join(festivalsRoot, status)
}

func warnDungeonConflict(festivalsRoot string) {
	key := festivalsRoot
	if abs, err := filepath.Abs(festivalsRoot); err == nil {
		key = abs
	}
	if _, loaded := dungeonConflictWarned.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: both %s/ and %s/ exist under %s; using %s/\n",
		DungeonDir, HiddenDungeonDir, festivalsRoot, DungeonDir)
}

func isExistingDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
