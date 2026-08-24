package activity

import (
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/version"
)

// sensitiveFlags are command-line flags whose values must be redacted from
// source_cmd before being written to the activity log.
var sensitiveFlags = []string{
	"--token", "--password", "--secret", "--signing-key",
}

// resolveActor builds the Actor record from environment and version info.
func resolveActor() Actor {
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	host, _ := os.Hostname()
	return Actor{
		User:        user,
		Host:        host,
		FestVersion: version.Version,
	}
}

// redact replaces the values of known sensitive flags in a command string with
// <REDACTED>. Arbitrary user messages (e.g. --reason "...") are logged verbatim
// per the issue spec — only recognized credential-bearing flags are redacted.
func redact(cmd string) string {
	if cmd == "" {
		return cmd
	}
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return cmd
	}
	for i := 0; i < len(parts); i++ {
		for _, flag := range sensitiveFlags {
			if parts[i] == flag {
				// Redact the value that follows the flag.
				if i+1 < len(parts) {
					parts[i+1] = "<REDACTED>"
					i++
				}
				break
			}
			// Handle --flag=value form.
			if strings.HasPrefix(parts[i], flag+"=") {
				parts[i] = flag + "=<REDACTED>"
				break
			}
		}
	}
	return strings.Join(parts, " ")
}
