package festival

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	tpl "github.com/Obedience-Corp/fest/internal/template"
)

func renderTaskContent(ctx context.Context, tmplRoot string, catalog *tpl.Catalog, mgr *tpl.Manager, loader tpl.Loader, tmplCtx *tpl.Context, name string) (string, error) {
	var content string
	var renderErr error
	if catalog != nil {
		content, renderErr = mgr.RenderByID(ctx, catalog, "TASK", tmplCtx)
	}
	if renderErr != nil || content == "" {
		tpath := filepath.Join(tmplRoot, "tasks", "TASK.md")
		if _, err := os.Stat(tpath); err == nil {
			t, err := loader.Load(ctx, tpath)
			if err != nil {
				return "", errors.Wrap(err, "loading task template")
			}
			if strings.Contains(t.Content, "{{") {
				out, err := mgr.Render(t, tmplCtx)
				if err != nil {
					return "", errors.Wrap(err, "rendering task")
				}
				content = out
			} else {
				content = t.Content
			}
		}
	}
	if content == "" {
		content = fmt.Sprintf("# Task: %s\n\n## Objective\n\n[REPLACE: Describe the task objective]\n\n## Steps\n\n1. [REPLACE: Step 1]\n2. [REPLACE: Step 2]\n\n## Definition of Done\n\n- [ ] [REPLACE: Completion criterion]\n", name)
	}
	return content, nil
}

func finalizeTaskContent(content, taskID, name, parentSequenceID string, newNumber int, tmplCtx *tpl.Context, festivalPath string, skipMarkers bool) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		fm := frontmatter.NewTaskFrontmatter(taskID, name, parentSequenceID, newNumber, frontmatter.AutonomyMedium)
		contentWithFM, fmErr := frontmatter.InjectString(content, fm)
		if fmErr != nil {
			return "", errors.Wrap(fmErr, "injecting frontmatter")
		}
		content = contentWithFM
	}
	if skipMarkers {
		return content, nil
	}
	renderer := tpl.NewRenderer()
	var configMarkers map[string]string
	if festivalPath != "" {
		festCfg, cfgErr := config.LoadFestivalConfig(festivalPath, "")
		if cfgErr == nil && festCfg != nil {
			configMarkers = extractConfigMarkers(festCfg)
		}
	}
	rendered, err := renderer.RenderWithMarkerReplacement(content, tmplCtx, configMarkers)
	if err == nil {
		content = rendered
	}
	return content, nil
}

func buildTaskFileContent(ctx context.Context, absPath, festivalPath, tmplRoot string, catalog *tpl.Catalog, mgr *tpl.Manager, loader tpl.Loader, name string, newNumber int, vars map[string]any, skipMarkers bool) (string, error) {
	tmplCtx, ctxErr := tpl.BuildTaskContext(absPath, festivalPath, newNumber, name)
	if ctxErr != nil {
		tmplCtx = tpl.NewContext()
		tmplCtx.SetTask(newNumber, name)
		tmplCtx.ComputeStructureVariables()
	}
	for k, v := range vars {
		tmplCtx.SetCustom(k, v)
	}

	content, err := renderTaskContent(ctx, tmplRoot, catalog, mgr, loader, tmplCtx, name)
	if err != nil {
		return "", err
	}
	return finalizeTaskContent(content, tpl.FormatTaskID(newNumber, name), name, filepath.Base(absPath), newNumber, tmplCtx, festivalPath, skipMarkers)
}

func previewCreateTask(ctx context.Context, opts *CreateTaskOptions, absPath, festivalPath, tmplRoot string, catalog *tpl.Catalog, mgr *tpl.Manager, loader tpl.Loader, vars map[string]any, skipMarkers bool) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("previewCreateTask")
	}
	planned := make([]string, 0, len(opts.Names))
	combined := &MarkerResult{}
	currentAfter := opts.After
	for _, name := range opts.Names {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled").WithOp("previewCreateTask")
		}
		newNumber := currentAfter + 1
		taskID := tpl.FormatTaskID(newNumber, name)
		content, err := buildTaskFileContent(ctx, absPath, festivalPath, tmplRoot, catalog, mgr, loader, name, newNumber, vars, skipMarkers)
		if err != nil {
			return emitCreateTaskError(opts, err)
		}
		planned = append(planned, taskID)
		part := markerResultFromContent(content)
		combined.Total += part.Total
		combined.Markers = append(combined.Markers, part.Markers...)
		currentAfter = newNumber
	}
	return emitCreateDryRun(opts.JSONOutput, planned, combined)
}
