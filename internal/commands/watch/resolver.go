package watch

import (
	"context"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

type targetResolver struct {
	getwd           func() (string, error)
	resolveSelector func(context.Context, string, string) (string, error)
	findFestival    func(string) (string, error)
	findFestivals   func(string) (string, error)
	resolveLink     func(context.Context, string) (string, error)
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
		return r.detect(ctx, path)
	}

	if path, err := r.resolveLink(ctx, cwd); err != nil {
		return nil, err
	} else if path != "" {
		return r.detect(ctx, path)
	}

	festivalsDir, err := r.findFestivals(cwd)
	if err != nil {
		return nil, err
	}
	if festivalsDir == "" {
		return nil, noWatchTargetError()
	}

	path, err := r.pickFestival(ctx, festivalsDir, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, noWatchTargetError()
	}
	return r.detect(ctx, path)
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
	path, err := template.FindFestivalRoot(cwd)
	if err != nil {
		return "", nil
	}
	return path, nil
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

func noWatchTargetError() error {
	return errors.NotFound("festival context").
		WithHint("Run from a festival, run from a linked project, or pass a selector like 'fest watch <festival>'")
}
