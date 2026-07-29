package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Obedience-Corp/fest/internal/errors"
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
// printed to stderr. If neither exists yet, it follows the campaign's
// established spelling (see campaignDungeonSpelling) so that a path a caller is
// about to create matches the rest of the campaign — a dungeon_hidden campaign
// creates festivals/.dungeon rather than a stray visible dungeon. When there is
// no campaign signal (a standalone festivals tree, or a campaign whose root
// dungeon is itself absent) it falls back to the visible spelling, preserving
// prior behavior.
//
// Deciding the neither-exists case here rather than in a separate create-only
// helper keeps every JoinDungeon/JoinStatus call site correct without each one
// having to remember to opt into a "for new" variant; a read against a
// not-yet-created dungeon returns a non-existent path under either spelling, so
// following the campaign spelling is harmless for reads and correct for the
// creation paths that flow through the same choke point.
func ResolveDungeonDir(festivalsRoot string) string {
	visible := isExistingDir(filepath.Join(festivalsRoot, DungeonDir))
	hidden := isExistingDir(filepath.Join(festivalsRoot, HiddenDungeonDir))

	switch {
	case visible && hidden:
		warnDungeonConflict(festivalsRoot)
		return DungeonDir
	case hidden:
		return HiddenDungeonDir
	case visible:
		return DungeonDir
	default:
		return campaignDungeonSpelling(festivalsRoot)
	}
}

// campaignDungeonSpelling infers the campaign-wide dungeon spelling from the
// campaign root (the parent of festivalsRoot) for the case where no festivals
// dungeon exists yet. A dungeon_hidden campaign has a hidden root dungeon, so
// its festivals dungeon should be hidden too. When the signal is absent or
// ambiguous — both root spellings present, neither present, or a standalone
// festivals tree with no campaign root — it returns the visible spelling to
// preserve prior behavior.
func campaignDungeonSpelling(festivalsRoot string) string {
	campaignRoot := filepath.Dir(festivalsRoot)
	rootVisible := isExistingDir(filepath.Join(campaignRoot, DungeonDir))
	rootHidden := isExistingDir(filepath.Join(campaignRoot, HiddenDungeonDir))
	if rootHidden && !rootVisible {
		return HiddenDungeonDir
	}
	return DungeonDir
}

// NormalizeNewDungeonSpelling renames a freshly scaffolded visible dungeon/ to
// the hidden .dungeon/ spelling when the campaign is dungeon_hidden, so a newly
// initialized festivals tree matches the rest of the campaign. It exists for
// callers that materialize the dungeon by copying a template tree (which always
// ships the visible spelling) rather than through JoinDungeon, e.g. fest init.
// It is a no-op for visible campaigns, when there is nothing to rename, or when
// the hidden spelling already exists.
func NormalizeNewDungeonSpelling(festivalsRoot string) error {
	if campaignDungeonSpelling(festivalsRoot) != HiddenDungeonDir {
		return nil
	}
	visiblePath := filepath.Join(festivalsRoot, DungeonDir)
	hiddenPath := filepath.Join(festivalsRoot, HiddenDungeonDir)
	if !isExistingDir(visiblePath) || isExistingDir(hiddenPath) {
		return nil
	}
	if err := os.Rename(visiblePath, hiddenPath); err != nil {
		return errors.IO("renaming dungeon to hidden spelling", err).
			WithField("from", visiblePath).
			WithField("to", hiddenPath)
	}
	return nil
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

// CheckDungeonConflict reports an error when both dungeon/ and .dungeon/
// exist under festivalsRoot. A campaign has exactly one dungeon spelling;
// both present at once is a broken migration state, not a supported layout,
// because it lets whichever spelling ResolveDungeonDir prefers silently hide
// festivals filed under the other one.
//
// Call this at the entry of operations whose actual job is to read or write
// the dungeon (list dungeon, promote/complete into a dungeon status, chain
// completion, festival discovery that falls through to "not found"). Do not
// call it from generic multi-status resolution helpers that touch the
// dungeon only incidentally while resolving something else — those keep
// using ResolveDungeonDir's existing prefer-visible fallback and one-time
// warning.
func CheckDungeonConflict(festivalsRoot string) error {
	visible := isExistingDir(filepath.Join(festivalsRoot, DungeonDir))
	hidden := isExistingDir(filepath.Join(festivalsRoot, HiddenDungeonDir))
	if !visible || !hidden {
		return nil
	}
	return errors.Validation("both dungeon/ and .dungeon/ exist").
		WithField("festivalsRoot", festivalsRoot).
		WithField("visible", DungeonDir).
		WithField("hidden", HiddenDungeonDir).
		WithHint("run 'camp dungeon migrate' to consolidate to a single dungeon spelling")
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
