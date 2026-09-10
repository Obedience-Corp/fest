package hooks

// JudgeInstallCommand installs the public reference judge that speaks
// fest.approval.judge/v1. It is the command every "how do I get a judge"
// surface points at, so a trial user sees one install line everywhere.
const JudgeInstallCommand = "go install github.com/Obedience-Corp/judge-agent/cmd/judge-agent@latest"

// JudgeExampleCommand is the hook command shown in examples. judge-agent
// wraps a CLI the user already runs; swap the agent for grok, codex, or fx.
const JudgeExampleCommand = "judge-agent --agent claude"

// JudgeConfigExample is the smallest festivals/.festival/config.yaml block that
// turns on the approval judge for every festival in a camp.
const JudgeConfigExample = `hooks:
  definitions:
    approval_judge:
      command: ` + JudgeExampleCommand

// JudgeSetupLines is the two-step recipe printed wherever fest teaches that a
// gate can be delegated: install the judge, then declare the hook.
func JudgeSetupLines() []string {
	return []string{
		"Install a judge:  " + JudgeInstallCommand,
		"Then add to festivals/.festival/config.yaml (swap claude for grok, codex, or fx):",
		"  hooks:",
		"    definitions:",
		"      approval_judge:",
		"        command: " + JudgeExampleCommand,
	}
}
