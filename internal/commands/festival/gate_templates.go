package festival

// DefaultGateTemplates contains embedded default gate template content.
// These are used as fallback when templates don't exist in the template root.
var DefaultGateTemplates = map[string]string{
	"QUALITY_GATE_TESTING.md": `# Gate: Testing and Verification

Run all tests for this sequence. Verify no regressions.

- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] No regressions introduced
- [ ] Build completes without warnings
`,
	"QUALITY_GATE_REVIEW.md": `# Gate: Code Review

Review all changes in this sequence for correctness and standards compliance.

- [ ] Linting passes without warnings
- [ ] Code is readable and follows project conventions
- [ ] No obvious bugs or security issues
- [ ] Changes align with sequence goal

**Findings:** Document any issues that must be addressed before commit.
`,
	"QUALITY_GATE_ITERATE.md": `# Gate: Review Results and Iterate

Address all findings from testing and code review. Iterate until clean.

- [ ] All critical findings fixed
- [ ] Tests re-run and pass
- [ ] Linting passes
- [ ] Ready to commit
`,
	"QUALITY_GATE_FEST_COMMIT.md": `# Gate: Commit Sequence Changes

Commit all changes from this sequence using fest commit.

## Pre-Commit

- [ ] All tests pass
- [ ] Linting is clean
- [ ] No debug code or temporary files

## Commit

**Use fest commit** so task references are preserved:

    fest commit -m "<type>: <summary>"

Do NOT use raw git commit. The fest commit command tags commits for tracking.
`,
}
