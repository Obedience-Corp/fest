package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/version"
	"github.com/Obedience-Corp/obey-shared/updatecheck"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// suiteNoticeExcludedCommands never show the ambient suite notice, nor does
// anything beneath them. next is the agent loop surface (design D6); version
// prints its own result with --check.
var suiteNoticeExcludedCommands = map[string]bool{
	"completion": true,
	"gendocs":    true,
	"help":       true,
	"next":       true,
	"shell-init": true,
	"version":    true,
}

// suiteMachineOutputFlags are flags whose presence means stdout is consumed
// by a program, so the invocation is not a human surface.
var suiteMachineOutputFlags = []string{"json", "agent", "mermaid"}

var suiteMachineFormats = map[string]bool{
	"json":   true,
	"jsonl":  true,
	"ndjson": true,
	"yaml":   true,
	"yml":    true,
}

var (
	suiteBundle      = func() string { return version.Bundle }
	suiteStdoutIsTTY = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }
	suiteStderrIsTTY = func() bool { return term.IsTerminal(int(os.Stderr.Fd())) }
	suiteGetenv      = os.Getenv
	suiteObeyDir     = config.ObeyDir
	suiteExecutable  = runningExecutable
	suiteFetcher     *updatecheck.Fetcher
	suiteNotify      = updatecheck.MaybeNotify
)

// suiteHelpShown records that the run printed help, as the help command
// does. Help is not the command the user meant to run, so it gets no notice.
var suiteHelpShown bool

func init() {
	rootCmd.PersistentPostRun = ambientSuiteNotice
	showHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		suiteHelpShown = true
		showHelp(cmd, args)
	})
}

// ambientSuiteNotice tells a human at a terminal that a newer Festival suite
// release exists. Cobra only reaches it after a command succeeded. It never
// writes to stdout and never fails the command. Captured stdout, as in
// $(fest go --print), means a program is reading fest, so it stays silent.
func ambientSuiteNotice(cmd *cobra.Command, _ []string) {
	bundle := suiteBundle()
	if bundle == "" || suiteHelpShown || !suiteNoticeEligible(cmd) || !suiteStdoutIsTTY() {
		return
	}
	in := updatecheck.SuppressInput{LookupEnv: suiteGetenv, StderrIsTTY: suiteStderrIsTTY()}
	if updatecheck.AmbientSuppressed(in) {
		return
	}
	cachePath, ok := suiteCachePath()
	if !ok {
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	notice := suiteNotify(ctx, bundle, updatecheck.Options{
		CachePath:      cachePath,
		ExecutablePath: suiteExecutable(),
		Fetcher:        suiteFetcher,
		Suppress:       in,
	})
	if notice != nil {
		renderSuiteNotice(cmd.ErrOrStderr(), notice)
	}
}

// suiteNoticeEligible reports whether cmd is a human command surface.
func suiteNoticeEligible(cmd *cobra.Command) bool {
	if cmd == nil || !cmd.HasParent() {
		return false
	}
	for c := cmd; c.HasParent(); c = c.Parent() {
		if c.Hidden || suiteNoticeExcludedCommands[c.Name()] {
			return false
		}
	}
	return !machineOutputRequested(cmd)
}

func machineOutputRequested(cmd *cobra.Command) bool {
	flags := cmd.Flags()
	for _, name := range suiteMachineOutputFlags {
		if f := flags.Lookup(name); f != nil && (f.Changed || f.Value.String() == "true") {
			return true
		}
	}
	if f := flags.Lookup("format"); f != nil && suiteMachineFormats[strings.ToLower(f.Value.String())] {
		return true
	}
	return false
}

// suiteCachePath is ~/.obey/update-check/suite.json, the cache camp shares.
func suiteCachePath() (string, bool) {
	dir, err := suiteObeyDir()
	if err != nil || !filepath.IsAbs(dir) {
		return "", false
	}
	return filepath.Join(dir, "update-check", "suite.json"), true
}

func runningExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

func renderSuiteNotice(w io.Writer, n *updatecheck.Notice) {
	_, _ = fmt.Fprintf(w, "%s %s %s\n", ui.Warning(ui.WarningIcon), ui.Warning("notice:"), n.Message)
	_, _ = fmt.Fprintf(w, "  %s %s\n", ui.Dim(n.Kind+":"), ui.Accent(n.Command))
}
