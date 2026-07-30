package show

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/fest/internal/resident"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// residentStages are the lifecycle folders a resident can occupy, mirroring
// camp's v1 rail. Other statuses never yield residents.
var residentStages = map[string]bool{"ready": true, "active": true}

// ResidentCard is the lightweight view of a camp lifecycle resident. Residents
// deliberately carry no phase, sequence, or task semantics: they are workitems
// parked on a stage, not festivals.
type ResidentCard struct {
	Name  string
	Title string
	Type  string
	Path  string
	Run   *StandaloneWorkflowInfo
}

// residentCardJSON is the published shape for a resident card. Defined explicitly
// rather than tagging ResidentCard so internal fields (the absolute Path, the whole
// standalone runtime) cannot leak into the contract, and so every surface emits the
// same keys.
type residentCardJSON struct {
	Name           string `json:"name"`
	Title          string `json:"title"`
	Type           string `json:"type"`
	RunStatus      string `json:"run_status,omitempty"`
	CompletedSteps int    `json:"completed_steps,omitempty"`
	TotalSteps     int    `json:"total_steps,omitempty"`
}

// MarshalJSON keeps fest list and fest status list in agreement: both marshal the
// same card type, so neither can drift into exposing internals.
func (c *ResidentCard) MarshalJSON() ([]byte, error) {
	out := residentCardJSON{Name: c.Name, Title: c.Title, Type: c.Type}
	if c.Run != nil {
		out.RunStatus = c.Run.RunStatus
		out.CompletedSteps = c.Run.CompletedSteps
		out.TotalSteps = c.Run.TotalSteps
	}
	return json.Marshal(out)
}

// Progress renders the resident's standalone run state, or "" when it has no
// .workflow/ runtime.
func (c *ResidentCard) Progress() string {
	if c.Run == nil {
		return ""
	}
	if c.Run.TotalSteps > 0 {
		return strconv.Itoa(c.Run.CompletedSteps) + "/" + strconv.Itoa(c.Run.TotalSteps) + " steps"
	}
	return c.Run.RunStatus
}

// ListResidentsByStatus returns the residents parked on one lifecycle stage,
// sorted by name. Only ready and active can hold residents; every other status
// returns nil so callers need no special-casing.
func ListResidentsByStatus(ctx context.Context, festivalsDir, status string) []*ResidentCard {
	if !residentStages[status] {
		return nil
	}
	stageDir := workspace.JoinStatus(festivalsDir, status)
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		return nil
	}

	var cards []*ResidentCard
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(stageDir, e.Name())
		marker, rerr := resident.Read(dir)
		if rerr != nil || marker == nil {
			continue
		}
		cards = append(cards, residentCard(ctx, dir, e.Name(), marker))
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Name < cards[j].Name })
	return cards
}

func residentCard(ctx context.Context, dir, name string, marker *resident.Marker) *ResidentCard {
	card := &ResidentCard{
		Name:  name,
		Title: marker.Title,
		Type:  marker.Type,
		Path:  dir,
	}
	if card.Title == "" {
		card.Title = name
	}
	// Run progress comes from fest's own standalone runtime, never from camp.
	// A resident without a .workflow/ runtime simply has no progress to show.
	if run, err := ResolveStandaloneWorkflow(ctx, dir); err == nil && run != nil {
		card.Run = run
	}
	return card
}
