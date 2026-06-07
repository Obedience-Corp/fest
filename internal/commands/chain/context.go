package chain

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/tui/picker"
	"golang.org/x/term"
)

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// resolveCurrentFestivalID infers the current festival id from the cwd festival
// dir or a linked project working dir, or ("", false).
func resolveCurrentFestivalID(ctx context.Context) (string, bool) {
	if err := ctx.Err(); err != nil {
		return "", false
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	if festID := festivalIDFromMarkers(cwd); festID != "" {
		return festID, true
	}
	if festID := festivalIDFromLink(cwd); festID != "" {
		return festID, true
	}
	return "", false
}

func festivalIDFromMarkers(cwd string) string {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, config.FestivalConfigFileName)); err == nil {
			if cfg, err := config.LoadFestivalConfig(dir, ""); err == nil && cfg.Metadata.HasMetadata() {
				return cfg.Metadata.ID
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func festivalIDFromLink(cwd string) string {
	nav, err := navigation.LoadNavigation()
	if err != nil {
		return ""
	}
	name := nav.FindFestivalForPath(cwd)
	if name == "" {
		return ""
	}
	link, ok := nav.GetLink(name)
	if !ok || link.FestivalPath == "" {
		return ""
	}
	cfg, err := config.LoadFestivalConfig(link.FestivalPath, "")
	if err != nil || !cfg.Metadata.HasMetadata() {
		return ""
	}
	return cfg.Metadata.ID
}

// resolveChainID returns the chain id: explicit if given, else inferred from the
// current festival's chain, else picked interactively in a TTY. inferred is true
// when not passed explicitly, so mutating commands can confirm first.
func resolveChainID(ctx context.Context, explicit string) (chainID string, inferred bool, err error) {
	if explicit != "" {
		return explicit, false, nil
	}

	root, err := festivalsRoot()
	if err != nil {
		return "", false, err
	}

	if festID, ok := resolveCurrentFestivalID(ctx); ok {
		if c, _, findErr := chainpkg.FindForFestival(ctx, festID, root); findErr == nil && c != nil {
			return c.Metadata.ID, true, nil
		}
	}

	picked, pickErr := pickChainID(ctx, root)
	if pickErr != nil {
		return "", false, pickErr
	}
	if picked == "" {
		return "", false, errors.Validation("chain id required").
			WithHint("pass a chain id, or run from inside a festival or linked project in an interactive terminal")
	}
	return picked, true, nil
}

// pickChainID picks a non-terminal chain interactively, or returns "" when not a
// TTY, no chains exist, or cancelled.
func pickChainID(ctx context.Context, festivalsRoot string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return "", nil
	}

	chains, err := chainpkg.DiscoverAll(ctx, festivalsRoot)
	if err != nil {
		return "", errors.Wrap(err, "discovering chains")
	}

	items := make([]picker.Item, 0, len(chains))
	for _, c := range chains {
		if c.Metadata.Status == chainpkg.StatusCompleted {
			continue
		}
		items = append(items, picker.Item{
			Name:  fmt.Sprintf("[%s] %s", c.Metadata.ID, c.Metadata.Name),
			Value: c.Metadata.ID,
		})
	}
	if len(items) == 0 {
		return "", nil
	}

	selected, err := picker.Run(items, navigation.Score)
	if err != nil {
		return "", errors.Wrap(err, "running chain picker")
	}
	if selected == nil {
		return "", nil
	}
	return selected.Value, nil
}

// confirmChainCompletion confirms archiving an inferred/picked chain; refuses in
// a non-interactive shell so an inferred target is never archived without consent.
func confirmChainCompletion(chainID string) bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		fmt.Fprintf(os.Stderr,
			"refusing to complete inferred chain %s non-interactively; pass the chain id explicitly\n", chainID)
		return false
	}
	fmt.Fprintf(os.Stderr, "Complete and archive chain %s? [y/N]: ", chainID)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}
