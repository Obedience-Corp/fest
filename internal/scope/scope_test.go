package scope

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// normalizePath resolves symlinks to handle macOS /var -> /private/var differences
func normalizePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func TestCommandScope_String(t *testing.T) {
	tests := []struct {
		scope CommandScope
		want  string
	}{
		{Global, "global"},
		{Workspace, "workspace"},
		{Festival, "festival"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.scope.String(); got != tt.want {
				t.Errorf("CommandScope.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolve_GlobalScope(t *testing.T) {
	t.Run("global scope returns nil immediately", func(t *testing.T) {
		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": string(Global),
			},
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("global scope works from any directory", func(t *testing.T) {
		// Change to a random temp directory with no festivals
		dir := t.TempDir()
		oldCwd, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(oldCwd) }()

		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": string(Global),
			},
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err != nil {
			t.Errorf("expected nil error for global scope, got %v", err)
		}
	})
}

func TestResolve_WorkspaceScope(t *testing.T) {
	t.Run("workspace scope finds festivals directory", func(t *testing.T) {
		// Create temp directory with festivals/.festival structure
		dir := t.TempDir()
		festivalsDir := filepath.Join(dir, "festivals")
		dotFestivalDir := filepath.Join(festivalsDir, ".festival")
		if err := os.MkdirAll(dotFestivalDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		oldCwd, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(oldCwd) }()

		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": string(Workspace),
			},
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		// Verify workspace was injected into context
		ws, ok := WorkspaceFrom(cmd.Context())
		if !ok {
			t.Error("expected workspace in context")
		}
		// Normalize both paths to handle macOS /var -> /private/var symlink
		gotPath := normalizePath(ws.FestivalsPath)
		wantPath := normalizePath(festivalsDir)
		if gotPath != wantPath {
			t.Errorf("got festivals path %q, want %q", gotPath, wantPath)
		}
	})

	t.Run("workspace scope finds festivals from subdirectory", func(t *testing.T) {
		// Create temp directory with festivals/.festival structure
		dir := t.TempDir()
		festivalsDir := filepath.Join(dir, "festivals")
		dotFestivalDir := filepath.Join(festivalsDir, ".festival")
		subDir := filepath.Join(dir, "projects", "myproject")
		if err := os.MkdirAll(dotFestivalDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		oldCwd, _ := os.Getwd()
		if err := os.Chdir(subDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(oldCwd) }()

		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": string(Workspace),
			},
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		// Verify workspace was injected into context
		ws, ok := WorkspaceFrom(cmd.Context())
		if !ok {
			t.Error("expected workspace in context")
		}
		// Normalize both paths to handle macOS /var -> /private/var symlink
		gotPath := normalizePath(ws.FestivalsPath)
		wantPath := normalizePath(festivalsDir)
		if gotPath != wantPath {
			t.Errorf("got festivals path %q, want %q", gotPath, wantPath)
		}
	})

	t.Run("workspace scope fails without festivals", func(t *testing.T) {
		dir := t.TempDir()
		oldCwd, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(oldCwd) }()

		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": string(Workspace),
			},
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not in a fest workspace") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})
}

func TestResolve_FestivalScope(t *testing.T) {
	t.Run("festival scope resolves from inside festival", func(t *testing.T) {
		// Create temp directory with full festival structure
		dir := t.TempDir()
		festivalsDir := filepath.Join(dir, "festivals")
		dotFestivalDir := filepath.Join(festivalsDir, ".festival")
		festDir := filepath.Join(festivalsDir, "active", "my-fest-F0001")
		if err := os.MkdirAll(dotFestivalDir, 0755); err != nil {
			t.Fatalf("failed to create .festival directory: %v", err)
		}
		if err := os.MkdirAll(festDir, 0755); err != nil {
			t.Fatalf("failed to create festival directory: %v", err)
		}
		// Create fest.yaml marker (this is what FindFestivalRoot looks for)
		configPath := filepath.Join(festDir, "fest.yaml")
		if err := os.WriteFile(configPath, []byte("name: my-fest-F0001"), 0644); err != nil {
			t.Fatalf("failed to create fest.yaml: %v", err)
		}

		oldCwd, _ := os.Getwd()
		if err := os.Chdir(festDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(oldCwd) }()

		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": string(Festival),
			},
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		// Verify workspace and festival were injected into context
		ws, ok := WorkspaceFrom(cmd.Context())
		if !ok {
			t.Error("expected workspace in context")
		}
		gotFestivalsPath := normalizePath(ws.FestivalsPath)
		wantFestivalsPath := normalizePath(festivalsDir)
		if gotFestivalsPath != wantFestivalsPath {
			t.Errorf("got festivals path %q, want %q", gotFestivalsPath, wantFestivalsPath)
		}

		fest, ok := FestivalFrom(cmd.Context())
		if !ok {
			t.Error("expected festival in context")
		}
		gotFestPath := normalizePath(fest)
		wantFestPath := normalizePath(festDir)
		if gotFestPath != wantFestPath {
			t.Errorf("got festival path %q, want %q", gotFestPath, wantFestPath)
		}
	})

	t.Run("festival scope fails from workspace root", func(t *testing.T) {
		dir := t.TempDir()
		festivalsDir := filepath.Join(dir, "festivals")
		dotFestivalDir := filepath.Join(festivalsDir, ".festival")
		if err := os.MkdirAll(dotFestivalDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		oldCwd, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(oldCwd) }()

		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": string(Festival),
			},
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not inside a festival") {
			t.Errorf("error %q does not contain expected message", err.Error())
		}
	})

	t.Run("festival scope with --festival flag defers resolution", func(t *testing.T) {
		// Create workspace but not a festival
		dir := t.TempDir()
		festivalsDir := filepath.Join(dir, "festivals")
		dotFestivalDir := filepath.Join(festivalsDir, ".festival")
		if err := os.MkdirAll(dotFestivalDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		oldCwd, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(oldCwd) }()

		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": string(Festival),
			},
		}
		cmd.SetContext(context.Background())
		// Add --festival flag with value
		cmd.Flags().String("festival", "/some/explicit/path", "")

		err := Resolve(cmd)
		if err != nil {
			t.Errorf("expected nil error when --festival flag is set, got %v", err)
		}

		// Workspace should be resolved, but festival should NOT be in context
		// (it will be resolved by the command using the flag value)
		ws, ok := WorkspaceFrom(cmd.Context())
		if !ok {
			t.Error("expected workspace in context")
		}
		gotPath := normalizePath(ws.FestivalsPath)
		wantPath := normalizePath(festivalsDir)
		if gotPath != wantPath {
			t.Errorf("got festivals path %q, want %q", gotPath, wantPath)
		}

		_, ok = FestivalFrom(cmd.Context())
		if ok {
			t.Error("expected no festival in context when flag is used")
		}
	})
}

func TestResolve_DefaultScope(t *testing.T) {
	t.Run("default scope is workspace when no annotation", func(t *testing.T) {
		dir := t.TempDir()
		festivalsDir := filepath.Join(dir, "festivals")
		dotFestivalDir := filepath.Join(festivalsDir, ".festival")
		if err := os.MkdirAll(dotFestivalDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		oldCwd, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(oldCwd) }()

		cmd := &cobra.Command{
			// No annotations - should default to workspace
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err != nil {
			t.Errorf("expected nil error for default workspace scope, got %v", err)
		}

		// Should have workspace context
		_, ok := WorkspaceFrom(cmd.Context())
		if !ok {
			t.Error("expected workspace in context for default scope")
		}
	})
}

func TestResolve_UnknownScope(t *testing.T) {
	t.Run("unknown scope returns error", func(t *testing.T) {
		cmd := &cobra.Command{
			Annotations: map[string]string{
				"scope": "invalid",
			},
		}
		cmd.SetContext(context.Background())

		err := Resolve(cmd)
		if err == nil {
			t.Error("expected error for unknown scope")
		}
		if !strings.Contains(err.Error(), "unknown scope") {
			t.Errorf("error %q does not mention unknown scope", err.Error())
		}
	})
}

func TestWorkspaceFrom(t *testing.T) {
	t.Run("returns workspace when set", func(t *testing.T) {
		ws := &WorkspaceInfo{
			Root:          "/path/to/campaign",
			FestivalsPath: "/path/to/campaign/festivals",
			Type:          WorkspaceTypeCampaign,
		}
		ctx := WithWorkspace(context.Background(), ws)

		got, ok := WorkspaceFrom(ctx)
		if !ok {
			t.Fatal("expected workspace in context")
		}
		if got.Root != ws.Root {
			t.Errorf("got root %q, want %q", got.Root, ws.Root)
		}
		if got.FestivalsPath != ws.FestivalsPath {
			t.Errorf("got festivals path %q, want %q", got.FestivalsPath, ws.FestivalsPath)
		}
		if got.Type != ws.Type {
			t.Errorf("got type %q, want %q", got.Type, ws.Type)
		}
	})

	t.Run("returns false when not set", func(t *testing.T) {
		_, ok := WorkspaceFrom(context.Background())
		if ok {
			t.Error("expected no workspace in context")
		}
	})
}

func TestFestivalFrom(t *testing.T) {
	t.Run("returns festival path when set", func(t *testing.T) {
		want := "/path/to/festival"
		ctx := WithFestival(context.Background(), want)

		got, ok := FestivalFrom(ctx)
		if !ok {
			t.Fatal("expected festival in context")
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("returns false when not set", func(t *testing.T) {
		_, ok := FestivalFrom(context.Background())
		if ok {
			t.Error("expected no festival in context")
		}
	})
}

func TestWithWorkspace(t *testing.T) {
	ws := &WorkspaceInfo{
		Root:          "/test/root",
		FestivalsPath: "/test/root/festivals",
		Type:          WorkspaceTypeStandalone,
	}

	ctx := WithWorkspace(context.Background(), ws)

	// Verify the value can be retrieved
	got, ok := WorkspaceFrom(ctx)
	if !ok {
		t.Fatal("expected workspace in context after WithWorkspace")
	}
	if got != ws {
		t.Error("retrieved workspace does not match original")
	}
}

func TestWithFestival(t *testing.T) {
	path := "/test/festival/path"

	ctx := WithFestival(context.Background(), path)

	// Verify the value can be retrieved
	got, ok := FestivalFrom(ctx)
	if !ok {
		t.Fatal("expected festival in context after WithFestival")
	}
	if got != path {
		t.Errorf("got %q, want %q", got, path)
	}
}

func TestWorkspaceType(t *testing.T) {
	t.Run("campaign type", func(t *testing.T) {
		if WorkspaceTypeCampaign != "campaign" {
			t.Errorf("expected %q, got %q", "campaign", WorkspaceTypeCampaign)
		}
	})

	t.Run("standalone type", func(t *testing.T) {
		if WorkspaceTypeStandalone != "standalone" {
			t.Errorf("expected %q, got %q", "standalone", WorkspaceTypeStandalone)
		}
	})
}

func TestIsValidFestival(t *testing.T) {
	t.Run("valid with FESTIVAL_GOAL.md", func(t *testing.T) {
		dir := t.TempDir()
		goalPath := filepath.Join(dir, "FESTIVAL_GOAL.md")
		if err := os.WriteFile(goalPath, []byte("# Goal"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		if !isValidFestival(dir) {
			t.Error("expected festival with FESTIVAL_GOAL.md to be valid")
		}
	})

	t.Run("valid with FESTIVAL_OVERVIEW.md", func(t *testing.T) {
		dir := t.TempDir()
		overviewPath := filepath.Join(dir, "FESTIVAL_OVERVIEW.md")
		if err := os.WriteFile(overviewPath, []byte("# Overview"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		if !isValidFestival(dir) {
			t.Error("expected festival with FESTIVAL_OVERVIEW.md to be valid")
		}
	})

	t.Run("valid with fest.yaml", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "fest.yaml")
		if err := os.WriteFile(configPath, []byte("name: test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		if !isValidFestival(dir) {
			t.Error("expected festival with fest.yaml to be valid")
		}
	})

	t.Run("invalid with no markers", func(t *testing.T) {
		dir := t.TempDir()

		if isValidFestival(dir) {
			t.Error("expected empty directory to be invalid festival")
		}
	})
}
