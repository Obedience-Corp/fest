package shared

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Obedience-Corp/fest/internal/id"
)

// watchPickerStatusRank maps a status to its display priority for the watch
// picker. Lower sorts first. Statuses absent from the map sort after all known
// statuses.
var watchPickerStatusRank = map[string]int{
	"active":   0,
	"ready":    1,
	"planning": 2,
}

// orderWatchPickerCandidates returns candidates sorted by status priority
// (active, ready, planning), then most-recently-modified first, then name.
// It populates each candidate's ModTime from the filesystem before sorting and
// leaves the input slice untouched.
func orderWatchPickerCandidates(candidates []FestivalPickCandidate) []FestivalPickCandidate {
	ordered := make([]FestivalPickCandidate, len(candidates))
	copy(ordered, candidates)
	populateCandidateModTimes(ordered)
	sortWatchPickerCandidates(ordered)
	return ordered
}

// sortWatchPickerCandidates stable-sorts pre-populated candidates in place by the
// watch picker ordering. It performs no filesystem access, so it is the pure,
// directly testable core of the ordering.
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

// latestModTime returns the most recent modification time found within dir
// (recursively, including dir itself). It returns the zero time when dir cannot
// be read, so a candidate is never dropped from the picker on a stat error -- it
// simply sorts last within its status group.
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
