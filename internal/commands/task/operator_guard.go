package task

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/fest/internal/continuation"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
	"golang.org/x/term"
)

const ancestryDepthLimit = 32

// OperatorAudit is the record a guarded verb writes into its event so a
// deferral carries what the guard saw, not only that it passed.
type OperatorAudit struct {
	Actor        string
	TTY          bool
	AgentMarkers []string
	Ancestry     []string
	DeferredBy   string
}

// Progress converts the guard record into the value the progress store writes
// into a deferral or forced-completion event.
func (a *OperatorAudit) Progress() progress.DeferralAudit {
	if a == nil {
		return progress.DeferralAudit{}
	}
	return progress.DeferralAudit{
		Actor:        a.Actor,
		TTY:          a.TTY,
		AgentMarkers: a.AgentMarkers,
		Ancestry:     a.Ancestry,
		DeferredBy:   a.DeferredBy,
	}
}

var operatorGuardStdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

var operatorAgentMarkers = []string{
	"OBEY_AGENT",
	"CLAUDE_CODE",
	"CLAUDECODE",
	"CLAUDE_CODE_SESSION_ID",
	"CODEX_TASK",
	continuation.EnvSessionID,
}

var agentBinaries = map[string]bool{
	"claude":       true,
	"obey":         true,
	"codex":        true,
	"cursor-agent": true,
	"agent":        true,
	"grok":         true,
	"script":       true,
}

var operatorGuardAncestry = readProcessAncestry

var operatorGuardPSEntry = psEntry

// OperatorGuard is the single implementation of the operator-only rule shared
// by every verb that lets a festival move past a blocker. The TTY check runs
// before the marker check so the refusal for an agent with no terminal is
// deterministic. On success audit.AgentMarkers lists the markers that were
// checked and found absent.
func OperatorGuard(ctx context.Context, verb string) (*OperatorAudit, error) {
	audit := &OperatorAudit{Actor: "operator"}

	if !operatorGuardStdinIsTerminal() {
		return nil, errors.Validation(verb + " is an operator decision; run this from your terminal").
			WithHint("over ssh use 'ssh -t'; this verb has no --yes and no --json")
	}
	audit.TTY = true

	for _, marker := range operatorAgentMarkers {
		if os.Getenv(marker) != "" {
			return nil, errors.Validation(verb+" is an operator decision and "+marker+" is set").
				WithField("marker", marker).
				WithHint("run this from a plain terminal, not inside an agent session")
		}
		audit.AgentMarkers = append(audit.AgentMarkers, marker)
	}

	// An unreadable chain records unknown and proceeds. A partial chain is
	// discarded because the absence of an agent in it is not evidence.
	chain, err := operatorGuardAncestry(ctx)
	if err != nil {
		audit.Ancestry = []string{"unknown"}
		return audit, nil
	}
	audit.Ancestry = chain

	for _, comm := range chain {
		if agentBinaries[filepath.Base(comm)] {
			return nil, errors.Validation(verb+" is an operator decision and an agent process is in the parent chain").
				WithField("ancestor", comm).
				WithHint("run this from a plain terminal, not from inside an agent or a script session")
		}
	}

	audit.DeferredBy = gitUserName(ctx)

	return audit, nil
}

func readProcessAncestry(ctx context.Context) ([]string, error) {
	chain := make([]string, 0, 8)
	seen := make(map[int]bool)
	pid := os.Getppid()

	for range ancestryDepthLimit {
		if pid <= 1 || seen[pid] {
			break
		}
		seen[pid] = true

		ppid, comm, err := operatorGuardPSEntry(ctx, pid)
		if err != nil {
			return chain, err
		}
		chain = append(chain, comm)
		pid = ppid
	}

	return chain, nil
}

func psEntry(ctx context.Context, pid int) (int, string, error) {
	out, err := exec.CommandContext(ctx, "ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, "", errors.Wrap(err, "reading process ancestry")
	}

	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return 0, "", errors.Validation("unreadable ps output for pid " + strconv.Itoa(pid))
	}

	ppid, convErr := strconv.Atoi(fields[0])
	if convErr != nil {
		return 0, "", errors.Wrap(convErr, "parsing parent pid")
	}

	return ppid, strings.Join(fields[1:], " "), nil
}
