//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Criteria 15 and 15b: fest commit inherits camp's deferred-commit ordering
// guarantee through commitkit, and never itself defers. A `camp intent add`
// queues its bookkeeping commit; a `fest commit` that follows immediately
// must land that queued commit in history BEFORE the festival commit (the
// drain inside commitkit.StageFiles/Commit), return with the queue empty,
// and report a real hash that is HEAD at return.
//
// The camp binary is built from fest's own go.mod dependency, so the exact
// released version fest links against is the version driving the fixture.

var (
	campBuildOnce sync.Once
	campBuildErr  error
	campHostPath  string
)

// ensureCampInContainer builds camp from the module cache (the pinned
// dependency version, cross-compiled for the container) and installs it at
// /usr/local/bin/camp in the shared container.
func ensureCampInContainer(t *testing.T, tc *TestContainer) {
	t.Helper()
	campBuildOnce.Do(func() {
		// The version comes from fest's own go.mod, so the binary driving the
		// fixture is exactly the release fest links against. go install
		// resolves camp's full dependency tree from camp's module, which
		// fest's go.sum deliberately does not carry.
		verCmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}",
			"github.com/Obedience-Corp/camp")
		verOut, err := verCmd.Output()
		if err != nil {
			campBuildErr = &campBuildError{err: err, out: "resolving pinned camp version"}
			return
		}
		version := strings.TrimSpace(string(verOut))

		install := exec.Command("go", "install",
			"github.com/Obedience-Corp/camp/cmd/camp@"+version)
		env := make([]string, 0, len(os.Environ())+3)
		for _, e := range os.Environ() {
			// GOBIN forbids cross-compiled installs; the platform subdir of
			// GOPATH/bin is where a cross-compiled binary lands instead.
			if !strings.HasPrefix(e, "GOBIN=") {
				env = append(env, e)
			}
		}
		install.Env = append(env, "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
		if out, err := install.CombinedOutput(); err != nil {
			campBuildErr = &campBuildError{err: err, out: string(out)}
			return
		}

		gopathOut, err := exec.Command("go", "env", "GOPATH").Output()
		if err != nil {
			campBuildErr = &campBuildError{err: err, out: "resolving GOPATH"}
			return
		}
		gopath := strings.TrimSpace(string(gopathOut))
		campHostPath = filepath.Join(gopath, "bin", "linux_"+runtime.GOARCH, "camp")
		if runtime.GOOS == "linux" {
			campHostPath = filepath.Join(gopath, "bin", "camp")
		}
	})
	if campBuildErr != nil {
		t.Fatalf("building camp from the pinned dependency: %v", campBuildErr)
	}
	// Always install the host-built pinned binary. A preinstalled camp of a
	// different version would silently drive the fixture against the wrong
	// commitkit while fest links v0.4.0-rc.1.
	require.NoError(t, tc.CopyToContainer(campHostPath, "/usr/local/bin/camp"))
	_, err := tc.Exec("chmod", "+x", "/usr/local/bin/camp")
	require.NoError(t, err)
}

type campBuildError struct {
	err error
	out string
}

func (e *campBuildError) Error() string { return e.err.Error() + ":\n" + e.out }

func TestCommitDrain_QueuedBookkeepingLandsBeforeTheFestivalCommit(t *testing.T) {
	tc := GetSharedContainer(t)
	ensureGuardGit(t, tc)
	ensureCampInContainer(t, tc)

	// A real campaign, initialized by the released camp binary the fixture is
	// about: its queue wiring is the thing under test.
	out, err := tc.Exec("sh", "-c",
		"export HOME=/root && cd / && camp init guard-drain -d 'drain fixture' -m 'prove criterion 15'")
	require.NoError(t, err, "camp init should succeed: %s", out)
	camp := "/guard-drain"
	require.NoError(t, tc.WriteFile(camp+"/festivals/active/guard-festival/fest.yaml", guardFestYAML))
	require.NoError(t, tc.WriteFile(camp+"/festivals/active/guard-festival/NOTES.md", "# Progress\n"))
	// Any campaign fest has navigated has these; a bare camp init does not,
	// and fest's scoped staging lists them unconditionally (intent filed:
	// fest-commit-fails-in-a-fresh-campaign). The fixture mirrors the lived-in
	// shape rather than the bug.
	_, err = tc.Exec("sh", "-c",
		"mkdir -p "+camp+"/.campaign/fest "+camp+"/festivals/.festival/.state"+
			" && touch "+camp+"/.campaign/fest/.gitkeep "+camp+"/festivals/.festival/.state/.gitkeep")
	require.NoError(t, err)

	// A baseline commit first: camp init leaves the branch unborn, and a
	// deferred job cannot execute against no HEAD (intent filed:
	// deferred-bookkeeping-commits-fail-on-an-unborn-branch). The steady
	// state every real campaign reaches immediately is what the drain
	// contract is defined over.
	out, err = tc.Exec("sh", "-c",
		"export HOME=/root && cd "+camp+" && camp commit -m 'campaign baseline'")
	require.NoError(t, err, "baseline camp commit should succeed: %s", out)

	// The intent enqueues its bookkeeping commit and returns; fest commit
	// follows in the same shell so the queued job is still in flight when
	// fest starts staging. The festival edit gives fest a commit of its own
	// to make. The drain guarantee, not timing, is what must order history.
	out, err = tc.Exec("sh", "-c",
		"export HOME=/root && cd "+camp+"/festivals/active/guard-festival"+
			" && echo 'progress' >> NOTES.md"+
			" && camp intent add 'queued bookkeeping intent'"+
			" && /fest commit --json --no-tag -m 'festival work' > /tmp/drain-stdout 2>/tmp/drain-stderr")
	require.NoError(t, err, "intent add followed by fest commit must succeed: %s", out)

	stdout, err := tc.ReadFile("/tmp/drain-stdout")
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc),
		"fest commit --json stdout must be the JSON document, got:\n%s", stdout)
	require.Equal(t, true, doc["success"], "fest commit must succeed: %v", doc)
	hash, _ := doc["hash"].(string)
	require.NotEmpty(t, hash, "criterion 15b: the structured output always carries a real hash")

	// 15b: never defers. The reported hash is HEAD at return; a deferred
	// commit could not be, because it would not exist yet.
	head, err := tc.Exec("git", "-C", camp, "rev-parse", "--short", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(head), hash,
		"fest's reported hash must be HEAD at return; fest never defers")

	// 15: the queued intent commit landed, and landed before fest's commit.
	subjects, err := tc.Exec("git", "-C", camp, "log", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, subjects, "queued bookkeeping intent",
		"the queued bookkeeping commit must be in history at fest commit's return")
	lines := strings.Split(strings.TrimSpace(subjects), "\n")
	require.NotEmpty(t, lines)
	assert.Contains(t, lines[0], "festival work",
		"HEAD must be the festival commit, with the drained bookkeeping beneath it")

	// The drain leaves nothing behind: no pending or running jobs at return.
	remaining, err := tc.Exec("sh", "-c",
		"find "+camp+"/.campaign/cache/jobs/pending "+camp+"/.campaign/cache/jobs/running -name '*.json' 2>/dev/null | wc -l")
	require.NoError(t, err)
	assert.Equal(t, "0", strings.TrimSpace(remaining),
		"fest commit must return with camp's queue drained")
}
