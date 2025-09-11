package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"golang.org/x/term"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatHuman OutputFormat = "human"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
)

// Write outputs the analysis in the specified format
func (a *AnalysisOutput) Write(format string, w io.Writer) error {
	// Auto-detect format if not specified
	if format == "" {
		if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			return a.WriteOutput(w)
		}
		format = "json"
	}

	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(a)
	case "yaml", "yml":
		data, err := yaml.Marshal(a)
		if err != nil {
			return fmt.Errorf("failed to marshal to YAML: %w", err)
		}
		_, err = w.Write(data)
		return err
	case "human", "":
		return a.WriteOutput(w)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteOutput outputs human-readable format
func (a *AnalysisOutput) WriteOutput(w io.Writer) error {
	// Header section
	header := fmt.Sprintf("\nPOM Analysis: %s\nTimestamp: %s\n%s\n",
		a.POMFile,
		a.Timestamp.Format("2006-01-02 15:04:05"),
		strings.Repeat("=", 60))
	if _, err := fmt.Fprint(w, header); err != nil {
		return err
	}

	// Dependencies summary
	deps := fmt.Sprintf("\nDependencies Summary:\n  Total: %d\n  Direct: %d\n  Using properties: %d\n",
		a.Dependencies.Total, a.Dependencies.Direct, a.Dependencies.UsingProperties)

	if a.Dependencies.FromBOMs > 0 {
		deps += fmt.Sprintf("  From BOMs: %d\n", a.Dependencies.FromBOMs)
	}
	if a.Dependencies.Transitive > 0 {
		deps += fmt.Sprintf("  Transitive: %d\n", a.Dependencies.Transitive)
	}

	if _, err := fmt.Fprint(w, deps); err != nil {
		return err
	}

	// BOMs
	if len(a.BOMs) > 0 {
		boms := "\nImported BOMs:\n"
		for _, bom := range a.BOMs {
			boms += fmt.Sprintf("  - %s:%s:%s\n", bom.GroupID, bom.ArtifactID, bom.Version)
		}
		if _, err := fmt.Fprint(w, boms); err != nil {
			return err
		}
	}

	// Properties
	if len(a.Properties.Defined) > 0 {
		props := "\nDefined Properties:\n"
		for prop, value := range a.Properties.Defined {
			if deps, ok := a.Properties.UsedBy[prop]; ok && len(deps) > 0 {
				props += fmt.Sprintf("  %s = %s (used by %d dependencies)\n", prop, value, len(deps))
			} else {
				props += fmt.Sprintf("  %s = %s\n", prop, value)
			}
		}
		if _, err := fmt.Fprint(w, props); err != nil {
			return err
		}
	}

	// Issues
	if len(a.Issues) > 0 {
		issues := fmt.Sprintf("\nIssues Found: %d\n%s\n", len(a.Issues), strings.Repeat("-", 40))
		for i, issue := range a.Issues {
			issues += fmt.Sprintf("\n%d. %s (%s)\n   Current: %s\n",
				i+1, issue.Dependency, issue.Type, issue.CurrentVersion)

			if issue.RequiredVersion != "" {
				issues += fmt.Sprintf("   Required: %s\n", issue.RequiredVersion)
			}
			if len(issue.CVEs) > 0 {
				issues += fmt.Sprintf("   CVEs: %s\n", strings.Join(issue.CVEs, ", "))
			}
			if len(issue.Path) > 0 {
				issues += fmt.Sprintf("   Path: %s\n", strings.Join(issue.Path, " -> "))
			}
			if issue.Property != "" {
				issues += fmt.Sprintf("   Property: ${%s}\n", issue.Property)
			}
		}
		if _, err := fmt.Fprint(w, issues); err != nil {
			return err
		}
	}

	// Patches
	if len(a.Patches) > 0 || len(a.PropertyUpdates) > 0 {
		patches := fmt.Sprintf("\nRecommended Patches:\n%s\n", strings.Repeat("-", 40))

		if len(a.PropertyUpdates) > 0 {
			patches += "\nProperty Updates:\n"
			for prop, value := range a.PropertyUpdates {
				current := a.Properties.Defined[prop]
				if current != "" {
					patches += fmt.Sprintf("  %s: %s -> %s\n", prop, current, value)
				} else {
					patches += fmt.Sprintf("  %s: (new) -> %s\n", prop, value)
				}

				// Show affected dependencies
				if deps, ok := a.Properties.UsedBy[prop]; ok && len(deps) > 0 {
					patches += fmt.Sprintf("    Affects: %s\n", strings.Join(deps, ", "))
				}
			}
		}

		if len(a.Patches) > 0 {
			patches += "\nDirect Dependency Updates:\n"
			for _, patch := range a.Patches {
				patches += fmt.Sprintf("  %s:%s -> %s\n", patch.GroupID, patch.ArtifactID, patch.Version)
			}
		}

		if _, err := fmt.Fprint(w, patches); err != nil {
			return err
		}
	}

	// Warnings
	if len(a.Warnings) > 0 {
		warnings := "\nWarnings:\n"
		for _, warning := range a.Warnings {
			warnings += fmt.Sprintf("  ⚠ %s\n", warning)
		}
		if _, err := fmt.Fprint(w, warnings); err != nil {
			return err
		}
	}

	// Cannot fix
	if len(a.CannotFix) > 0 {
		cannotFix := "\nCannot Fix (Manual Intervention Required):\n"
		for _, issue := range a.CannotFix {
			cannotFix += fmt.Sprintf("  ✗ %s\n    Reason: %s\n    Action: %s\n",
				issue.Dependency, issue.Reason, issue.Action)
		}
		if _, err := fmt.Fprint(w, cannotFix); err != nil {
			return err
		}
	}

	// Summary
	fixable := len(a.Patches) + len(a.PropertyUpdates)
	unfixable := len(a.CannotFix)
	summary := fmt.Sprintf("\nSummary:\n%s\n  Fixable issues: %d\n  Unfixable issues: %d\n",
		strings.Repeat("-", 40), fixable, unfixable)

	if fixable > 0 {
		summary += fmt.Sprintf("\n  Run 'pombump %s", a.POMFile)
		if len(a.PropertyUpdates) > 0 {
			summary += " --properties-file <file>"
		}
		if len(a.Patches) > 0 {
			summary += " --patch-file <file>"
		}
		summary += "' to apply patches\n"
	}

	if _, err := fmt.Fprint(w, summary); err != nil {
		return err
	}

	return nil
}
