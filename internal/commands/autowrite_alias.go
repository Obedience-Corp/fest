package commands

// normalizeAutoWriteAlias rewrites the exact -aw token for fest commit before
// cobra parses flags. Cobra/pflag only supports single-character shorthand
// flags.
func normalizeAutoWriteAlias(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	if !isAutoWriteCommitInvocation(out) {
		return out
	}
	for i, arg := range out {
		if arg == "--" {
			break
		}
		if arg == "-aw" {
			out[i] = "--auto-write"
		}
	}
	return out
}

func isAutoWriteCommitInvocation(args []string) bool {
	command, _ := findFirstPositionalArg(args)
	return command == "commit"
}
