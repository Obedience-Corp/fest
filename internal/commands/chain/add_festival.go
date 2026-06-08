package chain

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
)

var festivalStatusDirs = []string{"planning", "active", "ready", "ritual"}

type resolvedFestival struct {
	ID       string
	Name     string
	Projects []string
	Path     string
}

func resolveFestivalToAdd(ctx context.Context, root, value string) (resolvedFestival, error) {
	if err := ctx.Err(); err != nil {
		return resolvedFestival{}, err
	}
	if value == "" {
		return resolvedFestival{}, errors.Validation("festival is required").
			WithHint("pass --festival <id|name|path> or run from inside a festival or linked project")
	}

	if dir, ok := festivalDirFromPath(value); ok {
		return loadResolvedFestival(dir)
	}

	dir, err := findFestivalDirByIdentifier(ctx, root, value)
	if err != nil {
		return resolvedFestival{}, err
	}
	return loadResolvedFestival(dir)
}

func festivalDirFromPath(value string) (string, bool) {
	if !strings.ContainsAny(value, "/\\") && !strings.HasPrefix(value, ".") {
		return "", false
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		if config.FestivalConfigExists(value) {
			return value, true
		}
		return "", false
	}
	if filepath.Base(value) == config.FestivalConfigFileName {
		return filepath.Dir(value), true
	}
	return "", false
}

func findFestivalDirByIdentifier(ctx context.Context, root, identifier string) (string, error) {
	for _, sub := range festivalStatusDirs {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		dir := filepath.Join(root, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			festDir := filepath.Join(dir, e.Name())
			if !config.FestivalConfigExists(festDir) {
				continue
			}
			cfg, err := config.LoadFestivalConfig(festDir, "")
			if err != nil {
				continue
			}
			if cfg.Metadata.ID == identifier || cfg.Metadata.Name == identifier {
				return festDir, nil
			}
		}
	}
	return "", errors.NotFound("festival").
		WithField("identifier", identifier).
		WithHint("pass --festival as a festival id, name, or path")
}

func loadResolvedFestival(dir string) (resolvedFestival, error) {
	cfg, err := config.LoadFestivalConfig(dir, "")
	if err != nil {
		return resolvedFestival{}, errors.Wrap(err, "loading festival config").
			WithField("path", dir)
	}
	if !cfg.Metadata.HasMetadata() {
		return resolvedFestival{}, errors.Validation("festival has no metadata id").
			WithField("path", dir).
			WithHint("the target festival's fest.yaml must declare metadata.id")
	}
	return resolvedFestival{
		ID:       cfg.Metadata.ID,
		Name:     cfg.Metadata.Name,
		Projects: festivalProjects(cfg),
		Path:     dir,
	}, nil
}

func festivalProjects(cfg *config.FestivalConfig) []string {
	if cfg.ProjectPath == "" {
		return nil
	}
	return []string{cfg.ProjectPath}
}
