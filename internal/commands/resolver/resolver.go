// Package resolver provides the shared festival target resolver used by fest
// watch and fest promote. It resolves a festival from four sources, in
// priority order:
//
//  1. Explicit selector (festival name or logical ID)
//  2. Festival marker found by walking up from the current directory
//  3. Festival linked to the current directory via navigation.json
//  4. Interactive picker (interactive terminals only)
//
// This package exists separately from shared to avoid an import cycle:
// show already imports shared, so the resolver (which needs both show types
// and shared picker functions) lives here rather than in shared.
package resolver

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"golang.org/x/term"
)

// ErrPickerCancelled is returned by Resolve when the user cancels the
// interactive festival picker. Callers that treat cancellation as success (not
// an error to surface) test for this sentinel via errors.Is.
var ErrPickerCancelled = errors.New("festival picker cancelled")

// ResolveSource identifies how a festival was resolved by Resolve.
type ResolveSource int

const (
	// ResolveSourceSelector: an explicit positional selector resolved the festival.
	ResolveSourceSelector ResolveSource = iota
	// ResolveSourceContext: the current directory is inside (or linked to) a festival.
	ResolveSourceContext
	// ResolveSourcePicker: the user selected a festival from the interactive picker.
	ResolveSourcePicker
)

// TargetResolver resolves a festival from four sources, in priority order:
//
//  1. Explicit selector (festival name or logical ID)
//  2. Festival marker found by walking up from the current directory
//  3. Festival linked to the current directory via navigation.json
//  4. Interactive picker (interactive terminals only)
//
// This is the single resolution implementation shared by fest watch and
// fest promote, extracted from the original watch-only targetResolver so both
// commands agree on call-from-anywhere behavior.
//
// Fields are exported so callers in other packages can construct custom
// resolvers for testing (see DefaultTargetResolver for the production wiring).
type TargetResolver struct {
	Getwd           func() (string, error)
	ResolveSelector func(context.Context, string, string) (string, error)
	FindFestival    func(string) (string, error)
	FindFestivals   func(string) (string, error)
	ResolveLink     func(context.Context, string) (string, error)
	CanPickFestival func() bool
	PickFestival    func(context.Context, string, shared.FestivalPickerOptions) (string, error)
	PickerOptions   func(cwd, festivalsDir string) shared.FestivalPickerOptions
	DetectFestival  func(context.Context, string) (*show.FestivalInfo, error)
}

// TargetResolverOptions configures a TargetResolver built by
// DefaultTargetResolver. Every field has a sensible default.
type TargetResolverOptions struct {
	// PickerOptions returns the picker configuration for a given cwd and
	// festivals directory. The default (and both watch and promote) narrows
	// to the status directory the user is browsing, then falls back to the
	// working statuses (active/ready/planning). When nil, the resolver
	// applies DefaultPickerOptions.
	PickerOptions func(cwd, festivalsDir string) shared.FestivalPickerOptions
}

// DefaultTargetResolver returns a TargetResolver wired to the production
// dependencies, configured by opts. A zero-value opts yields DefaultPickerOptions
// (cwd-scoped status narrowing, then the working statuses).
func DefaultTargetResolver(opts TargetResolverOptions) TargetResolver {
	r := TargetResolver{
		Getwd:           os.Getwd,
		ResolveSelector: shared.ResolveFestivalSelector,
		FindFestival:    DefaultFindFestivalRoot,
		FindFestivals:   workspace.FindFestivals,
		ResolveLink:     DefaultResolveLinkedFestivalPath,
		CanPickFestival: DefaultCanPickFestival,
		PickFestival:    shared.PickFestivalPath,
		PickerOptions:   opts.PickerOptions,
		DetectFestival: func(ctx context.Context, path string) (*show.FestivalInfo, error) {
			return show.DetectCurrentFestival(ctx, path, "")
		},
	}
	if r.PickerOptions == nil {
		r.PickerOptions = DefaultPickerOptions
	}
	return r
}

// Resolve resolves the festival for the given selector (which may be empty).
// When no selector is provided, it falls through marker walk-up, linked
// project resolution, and finally the interactive picker. A picker
// cancellation is reported as ErrPickerCancelled. The returned ResolveSource
// indicates which tier resolved the festival.
func (r TargetResolver) Resolve(ctx context.Context, selector string) (*show.FestivalInfo, ResolveSource, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	cwd, err := r.Getwd()
	if err != nil {
		return nil, 0, errors.IO("getting current directory", err)
	}

	selector = strings.TrimSpace(selector)
	if selector != "" {
		path, err := r.ResolveSelector(ctx, cwd, selector)
		if err != nil {
			return nil, 0, err
		}
		festival, err := r.detect(ctx, path)
		if err != nil {
			return nil, 0, err
		}
		return festival, ResolveSourceSelector, nil
	}

	if path, err := r.FindFestival(cwd); err != nil {
		return nil, 0, err
	} else if path != "" {
		if festival, found, err := r.tryDetect(ctx, path); err != nil {
			return nil, 0, err
		} else if found {
			return festival, ResolveSourceContext, nil
		}
	}

	if path, err := r.ResolveLink(ctx, cwd); err != nil {
		return nil, 0, err
	} else if path != "" {
		if festival, found, err := r.tryDetect(ctx, path); err != nil {
			return nil, 0, err
		} else if found {
			return festival, ResolveSourceContext, nil
		}
	}

	festivalsDir, err := r.FindFestivals(cwd)
	if err != nil {
		return nil, 0, err
	}
	if festivalsDir == "" {
		return nil, 0, NoTargetError()
	}
	if !r.CanPickFestival() {
		return nil, 0, NoTargetError()
	}

	path, err := r.PickFestival(ctx, festivalsDir, r.PickerOptions(cwd, festivalsDir))
	if err != nil {
		return nil, 0, err
	}
	if path == "" {
		return nil, 0, ErrPickerCancelled
	}
	festival, err := r.detect(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	return festival, ResolveSourcePicker, nil
}

func (r TargetResolver) tryDetect(ctx context.Context, path string) (*show.FestivalInfo, bool, error) {
	festival, err := r.DetectFestival(ctx, path)
	if err != nil {
		if IsFestivalNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if festival == nil {
		return nil, false, nil
	}
	return festival, true, nil
}

func (r TargetResolver) detect(ctx context.Context, path string) (*show.FestivalInfo, error) {
	festival, err := r.DetectFestival(ctx, path)
	if err != nil {
		return nil, err
	}
	if festival == nil {
		return nil, errors.NotFound("festival").WithField("path", path)
	}
	return festival, nil
}

// IsPickerCancelled reports whether err is a picker cancellation sentinel.
func IsPickerCancelled(err error) bool {
	return stderrors.Is(err, ErrPickerCancelled)
}

// NoTargetError returns the structured NotFound("festival") error shown when a
// festival cannot be resolved from the current directory and the terminal is
// non-interactive or there is no festivals directory to pick from. Callers
// and agents branch on errors.ErrCodeNotFound.
func NoTargetError() error {
	return errors.NotFound("festival").
		WithHint("run from a festival, a linked project, or a campaign workspace with an interactive terminal")
}

// IsFestivalNotFound reports whether err is a structured NotFound error, used
// to fall through to the next resolution tier rather than surfacing a hard
// failure.
func IsFestivalNotFound(err error) bool {
	var structured *errors.Error
	return stderrors.As(err, &structured) && structured.Code == errors.ErrCodeNotFound
}

// DefaultFindFestivalRoot walks up from cwd looking for festival marker files.
// Returns ("", nil) when no festival root is found.
func DefaultFindFestivalRoot(cwd string) (string, error) {
	path, err := findFestivalRootByMarkers(cwd)
	if err != nil {
		if IsFestivalNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return path, nil
}

func findFestivalRootByMarkers(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", errors.Wrap(err, "resolving absolute path").WithField("start_dir", startDir)
	}

	for {
		if hasFestivalMarker(dir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.NotFound("festival root").
		WithField("start_dir", startDir)
}

func hasFestivalMarker(dir string) bool {
	for _, marker := range []string{
		show.FestivalGoalFile,
		show.FestivalOverviewFile,
		show.FestivalConfigFile,
	} {
		if info, err := os.Stat(filepath.Join(dir, marker)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// DefaultResolveLinkedFestivalPath resolves a festival via navigation.json link
// for the current path, supporting linked project directories (worktrees).
func DefaultResolveLinkedFestivalPath(ctx context.Context, cwd string) (string, error) {
	nav, err := navigation.LoadNavigation()
	if err != nil {
		return "", nil
	}
	festivalName := nav.FindFestivalForPath(cwd)
	if festivalName == "" {
		return "", nil
	}
	if link, ok := nav.GetLink(festivalName); ok && link.FestivalPath != "" {
		return link.FestivalPath, nil
	}
	return shared.ResolveFestivalSelector(ctx, cwd, festivalName)
}

// DefaultCanPickFestival reports whether the interactive picker is available
// (both stdin and stderr are terminals).
func DefaultCanPickFestival() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

// DefaultPickerOptions is the default picker configuration used when a
// TargetResolverOptions does not supply its own. It narrows to the working
// status directory the user is browsing (so festivals/ready prefers ready),
// and falls back to the full working set (active/ready/planning) ordered by
// status then recency. Ritual and dungeon directories are not picker targets.
func DefaultPickerOptions(cwd, festivalsDir string) shared.FestivalPickerOptions {
	working := shared.WorkingFestivalPickerStatuses
	preferred := working
	if narrowed := PreferredPickerStatuses(cwd, festivalsDir, working); len(narrowed) > 0 {
		preferred = narrowed
	}
	return shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        preferred,
		FallbackStatuses:         working,
		OrderByStatusThenRecency: true,
	}
}

// PreferredPickerStatuses returns the working status directory the user is
// currently inside, if cwd is under festivalsDir in one of allowed. An empty
// result means "do not narrow" (campaign root, dungeon, ritual, or a path
// outside festivals/).
func PreferredPickerStatuses(cwd, festivalsDir string, allowed []string) []string {
	if cwd == "" || festivalsDir == "" {
		return nil
	}
	rel, err := filepath.Rel(festivalsDir, cwd)
	if err != nil || rel == "." || rel == ".." {
		return nil
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") {
		return nil
	}
	for _, status := range allowed {
		status = filepath.ToSlash(id.ResolveStatusPath(status))
		if rel == status || strings.HasPrefix(rel, status+"/") {
			return []string{status}
		}
	}
	return nil
}

// CanonicalPath resolves symlinks and returns an absolute, cleaned path. It is
// the shared version of the watch cycle's canonicalWatchPath helper.
func CanonicalPath(p string) string {
	if p == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return filepath.Clean(p)
}
