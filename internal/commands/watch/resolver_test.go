package watch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func TestTargetResolverSelectorWinsOverContext(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/work/project", &calls)
	resolver.resolveSelector = func(_ context.Context, cwd, selector string) (string, error) {
		calls = append(calls, "selector:"+cwd+":"+selector)
		return "/festivals/active/selector-FS0001", nil
	}
	resolver.findFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "/festivals/active/direct-FD0001", nil
	}
	resolver.resolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "/festivals/active/link-FL0001", nil
	}

	festival, err := resolver.resolve(context.Background(), " selector ")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if festival.Path != "/festivals/active/selector-FS0001" {
		t.Fatalf("resolved path = %q", festival.Path)
	}
	if !reflect.DeepEqual(calls, []string{"selector:/work/project:selector", "detect:/festivals/active/selector-FS0001"}) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestTargetResolverDirectFestivalWinsOverLink(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign/festivals/active/direct-FD0001/001_IMPLEMENT", &calls)
	resolver.findFestival = func(cwd string) (string, error) {
		calls = append(calls, "direct:"+cwd)
		return "/campaign/festivals/active/direct-FD0001", nil
	}
	resolver.resolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "/campaign/festivals/active/link-FL0001", nil
	}

	festival, err := resolver.resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if festival.Path != "/campaign/festivals/active/direct-FD0001" {
		t.Fatalf("resolved path = %q", festival.Path)
	}
	if !reflect.DeepEqual(calls, []string{
		"direct:/campaign/festivals/active/direct-FD0001/001_IMPLEMENT",
		"detect:/campaign/festivals/active/direct-FD0001",
	}) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestTargetResolverLinkedProjectFallback(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign/projects/fest/internal", &calls)
	resolver.findFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "", nil
	}
	resolver.resolveLink = func(_ context.Context, cwd string) (string, error) {
		calls = append(calls, "link:"+cwd)
		return "/campaign/festivals/active/link-FL0001", nil
	}

	festival, err := resolver.resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if festival.Path != "/campaign/festivals/active/link-FL0001" {
		t.Fatalf("resolved path = %q", festival.Path)
	}
	if !reflect.DeepEqual(calls, []string{"direct", "link:/campaign/projects/fest/internal", "detect:/campaign/festivals/active/link-FL0001"}) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestTargetResolverInvalidDirectContextFallsBackToLink(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign/festivals/active/stale-FS0001", &calls)
	resolver.findFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "/campaign/festivals/active/stale-FS0001", nil
	}
	resolver.resolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "/campaign/festivals/active/link-FL0001", nil
	}
	resolver.detectFestival = func(_ context.Context, path string) (*show.FestivalInfo, error) {
		calls = append(calls, "detect:"+path)
		if strings.Contains(path, "stale") {
			return nil, festerrors.NotFound("festival")
		}
		return &show.FestivalInfo{Path: path}, nil
	}

	festival, err := resolver.resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if festival.Path != "/campaign/festivals/active/link-FL0001" {
		t.Fatalf("resolved path = %q", festival.Path)
	}
	if !reflect.DeepEqual(calls, []string{
		"direct",
		"detect:/campaign/festivals/active/stale-FS0001",
		"link",
		"detect:/campaign/festivals/active/link-FL0001",
	}) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestTargetResolverInvalidLinkContextFallsBackToPicker(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign/projects/fest", &calls)
	resolver.findFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "", nil
	}
	resolver.resolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "/campaign/festivals/active/stale-FS0001", nil
	}
	resolver.findFestivals = func(string) (string, error) {
		calls = append(calls, "workspace")
		return "/campaign/festivals", nil
	}
	resolver.pickFestival = func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
		calls = append(calls, "picker")
		return "/campaign/festivals/active/picked-FP0001", nil
	}
	resolver.detectFestival = func(_ context.Context, path string) (*show.FestivalInfo, error) {
		calls = append(calls, "detect:"+path)
		if strings.Contains(path, "stale") {
			return nil, festerrors.NotFound("festival")
		}
		return &show.FestivalInfo{Path: path}, nil
	}

	festival, err := resolver.resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if festival.Path != "/campaign/festivals/active/picked-FP0001" {
		t.Fatalf("resolved path = %q", festival.Path)
	}
	if !reflect.DeepEqual(calls, []string{
		"direct",
		"link",
		"detect:/campaign/festivals/active/stale-FS0001",
		"workspace",
		"picker",
		"detect:/campaign/festivals/active/picked-FP0001",
	}) {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestTargetResolverPickerUsesWorkspaceContext(t *testing.T) {
	calls := []string{}
	var gotOptions shared.FestivalPickerOptions
	resolver := fakeResolver("/campaign/festivals/active", &calls)
	resolver.findFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "", nil
	}
	resolver.resolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "", nil
	}
	resolver.findFestivals = func(cwd string) (string, error) {
		calls = append(calls, "workspace:"+cwd)
		return "/campaign/festivals", nil
	}
	resolver.pickFestival = func(_ context.Context, festivalsDir string, opts shared.FestivalPickerOptions) (string, error) {
		calls = append(calls, "picker:"+festivalsDir)
		gotOptions = opts
		return "/campaign/festivals/active/picked-FP0001", nil
	}

	festival, err := resolver.resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if festival.Path != "/campaign/festivals/active/picked-FP0001" {
		t.Fatalf("resolved path = %q", festival.Path)
	}
	if gotOptions.IncludeStatusDirectories {
		t.Fatal("watch picker must exclude status-directory targets")
	}
	if !reflect.DeepEqual(gotOptions.PreferredStatuses, []string{"active"}) {
		t.Fatalf("preferred statuses = %#v", gotOptions.PreferredStatuses)
	}
	if !gotOptions.OrderByStatusThenRecency {
		t.Fatal("watch picker must order by status priority then recency")
	}
}

func TestTargetResolverPickerCancellationIsDistinct(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign", &calls)
	resolver.findFestival = func(string) (string, error) { return "", nil }
	resolver.resolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.findFestivals = func(string) (string, error) { return "/campaign/festivals", nil }
	resolver.pickFestival = func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
		return "", nil
	}

	_, err := resolver.resolve(context.Background(), "")
	if !isWatchPickerCancelled(err) {
		t.Fatalf("expected picker cancellation sentinel, got %v", err)
	}
}

func TestTargetResolverNonInteractiveNoContextIsActionable(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign", &calls)
	resolver.findFestival = func(string) (string, error) { return "", nil }
	resolver.resolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.findFestivals = func(string) (string, error) { return "/campaign/festivals", nil }
	resolver.canPickFestival = func() bool { return false }

	_, err := resolver.resolve(context.Background(), "")
	if err == nil {
		t.Fatal("expected no-context error")
	}
	if isWatchPickerCancelled(err) {
		t.Fatalf("non-interactive no-context error must not be cancellation: %v", err)
	}
	if !strings.Contains(err.Error(), "linked project") {
		t.Fatalf("error is not actionable: %v", err)
	}
}

func TestTargetResolverInvalidPickedPathReturnsContext(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign", &calls)
	resolver.findFestival = func(string) (string, error) { return "", nil }
	resolver.resolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.findFestivals = func(string) (string, error) { return "/campaign/festivals", nil }
	resolver.pickFestival = func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
		return "/campaign/festivals/active/not-a-festival", nil
	}
	resolver.detectFestival = func(context.Context, string) (*show.FestivalInfo, error) {
		return nil, nil
	}

	_, err := resolver.resolve(context.Background(), "")
	if err == nil {
		t.Fatal("expected invalid picked path error")
	}
	var structured *festerrors.Error
	if !errors.As(err, &structured) {
		t.Fatalf("expected structured error, got %T: %v", err, err)
	}
	if structured.Fields["path"] != "/campaign/festivals/active/not-a-festival" {
		t.Fatalf("path field = %#v", structured.Fields["path"])
	}
}

func TestPreferredPickerStatusesNarrowsToWorkingStatus(t *testing.T) {
	got := preferredPickerStatuses("/campaign/festivals/active/launch-LW0001", "/campaign/festivals")
	if !reflect.DeepEqual(got, []string{"active"}) {
		t.Fatalf("preferred statuses = %#v, want [active]", got)
	}
}

func TestPreferredPickerStatusesIgnoresDungeon(t *testing.T) {
	// Dungeon festivals are terminal and never watched, so a dungeon cwd must
	// not narrow (or surface) dungeon festivals in the picker.
	got := preferredPickerStatuses("/campaign/festivals/dungeon/completed/2026-05", "/campaign/festivals")
	if got != nil {
		t.Fatalf("preferred statuses for a dungeon cwd = %#v, want nil", got)
	}
}

func TestPreferredPickerStatusesIgnoresRitual(t *testing.T) {
	got := preferredPickerStatuses("/campaign/festivals/ritual/weekly-review-RI-WR0001", "/campaign/festivals")
	if got != nil {
		t.Fatalf("preferred statuses for a ritual cwd = %#v, want nil", got)
	}
}

func TestPickerStatusesFallsBackToWatchableStatuses(t *testing.T) {
	got := pickerStatuses("/campaign/festivals/dungeon/completed/2026-05", "/campaign/festivals")
	if !reflect.DeepEqual(got, watchPickerStatuses) {
		t.Fatalf("picker statuses for a dungeon cwd = %#v, want watchPickerStatuses", got)
	}
}

func TestWatchableStatusesExcludeRitualAndOrderActiveFirst(t *testing.T) {
	want := []string{"active", "ready", "planning"}
	if !reflect.DeepEqual(watchPickerStatuses, want) {
		t.Fatalf("watchPickerStatuses = %#v, want %#v", watchPickerStatuses, want)
	}
	for _, status := range watchPickerStatuses {
		if status == "ritual" || strings.HasPrefix(status, "dungeon") {
			t.Fatalf("watch picker must not surface %q (not a watch target)", status)
		}
	}
}

func fakeResolver(cwd string, calls *[]string) targetResolver {
	return targetResolver{
		getwd: func() (string, error) {
			return cwd, nil
		},
		resolveSelector: func(context.Context, string, string) (string, error) {
			return "", errors.New("unexpected selector resolution")
		},
		findFestival: func(string) (string, error) {
			return "", nil
		},
		findFestivals: func(string) (string, error) {
			return "", nil
		},
		resolveLink: func(context.Context, string) (string, error) {
			return "", nil
		},
		canPickFestival: func() bool {
			return true
		},
		pickFestival: func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
			return "", errors.New("unexpected picker")
		},
		detectFestival: func(_ context.Context, path string) (*show.FestivalInfo, error) {
			*calls = append(*calls, "detect:"+path)
			return &show.FestivalInfo{Path: path}, nil
		},
	}
}
