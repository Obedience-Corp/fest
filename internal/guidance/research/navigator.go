// Package research provides the ResearchNavigator for navigating
// festival research phases with topic-based organization.
package research

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

func init() {
	guidance.RegisterNavigator(guidance.ModeResearch, func(gctx *guidance.GuidanceContext) (guidance.Navigator, error) {
		return NewNavigator(gctx)
	})
}

// Navigator implements guidance.Navigator for research phases.
// Research phases are freeform with topic-based organization
// rather than strict task sequences.
//
// When a WORKFLOW.md file exists in the phase directory, the Navigator
// delegates to a WorkflowNavigator for step-based guidance. Otherwise,
// it uses the default topic-based discovery.
type Navigator struct {
	*guidance.BaseNavigator
	topics       []*Topic
	currentTopic int

	// workflowNav is used when WORKFLOW.md exists in the phase directory
	workflowNav   *workflow.Navigator
	useWorkflowMd bool
}

// Ensure Navigator implements guidance.Navigator.
var _ guidance.Navigator = (*Navigator)(nil)

// Topic represents a research topic or area of investigation.
type Topic struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`         // Directory for topic
	Status      string   `json:"status"`       // pending, in_progress, documented
	Questions   []string `json:"questions"`    // Research questions to investigate
	Findings    []string `json:"findings"`     // Documented findings
	SourceFiles []string `json:"source_files"` // Source files for research
}

// NewNavigator creates a new research navigator.
func NewNavigator(gctx *guidance.GuidanceContext) (*Navigator, error) {
	base, err := guidance.NewBaseNavigator(gctx, guidance.ModeResearch)
	if err != nil {
		return nil, errors.Wrap(err, "creating base navigator")
	}

	return &Navigator{
		BaseNavigator: base,
		topics:        make([]*Topic, 0),
		currentTopic:  0,
	}, nil
}

// Initialize loads state and discovers topics from filesystem.
func (n *Navigator) Initialize(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if err := n.BaseNavigator.Initialize(ctx); err != nil {
		return errors.Wrap(err, "initializing base navigator")
	}

	// Check if WORKFLOW.md exists in phase directory
	phaseDir := n.Ctx.PhasePath
	if phaseDir == "" {
		phaseDir = n.Ctx.FestivalPath
	}

	workflowPath := filepath.Join(phaseDir, "WORKFLOW.md")
	if _, err := os.Stat(workflowPath); err == nil {
		// WORKFLOW.md exists, initialize WorkflowNavigator
		wfNav, err := workflow.NewNavigator(n.Ctx, guidance.ModeResearch)
		if err != nil {
			return errors.Wrap(err, "creating workflow navigator")
		}
		if err := wfNav.Initialize(ctx); err != nil {
			return errors.Wrap(err, "initializing workflow navigator")
		}
		n.workflowNav = wfNav
		n.useWorkflowMd = true
		return nil
	}

	// No WORKFLOW.md, use default topic-based discovery
	// Discover topics from phase directory structure
	n.topics = n.discoverTopics()

	// Restore position from state
	state := n.StateManager.State()
	n.currentTopic = state.CurrentStep

	// Update total tasks in state
	state.TotalTasks = len(n.topics)

	return nil
}

// discoverTopics scans the festival for research topics.
// Topics are subdirectories with TOPIC.md or similar markers.
func (n *Navigator) discoverTopics() []*Topic {
	var topics []*Topic

	// Look for topics in phase directory
	phasePath := n.Ctx.FestivalPath
	if n.Ctx.PhasePath != "" {
		phasePath = n.Ctx.PhasePath
	}

	// Scan for directories that might be topics
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return topics
	}

	order := 1
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip hidden directories and common non-topic directories
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "input" || name == "output" {
			continue
		}

		topicPath := filepath.Join(phasePath, name)

		// Check for topic markers (TOPIC.md, RESEARCH.md, or README.md)
		if hasTopicMarker(topicPath) {
			topic := &Topic{
				ID:     fmt.Sprintf("topic_%02d_%s", order, name),
				Name:   formatTopicName(name),
				Path:   topicPath,
				Status: "pending",
			}

			// Try to load questions from marker file
			topic.Questions = loadTopicQuestions(topicPath)

			topics = append(topics, topic)
			order++
		}
	}

	return topics
}

// hasTopicMarker checks if a directory contains a topic marker file.
func hasTopicMarker(dir string) bool {
	markers := []string{"TOPIC.md", "RESEARCH.md", "README.md", "topic.md"}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// formatTopicName converts a directory name to a readable topic name.
func formatTopicName(name string) string {
	// Replace underscores and hyphens with spaces
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// Capitalize first letter of each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}

// loadTopicQuestions loads research questions from topic marker files.
func loadTopicQuestions(_ string) []string {
	// TODO: Parse questions from marker files
	// For now, return default questions
	return []string{
		"What is the current state of this topic?",
		"What are the key findings?",
		"What questions remain unanswered?",
	}
}

// GetNext returns the next research topic.
func (n *Navigator) GetNext(ctx context.Context) (*guidance.NextStep, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.GetNext(ctx)
	}

	// Find first topic with pending work
	for i, topic := range n.topics {
		if topic.Status != "documented" {
			n.currentTopic = i
			return &guidance.NextStep{
				Mode:      guidance.ModeResearch,
				StepType:  guidance.StepTypeResearchStep,
				ID:        topic.ID,
				Title:     "Research: " + topic.Name,
				Path:      topic.Path,
				Objective: fmt.Sprintf("Investigate and document findings for %s", topic.Name),
				Instructions: []string{
					"Review the research questions listed below",
					"Investigate each question thoroughly",
					"Document findings in the topic directory",
					"Update the findings list when complete",
					"Mark as documented when all questions are addressed",
				},
				AutonomyLevel:     guidance.AutonomyMedium,
				ContextFiles:      n.getTopicContextFiles(topic),
				CompletionCommand: fmt.Sprintf("fest research next --complete %s", topic.ID),
				Metadata: map[string]any{
					"questions":    topic.Questions,
					"findings":     topic.Findings,
					"source_files": topic.SourceFiles,
					"topic_number": i + 1,
					"total_topics": len(n.topics),
				},
			}, nil
		}
	}

	return nil, nil // All topics documented
}

// getTopicContextFiles returns context files for a specific topic.
func (n *Navigator) getTopicContextFiles(topic *Topic) []string {
	files := []string{topic.Path}

	// Add phase goal if available
	if n.Ctx.PhasePath != "" {
		files = append(files, filepath.Join(n.Ctx.PhasePath, "PHASE_GOAL.md"))
	}

	// Add topic-specific source files
	files = append(files, topic.SourceFiles...)

	return files
}

// MarkComplete marks a topic as documented.
func (n *Navigator) MarkComplete(ctx context.Context, topicID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.MarkComplete(ctx, topicID)
	}

	for i, topic := range n.topics {
		if topic.ID == topicID {
			topic.Status = "documented"
			n.StateManager.SetTaskStatus(topicID, guidance.StatusCompleted)

			// Move to next topic
			n.currentTopic = i + 1
			n.StateManager.SetPosition(0, 0, n.currentTopic)

			return n.StateManager.Save(ctx)
		}
	}

	return errors.NotFound("research topic not found").WithField("topicID", topicID)
}

// MarkSkipped marks a topic as skipped.
func (n *Navigator) MarkSkipped(ctx context.Context, topicID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.MarkSkipped(ctx, topicID)
	}

	for i, topic := range n.topics {
		if topic.ID == topicID {
			topic.Status = "skipped"
			n.StateManager.SetTaskStatus(topicID, guidance.StatusSkipped)
			n.currentTopic = i + 1
			n.StateManager.SetPosition(0, 0, n.currentTopic)
			return n.StateManager.Save(ctx)
		}
	}

	return errors.NotFound("research topic not found").WithField("topicID", topicID)
}

// MarkFailed marks a topic as failed.
func (n *Navigator) MarkFailed(ctx context.Context, topicID string) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.MarkFailed(ctx, topicID)
	}

	for _, topic := range n.topics {
		if topic.ID == topicID {
			topic.Status = "failed"
			n.StateManager.SetTaskStatus(topicID, guidance.StatusFailed)
			return n.StateManager.Save(ctx)
		}
	}

	return errors.NotFound("research topic not found").WithField("topicID", topicID)
}

// Advance moves to the next topic.
func (n *Navigator) Advance(ctx context.Context) error {
	if err := n.EnsureInitialized(); err != nil {
		return err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.Advance(ctx)
	}

	step, err := n.GetNext(ctx)
	if err != nil {
		return err
	}

	if step == nil {
		return guidance.ErrAlreadyComplete
	}

	return n.MarkComplete(ctx, step.ID)
}

// GetProgress returns research progress.
func (n *Navigator) GetProgress(ctx context.Context) (*guidance.Progress, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.GetProgress(ctx)
	}

	progress := guidance.NewProgress(guidance.ModeResearch)
	progress.Total = len(n.topics)

	for _, topic := range n.topics {
		switch topic.Status {
		case "documented":
			progress.Completed++
		case "skipped":
			progress.Skipped++
		case "failed":
			progress.Failed++
		default:
			progress.Pending++
		}
	}

	progress.Calculate()

	if n.currentTopic < len(n.topics) {
		progress.CurrentTask = n.topics[n.currentTopic].Name
	}

	return progress, nil
}

// FormatInstructions generates agent-friendly research instructions.
func (n *Navigator) FormatInstructions(ctx context.Context) (string, error) {
	if err := n.EnsureInitialized(); err != nil {
		return "", err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.FormatInstructions(ctx)
	}

	if len(n.topics) == 0 {
		return n.formatNoTopics(), nil
	}

	step, err := n.GetNext(ctx)
	if err != nil {
		return "", err
	}

	if step == nil {
		return n.formatComplete(), nil
	}

	progress, _ := n.GetProgress(ctx)

	return n.formatInstructions(step, progress), nil
}

// formatNoTopics renders message when no topics found.
func (n *Navigator) formatNoTopics() string {
	var sb strings.Builder

	sb.WriteString("# Research Phase\n")
	sb.WriteString("────────────────────\n\n")
	sb.WriteString("No research topics discovered.\n\n")
	sb.WriteString("To add topics, create subdirectories with TOPIC.md files.\n")

	return sb.String()
}

// formatComplete renders the completion message.
func (n *Navigator) formatComplete() string {
	var sb strings.Builder

	sb.WriteString("# Research Complete\n")
	sb.WriteString("────────────────────\n\n")
	sb.WriteString("All research topics have been documented.\n\n")
	sb.WriteString("Run `fest status` to review findings.\n")

	return sb.String()
}

// formatInstructions renders the research instructions.
func (n *Navigator) formatInstructions(step *guidance.NextStep, progress *guidance.Progress) string {
	var sb strings.Builder

	sb.WriteString(guidance.InstructionHeader)
	// Header
	sb.WriteString("# Research Phase Guidance\n")
	sb.WriteString("────────────────────────────────\n\n")

	// Progress line
	fmt.Fprintf(&sb, "Progress       %s\n\n", progress.Summary())

	// Current topic section
	sb.WriteString("## Current Topic\n")
	sb.WriteString("────────────────\n")
	if md, ok := step.Metadata["topic_number"].(int); ok {
		fmt.Fprintf(&sb, "Topic          %d of %d\n", md, step.Metadata["total_topics"])
	}
	fmt.Fprintf(&sb, "Title          %s\n", step.Title)
	fmt.Fprintf(&sb, "Path           %s\n\n", step.Path)

	// Questions section
	if questions, ok := step.Metadata["questions"].([]string); ok && len(questions) > 0 {
		sb.WriteString("## Research Questions\n")
		sb.WriteString("────────────────────\n")
		for _, q := range questions {
			fmt.Fprintf(&sb, "  ? %s\n", q)
		}
		sb.WriteString("\n")
	}

	// Instructions section
	sb.WriteString("## Instructions\n")
	sb.WriteString("────────────────\n")
	for _, instruction := range step.Instructions {
		fmt.Fprintf(&sb, "  • %s\n", instruction)
	}
	sb.WriteString("\n")

	// Context files section
	sb.WriteString("## Context Files\n")
	sb.WriteString("─────────────\n")
	for _, file := range step.ContextFiles {
		fmt.Fprintf(&sb, "  - %s\n", file)
	}
	sb.WriteString("\n")

	// Completion command
	sb.WriteString("## When Complete\n")
	sb.WriteString("────────────────\n")
	fmt.Fprintf(&sb, "Run: %s\n", step.CompletionCommand)

	return sb.String()
}

// GetContextFiles returns context files.
func (n *Navigator) GetContextFiles(ctx context.Context) ([]string, error) {
	if err := n.EnsureInitialized(); err != nil {
		return nil, err
	}

	// Check context cancellation
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(err, "context cancelled")
		}
	}

	// Delegate to WorkflowNavigator if WORKFLOW.md is being used
	if n.useWorkflowMd && n.workflowNav != nil {
		return n.workflowNav.GetContextFiles(ctx)
	}

	files := []string{
		filepath.Join(n.Ctx.FestivalPath, "FESTIVAL_GOAL.md"),
	}

	if n.Ctx.PhasePath != "" {
		files = append(files, filepath.Join(n.Ctx.PhasePath, "PHASE_GOAL.md"))
	}

	return files, nil
}

// AddTopic adds a new research topic.
func (n *Navigator) AddTopic(topic *Topic) {
	n.topics = append(n.topics, topic)
}

// GetTopics returns all research topics.
func (n *Navigator) GetTopics() []*Topic {
	return n.topics
}
