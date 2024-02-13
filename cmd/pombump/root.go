package pombump

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/2000Slash/gopom"
	"github.com/spf13/cobra"
	"github.com/vaikas/pombump/pkg"
	"sigs.k8s.io/release-utils/version"
)

type rootCLIFlags struct {
	dependencies string
	properties   string
}

var rootFlags rootCLIFlags

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pombump",
	Short: "pombump cli",
	Args:  cobra.ExactArgs(1),
	// Uncomment the following line if your bare application
	// has an action associated with it:
	RunE: func(cmd *cobra.Command, args []string) error {
		if rootFlags.dependencies == "" && rootFlags.properties == "" {
			return fmt.Errorf("no dependencies or properties provided. Usage: pombump --dependencies=\"<groupID@artifactID@version> <groupID@artifactID@version> --properties=\"<property@version> <property@version>\"...\"")
		}
		dependencies := strings.Split(rootFlags.dependencies, " ")
		properties := strings.Split(rootFlags.properties, " ")

		patches := []pkg.Patch{}
		for _, dep := range dependencies {
			if dep == "" {
				continue
			}
			parts := strings.Split(dep, "@")
			if len(parts) < 3 {
				return fmt.Errorf("invalid dependencies format (%s). Each dependency should be in the format <groupID@artifactID@version>. Usage: pombump --dependencies=\"<groupID@artifactID@version> <groupID@artifactID@version> ...\"", dep)
			}
			scope := "import"
			if len(parts) == 4 {
				scope = parts[3]
			}
			patches = append(patches, pkg.Patch{GroupID: parts[0], ArtifactID: parts[1], Version: parts[2], Scope: scope})
		}

		propertiesPatches := map[string]string{}
		for _, prop := range properties {
			if prop == "" {
				continue
			}
			parts := strings.Split(prop, "@")
			if len(parts) != 2 {
				return fmt.Errorf("invalid properties format. Each dependency should be in the format <property@value>. Usage: pombump --properties=\"<property@value> <property@value>\" ...\"")
			}
			propertiesPatches[parts[0]] = parts[1]
		}
		parsedPom, err := gopom.Parse(args[0])
		if err != nil {
			return fmt.Errorf("failed to parse the pom file: %w", err)
		}

		newPom, err := pkg.PatchProject(parsedPom, patches, propertiesPatches)
		if err != nil {
			return fmt.Errorf("failed to patch the pom file: %w", err)
		}
		out, err := xml.MarshalIndent(newPom, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal the pom file: %w", err)
		}
		fmt.Println(string(out))
		return nil
	},
}

func RootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.AddCommand(version.WithFont("starwars"))

	rootCmd.DisableAutoGenTag = true

	flagSet := rootCmd.Flags()
	flagSet.StringVar(&rootFlags.dependencies, "dependencies", "", "A space-separated list of dependencies to update in form groupID@artifactID@version")
	flagSet.StringVar(&rootFlags.properties, "properties", "", "A space-separated list of properties to update in form property@value")
}
