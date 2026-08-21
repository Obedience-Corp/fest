package commit

import (
	"context"
	"fmt"
	"strings"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
	"github.com/Obedience-Corp/fest/internal/scope"
)

// Commit reference format prefix: FE (Festival component)
const commitRefPrefix = "FE"

// formatCommitRef formats an ID with the standard commit reference prefix.
// Format: [FE-{id}] where FE = Festival component
func formatCommitRef(id string) string {
	return fmt.Sprintf("%s-%s", commitRefPrefix, id)
}

// festRefWithPosition appends the phase and sequence segments to a bare
// "FE-{id}" reference for the tags campaign formatting does not build. The
// segments are dependent: a sequence is only meaningful under a phase, so it
// is dropped when the phase is unknown.
func festRefWithPosition(festRef string, pos position) string {
	if festRef == "" || pos.Phase == "" {
		return festRef
	}
	ref := festRef + "-PH-" + pos.Phase
	if pos.Sequence != "" {
		ref += "-SQ-" + pos.Sequence
	}
	return ref
}

// festCommitMessage prefixes a message with the festival reference, carrying
// the position when one was resolved.
func festCommitMessage(festRef string, pos position, rawMsg string) string {
	if festRef == "" {
		return rawMsg
	}
	return fmt.Sprintf("[%s] %s", festRefWithPosition(festRef, pos), rawMsg)
}

// campaignCommitMessage builds the shared campaign/festival tag without
// requiring a project commit. Festival-only commits use the same tag for the
// campaign-root commit even when the linked project has nothing staged.
func campaignCommitMessage(ctx context.Context, ws *scope.WorkspaceInfo, festRef string, pos position, rawMsg, festMessage string) (string, string) {
	commitMessage := festMessage
	var campaignTag string

	if ws != nil && ws.Type == scope.WorkspaceTypeCampaign {
		cid, err := commitkit.DetectCampaign(ctx)
		if err == nil && cid != "" {
			name, _ := commitkit.DetectCampaignName(ctx)
			body := festMessage
			if festRef != "" {
				body = rawMsg
			}
			campaignTag = commitkit.FormatTag(campaignTagComponents(name, cid, festRef, pos))
			commitMessage = campaignTag + " " + body
		}
	}

	return commitMessage, campaignTag
}

// festivalRootCommitMessage builds the message for the campaign root commit
// that carries festival-scoped files.
func festivalRootCommitMessage(campaignTag, festRef string, pos position, rawMsg string) string {
	rootMsg := "fest: " + rawMsg
	if campaignTag != "" {
		return campaignTag + " " + rootMsg
	}
	return festCommitMessage(festRef, pos, rootMsg)
}

// campaignTagComponents maps the resolved commit context onto the canonical
// tag components. The position rides along only with a festival reference,
// which is the identifier both segments index into.
func campaignTagComponents(campaignName, campaignID, festRef string, pos position) commitkit.TagComponents {
	components := commitkit.TagComponents{CampaignName: campaignName, CampaignID: campaignID}
	if festRef == "" {
		return components
	}
	// festRef is "FE-<id>"; pass the bare id (the formatter re-adds FE-).
	components.FestRef = strings.TrimPrefix(festRef, commitRefPrefix+"-")
	components.Phase = pos.Phase
	components.Sequence = pos.Sequence
	return components
}
