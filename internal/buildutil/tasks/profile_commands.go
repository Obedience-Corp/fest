package tasks

import (
	"fmt"
	"os/exec"
	"strings"
)

// ProfileCommands prints the visible command surface for the requested build
// profile and can also show the dev-only delta between stable and dev.
func ProfileCommands(profile string) error {
	switch profile {
	case "", "all":
		stable, err := commandSurface("")
		if err != nil {
			return err
		}

		dev, err := commandSurface("dev")
		if err != nil {
			return err
		}

		printCommandProfile("stable", stable)
		fmt.Println()
		printCommandProfile("dev", dev)
		fmt.Println()

		devOnly := diffCommandSurfaces(stable, dev)
		fmt.Printf("== dev-only commands (%d commands) ==\n", len(devOnly))
		for _, path := range devOnly {
			fmt.Println(path)
		}
		return nil

	case "stable":
		commands, err := commandSurface("")
		if err != nil {
			return err
		}
		printCommandProfile("stable", commands)
		return nil

	case "dev":
		commands, err := commandSurface("dev")
		if err != nil {
			return err
		}
		printCommandProfile("dev", commands)
		return nil

	default:
		return fmt.Errorf("unknown profile %q (use stable, dev, or all)", profile)
	}
}

func commandSurface(buildTag string) ([]string, error) {
	args := []string{"run"}
	if buildTag != "" {
		args = append(args, "-tags", buildTag)
	}
	args = append(args, "./cmd/fest", "__commands")

	cmd := exec.Command("go", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("collect %s command surface: %w\n%s", profileName(buildTag), err, strings.TrimSpace(string(output)))
	}

	return parseCommandSurfaceOutput(output), nil
}

func parseCommandSurfaceOutput(output []byte) []string {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	commands := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		commands = append(commands, line)
	}
	return commands
}

func diffCommandSurfaces(base, candidate []string) []string {
	baseSet := make(map[string]struct{}, len(base))
	for _, path := range base {
		baseSet[path] = struct{}{}
	}

	diff := make([]string, 0, len(candidate))
	for _, path := range candidate {
		if _, exists := baseSet[path]; exists {
			continue
		}
		diff = append(diff, path)
	}
	return diff
}

func printCommandProfile(name string, commands []string) {
	fmt.Printf("== %s profile (%d commands) ==\n", name, len(commands))
	for _, path := range commands {
		fmt.Println(path)
	}
}

func profileName(buildTag string) string {
	if buildTag == "" {
		return "stable"
	}
	return buildTag
}
