// Package pack implements fest pack / fest unbundle for Festival Bundles.
package pack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/validator"
	"github.com/Obedience-Corp/obey-shared/festivalbundle"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewPackCommand returns fest pack (global scope: works outside a workspace).
func NewPackCommand() *cobra.Command {
	var (
		output  string
		kind    string
		name    string
		strict  bool
		noSent  bool
		jsonOut bool
		creator string
	)
	cmd := &cobra.Command{
		Use:   "pack [source-dir]",
		Short: "Pack a festival or ritual directory into a .festival bundle",
		Long: `Pack a festival/ritual tree into a portable .festival ZIP.

Does not execute or promote the festival. Out-of-root file links are vendored
into .artifacts/; in-root links are left unchanged.

Works from any directory (global scope); source path is explicit.`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			source, err := filepath.Abs(args[0])
			if err != nil {
				return festerrors.Wrap(err, "resolve source")
			}
			if st, err := os.Stat(source); err != nil || !st.IsDir() {
				return festerrors.New(fmt.Sprintf("not a directory: %s", source))
			}

			k := kind
			if k == "" {
				k = inferKind(source)
			}
			n := name
			if n == "" {
				n = filepath.Base(source)
			}
			opts := festivalbundle.PackOptions{
				Kind:            k,
				Name:            n,
				Creator:         creator,
				Strict:          strict,
				WriteSentRecord: !noSent,
			}
			if sub := loadFestSubject(source); sub != nil {
				opts.Subject = sub
				if n == filepath.Base(source) && sub.Title != "" {
					opts.Name = sub.Title
				}
			}

			info, err := festivalbundle.Pack(ctx, source, output, opts)
			if err != nil {
				return festerrors.Wrap(err, "pack")
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", output)
			fmt.Fprintf(cmd.OutOrStdout(), "kind=%s id=%s name=%q\n", info.Kind, info.Bundle.ID, info.Bundle.Name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output .festival path (required)")
	cmd.Flags().StringVar(&kind, "kind", "", "bundle kind (festival|ritual); inferred when empty")
	cmd.Flags().StringVar(&name, "name", "", "bundle name")
	cmd.Flags().BoolVar(&strict, "strict", false, "fail if out-of-root links are missing")
	cmd.Flags().BoolVar(&noSent, "no-sent-record", false, "skip .bundles/sent on source")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print info.json on stdout")
	cmd.Flags().StringVar(&creator, "creator", "fest", "bundle.creator")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}

// NewUnbundleCommand returns fest unbundle (global scope).
func NewUnbundleCommand() *cobra.Command {
	var (
		dest     string
		force    bool
		noVerify bool
		noRecv   bool
		jsonOut  bool
		validate bool
	)
	cmd := &cobra.Command{
		Use:   "unbundle [path.festival]",
		Short: "Unbundle a .festival archive into a directory",
		Long: `Extract a Festival Bundle into a live directory.

Does NOT run, promote, or activate the festival. Use fest ritual run or normal
fest workflow separately after unbundle if execution is desired.

Optional --validate runs in-process festival validation on the destination
(this binary's validator, not a PATH-installed fest). Validation diagnostics
go to stderr so --json still emits a single JSON document on stdout.`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			festivalPath, err := filepath.Abs(args[0])
			if err != nil {
				return festerrors.Wrap(err, "resolve festival")
			}
			d, err := filepath.Abs(dest)
			if err != nil {
				return festerrors.Wrap(err, "resolve dest")
			}
			info, err := festivalbundle.Unbundle(ctx, festivalPath, d, festivalbundle.UnbundleOptions{
				Force:               force,
				SkipVerify:          noVerify,
				WriteReceivedRecord: !noRecv,
			})
			if err != nil {
				return festerrors.Wrap(err, "unbundle")
			}

			if validate {
				result, err := validator.FullValidate(ctx, d)
				if err != nil {
					return festerrors.Wrap(err, "validate after unbundle")
				}
				// Always send human validation summary to stderr so --json
				// stdout remains a single JSON document.
				fmt.Fprintf(cmd.ErrOrStderr(), "validate: score=%d valid=%v issues=%d\n",
					result.Score, result.Valid, len(result.Issues))
				for _, issue := range result.Issues {
					fmt.Fprintf(cmd.ErrOrStderr(), "  - %s: %s\n", issue.Code, issue.Message)
				}
				if !result.Valid {
					return festerrors.New(fmt.Sprintf("validation failed (score %d)", result.Score))
				}
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "unbundled to %s\n", d)
			fmt.Fprintf(cmd.OutOrStdout(), "kind=%s id=%s name=%q\n", info.Kind, info.Bundle.ID, info.Bundle.Name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&dest, "dest", "d", "", "destination directory (required)")
	cmd.Flags().BoolVar(&force, "force", false, "allow non-empty destination")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "skip content-hash verification")
	cmd.Flags().BoolVar(&noRecv, "no-received-record", false, "skip .bundles/received")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print info.json on stdout")
	cmd.Flags().BoolVar(&validate, "validate", false, "run in-process fest validate on destination after unbundle")
	_ = cmd.MarkFlagRequired("dest")
	return cmd
}

func inferKind(source string) string {
	parts := strings.Split(filepath.ToSlash(source), "/")
	for _, p := range parts {
		if p == "ritual" {
			return festivalbundle.KindRitual
		}
	}
	if meta := loadFestYAMLMeta(source); meta != nil && meta.FestivalType == "ritual" {
		return festivalbundle.KindRitual
	}
	return festivalbundle.KindFestival
}

type festYAMLFile struct {
	Metadata festYAMLMeta `yaml:"metadata"`
}

type festYAMLMeta struct {
	ID           string    `yaml:"id"`
	UUID         string    `yaml:"uuid"`
	Name         string    `yaml:"name"`
	FestivalType string    `yaml:"festival_type"`
	CreatedAt    time.Time `yaml:"created_at"`
}

func loadFestYAMLMeta(source string) *festYAMLMeta {
	raw, err := os.ReadFile(filepath.Join(source, "fest.yaml"))
	if err != nil {
		return nil
	}
	var doc festYAMLFile
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	if doc.Metadata.ID == "" && doc.Metadata.Name == "" {
		return nil
	}
	return &doc.Metadata
}

func loadFestSubject(source string) *festivalbundle.SubjectMeta {
	m := loadFestYAMLMeta(source)
	if m == nil {
		return nil
	}
	sub := &festivalbundle.SubjectMeta{
		ID:    m.ID,
		UUID:  m.UUID,
		Type:  m.FestivalType,
		Title: m.Name,
	}
	if !m.CreatedAt.IsZero() {
		sub.CreatedAt = m.CreatedAt.UTC().Format(time.RFC3339)
	}
	return sub
}
