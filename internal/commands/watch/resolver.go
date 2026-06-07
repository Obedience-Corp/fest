package watch

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

var errWatchPickerCancelled = errors.New("festival picker cancelled")

type targetResolver struct {
	getwd           func() (string, error)
	resolveSelector func(context.Context, string, string) (string, error)
	findFestival    func(string) (string, error)
	findFestivals   func(string) (string, error)
	resolveLink     func(context.Context, string) (string, error)
	canPickFestival func() bool
	pickFestival    func(context.Context, string, shared.FestivalPickerOptions) (string, error)
	detectFestival  func(context.Context, string) (*show.FestivalInfo, error)
}

func defaultResolver() targetResolver {
	return targetResolver{
		getwd:           os.Getwd,
		resolveSelector: shared.ResolveFestivalSelector,
		findFestival:    defaultFindFestivalRoot,
		findFestivals:   workspace.FindFestivals,
		resolveLink:     defaultResolveLinkedFestivalPath,
		canPickFestival: defaultCanPickFestival,
		pickFestival:    shared.PickFestivalPath,
		detectFestival: func(ctx context.Context, path string) (*show.FestivalInfo, error) {
			return show.DetectCurrentFestival(ctx, path, "")
		},
	}
}

func (r targetResolver) resolve(ctx context.Context, selector string) (*show.FestivalInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cwd, err := r.getwd()
	if err != nil {
		return nil, errors.IO("getting current directory", err)
	}

	selector = strings.TrimSpace(selector)
	if selector != "" {
		path, err := r.resolveSelector(ctx, cwd, selector)
		if err != nil {
			return nil, err
		}
		return r.detect(ctx, path)
	}

	if path, err := r.findFestival(cwd); err != nil {
		return nil, err
	} else if path != "" {
		if festival, found, err := r.tryDetect(ctx, path); err != nil {
			return nil, err
		} else if found {
			return festival, nil
		}
	}

	if path, err := r.resolveLink(ctx, cwd); err != nil {
		return nil, err
	} else if path != "" {
		if festival, found, err := r.tryDetect(ctx, path); err != nil {
			return nil, err
		} else if found {
			return festival, nil
		}
	}

	festivalsDir, err := r.findFestivals(cwd)
	if err != nil {
		return nil, err
	}
	if festivalsDir == "" {
		return nil, noWatchTargetError()
	}
	if !r.canPickFestival() {
		return nil, noWatchTargetError()
	}

	path, err := r.pickFestival(ctx, festivalsDir, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        pickerStatuses(cwd, festivalsDir),
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, errWatchPickerCancelled
	}
	return r.detect(ctx, path)
}

func (r targetResolver) tryDetect(ctx context.Context, path string) (*show.FestivalInfo, bool, error) {
	festival, err := r.detectFestival(ctx, path)
	if err != nil {
		if isFestivalNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if festival == nil {
		return nil, false, nil
	}
	return festival, true, nil
}

func (r targetResolver) detect(ctx context.Context, path string) (*show.FestivalInfo, error) {
	festival, err := r.detectFestival(ctx, path)
	if err != nil {
		return nil, err
	}
	if festival == nil {
		return nil, errors.NotFound("festival").WithField("path", path)
	}
	return festival, nil
}

func defaultFindFestivalRoot(cwd string) (string, error) {
	path, err := findFestivalRootByMarkers(cwd)
	if err != nil {
		if isFestivalNotFound(err) {
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

func defaultResolveLinkedFestivalPath(ctx context.Context, cwd string) (string, error) {
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

func defaultCanPickFestival() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

func isWatchPickerCancelled(err error) bool {
	return stderrors.Is(err, errWatchPickerCancelled)
}

func isFestivalNotFound(err error) bool {
	var structured *errors.Error
	return stderrors.As(err, &structured) && structured.Code == errors.ErrCodeNotFound
}

// pickerStatuses narrows the watch picker to the working status dir the user is
// inside, else all working statuses. Dungeon is terminal, so never offered.
func pickerStatuses(cwd, festivalsDir string) []string {
	if narrowed := preferredPickerStatuses(cwd, festivalsDir); len(narrowed) > 0 {
		return narrowed
	}
	return id.WorkingStatusDirectories
}

func preferredPickerStatuses(cwd, festivalsDir string) []string {
	rel, err := filepath.Rel(festivalsDir, cwd)
	if err != nil || rel == "." || rel == ".." {
		return nil
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") {
		return nil
	}
	for _, status := range id.WorkingStatusDirectories {
		status = filepath.ToSlash(id.ResolveStatusPath(status))
		if rel == status || strings.HasPrefix(rel, status+"/") {
			return []string{status}
		}
	}
	return nil
}

func noWatchTargetError() error {
	return errors.Validation("festival could not be resolved from this directory").
		WithHint("run from a festival, a linked project, or a campaign workspace with an interactive terminal")
}
