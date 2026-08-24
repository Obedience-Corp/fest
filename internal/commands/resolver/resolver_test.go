package resolver

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
	resolver.ResolveSelector = func(_ context.Context, cwd, selector string) (string, error) {
		calls = append(calls, "selector:"+cwd+":"+selector)
		return "/festivals/active/selector-FS0001", nil
	}
	resolver.FindFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "/festivals/active/direct-FD0001", nil
	}
	resolver.ResolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "/festivals/active/link-FL0001", nil
	}

	festival, _, err := resolver.Resolve(context.Background(), " selector ")
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
	resolver.FindFestival = func(cwd string) (string, error) {
		calls = append(calls, "direct:"+cwd)
		return "/campaign/festivals/active/direct-FD0001", nil
	}
	resolver.ResolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "/campaign/festivals/active/link-FL0001", nil
	}

	festival, _, err := resolver.Resolve(context.Background(), "")
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
	resolver.FindFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "", nil
	}
	resolver.ResolveLink = func(_ context.Context, cwd string) (string, error) {
		calls = append(calls, "link:"+cwd)
		return "/campaign/festivals/active/link-FL0001", nil
	}

	festival, _, err := resolver.Resolve(context.Background(), "")
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
	resolver.FindFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "/campaign/festivals/active/stale-FS0001", nil
	}
	resolver.ResolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "/campaign/festivals/active/link-FL0001", nil
	}
	resolver.DetectFestival = func(_ context.Context, path string) (*show.FestivalInfo, error) {
		calls = append(calls, "detect:"+path)
		if strings.Contains(path, "stale") {
			return nil, festerrors.NotFound("festival")
		}
		return &show.FestivalInfo{Path: path}, nil
	}

	festival, _, err := resolver.Resolve(context.Background(), "")
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
	resolver.FindFestival = func(string) (string, error) {
		calls = append(calls, "direct")
		return "", nil
	}
	resolver.ResolveLink = func(context.Context, string) (string, error) {
		calls = append(calls, "link")
		return "/campaign/festivals/active/stale-FS0001", nil
	}
	resolver.FindFestivals = func(string) (string, error) {
		calls = append(calls, "workspace")
		return "/campaign/festivals", nil
	}
	resolver.PickFestival = func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
		calls = append(calls, "picker")
		return "/campaign/festivals/active/picked-FP0001", nil
	}
	resolver.DetectFestival = func(_ context.Context, path string) (*show.FestivalInfo, error) {
		calls = append(calls, "detect:"+path)
		if strings.Contains(path, "stale") {
			return nil, festerrors.NotFound("festival")
		}
		return &show.FestivalInfo{Path: path}, nil
	}

	festival, _, err := resolver.Resolve(context.Background(), "")
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

func TestTargetResolverPickerCancellationIsDistinct(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign", &calls)
	resolver.FindFestival = func(string) (string, error) { return "", nil }
	resolver.ResolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.FindFestivals = func(string) (string, error) { return "/campaign/festivals", nil }
	resolver.PickFestival = func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
		return "", nil
	}

	_, _, err := resolver.Resolve(context.Background(), "")
	if !IsPickerCancelled(err) {
		t.Fatalf("expected picker cancellation sentinel, got %v", err)
	}
}

func TestTargetResolverNonInteractiveNoContextIsActionable(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign", &calls)
	resolver.FindFestival = func(string) (string, error) { return "", nil }
	resolver.ResolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.FindFestivals = func(string) (string, error) { return "/campaign/festivals", nil }
	resolver.CanPickFestival = func() bool { return false }

	_, _, err := resolver.Resolve(context.Background(), "")
	if err == nil {
		t.Fatal("expected no-context error")
	}
	if IsPickerCancelled(err) {
		t.Fatalf("non-interactive no-context error must not be cancellation: %v", err)
	}
	assertNoTargetNotFound(t, err)
}

func TestTargetResolverMissingFestivalsDirIsNotFound(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign", &calls)
	resolver.FindFestival = func(string) (string, error) { return "", nil }
	resolver.ResolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.FindFestivals = func(string) (string, error) { return "", nil }

	_, _, err := resolver.Resolve(context.Background(), "")
	assertNoTargetNotFound(t, err)
}

func assertNoTargetNotFound(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected no-target error")
	}
	var structured *festerrors.Error
	if !errors.As(err, &structured) {
		t.Fatalf("expected structured error, got %T: %v", err, err)
	}
	if structured.Code != festerrors.ErrCodeNotFound {
		t.Fatalf("code = %q, want %q", structured.Code, festerrors.ErrCodeNotFound)
	}
	if !festerrors.Is(err, festerrors.ErrCodeNotFound) {
		t.Fatalf("errors.Is(%q) = false", festerrors.ErrCodeNotFound)
	}
	if structured.Fields["resource"] != "festival" {
		t.Fatalf("resource field = %#v, want festival", structured.Fields["resource"])
	}
	if !strings.Contains(structured.Hint, "linked project") {
		t.Fatalf("hint is not actionable: %q", structured.Hint)
	}
	if !strings.Contains(structured.Hint, "interactive terminal") {
		t.Fatalf("hint should mention interactive terminal, got %q", structured.Hint)
	}
}

func TestPreferredPickerStatusesNarrowsToWorkingStatus(t *testing.T) {
	got := PreferredPickerStatuses("/campaign/festivals/active/launch-LW0001", "/campaign/festivals", shared.WorkingFestivalPickerStatuses)
	if !reflect.DeepEqual(got, []string{"active"}) {
		t.Fatalf("preferred statuses = %#v, want [active]", got)
	}
}

func TestPreferredPickerStatusesNarrowsToReady(t *testing.T) {
	got := PreferredPickerStatuses("/campaign/festivals/ready", "/campaign/festivals", shared.WorkingFestivalPickerStatuses)
	if !reflect.DeepEqual(got, []string{"ready"}) {
		t.Fatalf("preferred statuses = %#v, want [ready]", got)
	}
}

func TestPreferredPickerStatusesIgnoresDungeonAndRitual(t *testing.T) {
	for _, cwd := range []string{
		"/campaign/festivals/dungeon/completed/2026-05",
		"/campaign/festivals/ritual/weekly-review-RI-WR0001",
		"/campaign",
		"/campaign/festivals",
	} {
		got := PreferredPickerStatuses(cwd, "/campaign/festivals", shared.WorkingFestivalPickerStatuses)
		if got != nil {
			t.Fatalf("preferred statuses for %s = %#v, want nil", cwd, got)
		}
	}
}

func TestDefaultPickerOptionsNarrowsToStatusDirectory(t *testing.T) {
	got := DefaultPickerOptions("/campaign/festivals/ready", "/campaign/festivals")
	if !reflect.DeepEqual(got.PreferredStatuses, []string{"ready"}) {
		t.Fatalf("PreferredStatuses = %#v, want [ready]", got.PreferredStatuses)
	}
	if !reflect.DeepEqual(got.FallbackStatuses, shared.WorkingFestivalPickerStatuses) {
		t.Fatalf("FallbackStatuses = %#v, want working statuses", got.FallbackStatuses)
	}
}

func TestDefaultPickerOptionsFromCampaignRootUsesWorkingSet(t *testing.T) {
	got := DefaultPickerOptions("/campaign", "/campaign/festivals")
	if !reflect.DeepEqual(got.PreferredStatuses, shared.WorkingFestivalPickerStatuses) {
		t.Fatalf("PreferredStatuses = %#v, want working statuses", got.PreferredStatuses)
	}
}

func TestTargetResolverPassesCwdScopedPickerOptions(t *testing.T) {
	var got shared.FestivalPickerOptions
	resolver := fakeResolver("/campaign/festivals/ready", &[]string{})
	resolver.FindFestival = func(string) (string, error) { return "", nil }
	resolver.ResolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.FindFestivals = func(string) (string, error) { return "/campaign/festivals", nil }
	resolver.PickerOptions = DefaultPickerOptions
	resolver.PickFestival = func(_ context.Context, _ string, opts shared.FestivalPickerOptions) (string, error) {
		got = opts
		return "/campaign/festivals/ready/beta-FE0002", nil
	}

	_, _, err := resolver.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if !reflect.DeepEqual(got.PreferredStatuses, []string{"ready"}) {
		t.Fatalf("picker PreferredStatuses = %#v, want [ready]", got.PreferredStatuses)
	}
	if !reflect.DeepEqual(got.FallbackStatuses, shared.WorkingFestivalPickerStatuses) {
		t.Fatalf("picker FallbackStatuses = %#v, want working statuses", got.FallbackStatuses)
	}
}

func TestTargetResolverInvalidPickedPathReturnsContext(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign", &calls)
	resolver.FindFestival = func(string) (string, error) { return "", nil }
	resolver.ResolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.FindFestivals = func(string) (string, error) { return "/campaign/festivals", nil }
	resolver.PickFestival = func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
		return "/campaign/festivals/active/not-a-festival", nil
	}
	resolver.DetectFestival = func(context.Context, string) (*show.FestivalInfo, error) {
		return nil, nil
	}

	_, _, err := resolver.Resolve(context.Background(), "")
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

func TestResolveSourceReportsPickerSelection(t *testing.T) {
	calls := []string{}
	resolver := fakeResolver("/campaign", &calls)
	resolver.FindFestival = func(string) (string, error) { return "", nil }
	resolver.ResolveLink = func(context.Context, string) (string, error) { return "", nil }
	resolver.FindFestivals = func(string) (string, error) { return "/campaign/festivals", nil }
	resolver.PickFestival = func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
		return "/campaign/festivals/active/picked-FP0001", nil
	}

	festival, source, err := resolver.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if source != ResolveSourcePicker {
		t.Fatalf("source = %d, want ResolveSourcePicker", source)
	}
	if festival.Path != "/campaign/festivals/active/picked-FP0001" {
		t.Fatalf("resolved path = %q", festival.Path)
	}
}

func fakeResolver(cwd string, calls *[]string) TargetResolver {
	return TargetResolver{
		Getwd: func() (string, error) {
			return cwd, nil
		},
		ResolveSelector: func(context.Context, string, string) (string, error) {
			return "", errors.New("unexpected selector resolution")
		},
		FindFestival: func(string) (string, error) {
			return "", nil
		},
		FindFestivals: func(string) (string, error) {
			return "", nil
		},
		ResolveLink: func(context.Context, string) (string, error) {
			return "", nil
		},
		CanPickFestival: func() bool {
			return true
		},
		PickFestival: func(context.Context, string, shared.FestivalPickerOptions) (string, error) {
			return "", errors.New("unexpected picker")
		},
		PickerOptions: DefaultPickerOptions,
		DetectFestival: func(_ context.Context, path string) (*show.FestivalInfo, error) {
			*calls = append(*calls, "detect:"+path)
			return &show.FestivalInfo{Path: path}, nil
		},
	}
}
