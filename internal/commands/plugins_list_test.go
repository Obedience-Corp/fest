package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePluginFile(t *testing.T, dir, name string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func isolatePluginEnv(t *testing.T, pathDir string) {
	t.Helper()
	t.Setenv("PATH", pathDir)
	t.Setenv("FEST_CONFIG_DIR", t.TempDir())
}

func runPluginsCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd := newPluginsCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return buf.String()
}

func TestPluginsListEmpty(t *testing.T) {
	isolatePluginEnv(t, t.TempDir())

	out := runPluginsCmd(t)
	if !strings.Contains(out, "No fest plugins found.") {
		t.Fatalf("expected empty-state message, got %q", out)
	}
}

func TestPluginsListSkipsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	writePluginFile(t, dir, "fest-noexec", 0o644)
	isolatePluginEnv(t, dir)

	out := runPluginsCmd(t)
	if strings.Contains(out, "noexec") {
		t.Fatalf("non-executable file listed as plugin: %q", out)
	}
}

func TestPluginsListHuman(t *testing.T) {
	dir := t.TempDir()
	path := writePluginFile(t, dir, "fest-graph", 0o755)
	isolatePluginEnv(t, dir)

	out := runPluginsCmd(t)
	if !strings.Contains(out, "graph") {
		t.Fatalf("expected plugin name in output, got %q", out)
	}
	if !strings.Contains(out, path) {
		t.Fatalf("expected plugin path in output, got %q", out)
	}
}

func TestPluginsListJSON(t *testing.T) {
	dir := t.TempDir()
	writePluginFile(t, dir, "fest-graph", 0o755)
	writePluginFile(t, dir, "fest-export-jira", 0o755)
	isolatePluginEnv(t, dir)

	out := runPluginsCmd(t, "--json")

	var payload struct {
		Plugins []struct {
			Command string `json:"command"`
			Exec    string `json:"exec"`
			Source  string `json:"source"`
			Path    string `json:"path"`
		} `json:"plugins"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out)
	}
	if payload.Count != 2 || len(payload.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got count=%d len=%d", payload.Count, len(payload.Plugins))
	}
	if payload.Plugins[0].Command != "export jira" || payload.Plugins[1].Command != "graph" {
		t.Fatalf("expected sorted commands [export jira, graph], got %+v", payload.Plugins)
	}
	for _, p := range payload.Plugins {
		if p.Source != "path" {
			t.Fatalf("expected source %q, got %q", "path", p.Source)
		}
		if p.Path == "" || p.Exec == "" {
			t.Fatalf("expected non-empty path and exec, got %+v", p)
		}
	}
	if !strings.Contains(out, `"source"`) || !strings.Contains(out, `"path"`) {
		t.Fatalf("expected snake_case keys in JSON output, got %s", out)
	}
}

func TestPluginsListManifestSummaryAndMissingExec(t *testing.T) {
	configDir := t.TempDir()
	pluginsDir := filepath.Join(configDir, "active", "user", "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `version: 1
plugins:
  - command: export jira
    exec: fest-export-jira
    summary: Export festival data to Jira
`
	if err := os.WriteFile(filepath.Join(pluginsDir, "manifest.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("FEST_CONFIG_DIR", configDir)

	out := runPluginsCmd(t)
	if !strings.Contains(out, "Export festival data to Jira") {
		t.Fatalf("expected manifest summary in output, got %q", out)
	}
	if !strings.Contains(out, "exec not found: fest-export-jira") {
		t.Fatalf("expected missing-exec marker, got %q", out)
	}
}
