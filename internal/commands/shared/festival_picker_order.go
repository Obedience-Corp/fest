package shared

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Obedience-Corp/fest/internal/id"
)

var watchPickerStatusRank = map[string]int{
	"active":   0,
	"ready":    1,
	"planning": 2,
}

type rankedCandidate struct {
	candidate FestivalPickCandidate
	modTime   time.Time
}

func orderWatchPickerCandidates(candidates []FestivalPickCandidate) []FestivalPickCandidate {
	ranked := make([]rankedCandidate, len(candidates))
	for i, c := range candidates {
		ranked[i] = rankedCandidate{candidate: c}
		if !c.StatusDirectory {
			ranked[i].modTime = latestModTime(c.Path)
		}
	}
	sortRankedCandidates(ranked)

	ordered := make([]FestivalPickCandidate, len(ranked))
	for i, r := range ranked {
		ordered[i] = r.candidate
	}
	return ordered
}

func sortRankedCandidates(ranked []rankedCandidate) {
	sort.SliceStable(ranked, func(i, j int) bool {
		return lessRankedCandidate(ranked[i], ranked[j])
	})
}

func lessRankedCandidate(a, b rankedCandidate) bool {
	if ra, rb := watchStatusRank(a.candidate.Status), watchStatusRank(b.candidate.Status); ra != rb {
		return ra < rb
	}
	if !a.modTime.Equal(b.modTime) {
		return a.modTime.After(b.modTime)
	}
	return a.candidate.Name < b.candidate.Name
}

func watchStatusRank(status string) int {
	if rank, ok := watchPickerStatusRank[id.ResolveStatusPath(status)]; ok {
		return rank
	}
	return len(watchPickerStatusRank)
}

func latestModTime(dir string) time.Time {
	var latest time.Time
	walkErr := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		if mt := info.ModTime(); mt.After(latest) {
			latest = mt
		}
		return nil
	})
	if walkErr != nil {
		return time.Time{}
	}
	return latest
}
