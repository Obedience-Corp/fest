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

func orderWatchPickerCandidates(candidates []FestivalPickCandidate) []FestivalPickCandidate {
	ordered := make([]FestivalPickCandidate, len(candidates))
	copy(ordered, candidates)
	populateCandidateModTimes(ordered)
	sortWatchPickerCandidates(ordered)
	return ordered
}

func sortWatchPickerCandidates(candidates []FestivalPickCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		return lessWatchPickerCandidate(candidates[i], candidates[j])
	})
}

func lessWatchPickerCandidate(a, b FestivalPickCandidate) bool {
	if ra, rb := watchStatusRank(a.Status), watchStatusRank(b.Status); ra != rb {
		return ra < rb
	}
	if !a.ModTime.Equal(b.ModTime) {
		return a.ModTime.After(b.ModTime)
	}
	return a.Name < b.Name
}

func watchStatusRank(status string) int {
	if rank, ok := watchPickerStatusRank[id.ResolveStatusPath(status)]; ok {
		return rank
	}
	return len(watchPickerStatusRank)
}

func populateCandidateModTimes(candidates []FestivalPickCandidate) {
	for i := range candidates {
		if candidates[i].StatusDirectory {
			continue
		}
		candidates[i].ModTime = latestModTime(candidates[i].Path)
	}
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
