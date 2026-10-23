package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/response"
	"github.com/Obedience-Corp/fest/internal/version"
	"github.com/Obedience-Corp/obey-shared/updatecheck"
)

const versionCheckSchema = "version-check/v1alpha1"

const (
	suiteSkipNotRelease = "not a festival suite release build"
	suiteSkipNoHome     = "cannot determine home directory"
)

var suiteCheck = updatecheck.Check

// versionCheckInfo is the `fest version --check --json` contract. It matches
// `camp version --check --json` key for key, with fest_version in place of
// camp_version. current_version is the festival suite release this binary
// shipped in; fest_version is fest's own component version.
type versionCheckInfo struct {
	SchemaVersion   string `json:"schema_version"`
	Product         string `json:"product"`
	FestVersion     string `json:"fest_version"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	LatestTag       string `json:"latest_tag,omitempty"`
	Source          string `json:"source"`
	UpdateAvailable bool   `json:"update_available"`
	UpgradeKind     string `json:"upgrade_kind,omitempty"`
	UpgradeCommand  string `json:"upgrade_command,omitempty"`
	CheckedAt       string `json:"checked_at,omitempty"`
	FromCache       bool   `json:"from_cache"`
	RefreshError    string `json:"refresh_error,omitempty"`
	SkipReason      string `json:"skip_reason,omitempty"`
}

// suiteVersionCheck compares this binary's festival suite release against the
// latest one. A build outside a festival release has nothing to compare, so
// it reads no cache and makes no request.
func suiteVersionCheck(ctx context.Context) (versionCheckInfo, error) {
	info := versionCheckInfo{
		SchemaVersion: versionCheckSchema,
		Product:       "fest",
		FestVersion:   version.Version,
		Source:        updatecheck.Source,
	}
	bundle := suiteBundle()
	if bundle == "" {
		info.SkipReason = suiteSkipNotRelease
		return info, nil
	}
	info.CurrentVersion = updatecheck.NormalizeVersion(bundle)
	cachePath, ok := suiteCachePath()
	if !ok {
		info.SkipReason = suiteSkipNoHome
		return info, nil
	}
	res, err := suiteCheck(ctx, bundle, updatecheck.Options{
		CachePath:      cachePath,
		ExecutablePath: suiteExecutable(),
		Fetcher:        suiteFetcher,
	})
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return info, festerrors.Wrap(err, "checking festival suite version")
	}
	info.CurrentVersion = res.CurrentVersion
	info.LatestVersion = res.LatestVersion
	info.LatestTag = res.LatestTag
	if res.Source != "" {
		info.Source = res.Source
	}
	info.UpdateAvailable = res.UpdateAvailable
	if res.UpdateAvailable {
		info.UpgradeKind = res.UpgradeKind
		info.UpgradeCommand = res.UpgradeCommand
	}
	if !res.CheckedAt.IsZero() {
		info.CheckedAt = res.CheckedAt.UTC().Format(time.RFC3339)
	}
	info.FromCache = res.FromCache
	if res.RefreshErr != nil {
		info.RefreshError = res.RefreshErr.Error()
	}
	return info, nil
}

func emitVersionCheck(w io.Writer, info versionCheckInfo, jsonOut bool) error {
	if jsonOut {
		return response.Encode(w, info)
	}
	writeVersionCheck(w, info)
	return nil
}

func writeVersionCheck(w io.Writer, info versionCheckInfo) {
	if info.CurrentVersion == "" {
		_, _ = fmt.Fprintf(w, "fest %s\n", info.FestVersion)
	} else {
		_, _ = fmt.Fprintf(w, "fest %s (festival suite %s)\n", info.FestVersion, info.CurrentVersion)
	}
	switch {
	case info.SkipReason != "":
		_, _ = fmt.Fprintf(w, "festival suite update check skipped: %s\n", info.SkipReason)
	case info.LatestVersion == "":
		_, _ = fmt.Fprintln(w, "festival suite latest: unknown (could not reach GitHub)")
	default:
		latest := info.LatestVersion
		if info.RefreshError != "" {
			latest += " (cached " + info.CheckedAt + "; could not reach GitHub to refresh)"
		}
		_, _ = fmt.Fprintf(w, "festival suite latest: %s\n", latest)
		switch {
		case info.UpdateAvailable:
			_, _ = fmt.Fprintln(w, "update available")
			if info.UpgradeCommand != "" {
				_, _ = fmt.Fprintf(w, "%s: %s\n", suiteActionLabel(info.UpgradeKind), info.UpgradeCommand)
			}
		case updatecheck.UpdateAvailable(info.LatestVersion, info.CurrentVersion):
			_, _ = fmt.Fprintln(w, "newer than the latest festival suite release")
		default:
			_, _ = fmt.Fprintln(w, "up to date")
		}
	}
}

func suiteActionLabel(kind string) string {
	if kind == "" {
		return updatecheck.HintRun
	}
	return kind
}
