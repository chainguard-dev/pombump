package pombump

import (
	"fmt"
	"log/slog"
	"os"

	charmlog "github.com/charmbracelet/log"

	"github.com/chainguard-dev/gopom"
	"github.com/chainguard-dev/pombump/pkg"
	"github.com/spf13/cobra"
	"sigs.k8s.io/release-utils/version"
)

type rootCLIFlags struct {
	dependencies   string
	properties     string
	patchFile      string
	propertiesFile string
}

var rootFlags rootCLIFlags

func New() *cobra.Command {
	var logPolicy []string
	var levelStr string

	cmd := &cobra.Command{
		Use:   "pombump <file-to-bump>",
		Short: "pombump cli",
		Args:  cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Simple log writer setup (replace apko log.Writer)
			out := os.Stderr
			for _, policy := range logPolicy {
				if policy != "builtin:stderr" {
					f, err := os.OpenFile(policy, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
					if err != nil {
						return fmt.Errorf("failed to create log writer: %w", err)
					}
					out = f
					break
				}
			}
			// Parse log level
			var level charmlog.Level
			switch levelStr {
			case "debug":
				level = charmlog.DebugLevel
			case "info":
				level = charmlog.InfoLevel
			case "warn":
				level = charmlog.WarnLevel
			case "error":
				level = charmlog.ErrorLevel
			default:
				level = charmlog.InfoLevel
			}
			slog.SetDefault(slog.New(charmlog.NewWithOptions(out, charmlog.Options{ReportTimestamp: true, Level: level})))

			return nil
		},

		// Uncomment the following line if your bare application
		// has an action associated with it:
		RunE: func(cmd *cobra.Command, args []string) error {
			if rootFlags.dependencies == "" && rootFlags.properties == "" &&
				rootFlags.patchFile == "" && rootFlags.propertiesFile == "" {
				return fmt.Errorf("no dependencies or properties provides, use --dependencies/--patch-file or --properties/properties-file")
			}

			if rootFlags.patchFile != "" && rootFlags.dependencies != "" {
				return fmt.Errorf("use either --dependencies or --patch-file")
			}
			if rootFlags.propertiesFile != "" && rootFlags.properties != "" {
				return fmt.Errorf("use either --properties or --properties-file")
			}

			patches, err := pkg.ParsePatches(cmd.Context(), rootFlags.patchFile, rootFlags.dependencies)
			if err != nil {
				return fmt.Errorf("failed to parse patches: %w", err)
			}

			propertiesPatches, err := pkg.ParseProperties(cmd.Context(), rootFlags.propertiesFile, rootFlags.properties)
			if err != nil {
				return fmt.Errorf("failed to parse properties: %w", err)
			}

			parsedPom, err := gopom.Parse(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse the pom file: %w", err)
			}

			newPom, err := pkg.PatchProject(cmd.Context(), parsedPom, patches, propertiesPatches)
			if err != nil {
				return fmt.Errorf("failed to patch the pom file: %w", err)
			}

			out, err := newPom.Marshal()
			if err != nil {
				return fmt.Errorf("failed to marshal the pom file: %w", err)
			}
			fmt.Println(string(out))
			return nil
		},
	}
	cmd.PersistentFlags().StringSliceVar(&logPolicy, "log-policy", []string{"builtin:stderr"}, "log policy (e.g. builtin:stderr, /tmp/log/foo)")
	cmd.PersistentFlags().StringVar(&levelStr, "log-level", "info", "log level (e.g. debug, info, warn, error)")

	cmd.AddCommand(version.WithFont("starwars"))
	cmd.AddCommand(AnalyzeCmd())

	cmd.DisableAutoGenTag = true

	flagSet := cmd.Flags()
	flagSet.StringVar(&rootFlags.dependencies, "dependencies", "", "A space-separated list of dependencies to update in form groupID@artifactID@version")
	flagSet.StringVar(&rootFlags.properties, "properties", "", "A space-separated list of properties to update in form property@value")
	flagSet.StringVar(&rootFlags.patchFile, "patch-file", "", "The input file to read patches from")
	flagSet.StringVar(&rootFlags.propertiesFile, "properties-file", "", "The input file to read properties from")
	return cmd
}
