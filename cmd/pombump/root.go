package pombump

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vaikas/pombump/pkg"
	"github.com/vifraa/gopom"
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
			fmt.Printf("CHECKING: %s\n", dep)
			parts := strings.Split(dep, "@")
			fmt.Printf("PARTS: %v %d\n", parts, len(parts))
			if len(parts) != 3 {
				return fmt.Errorf("invalid dependencies format (%s). Each dependency should be in the format <groupID@artifactID@version>. Usage: pombump --dependencies=\"<groupID@artifactID@version> <groupID@artifactID@version> ...\"", dep)
			}
			patches = append(patches, pkg.Patch{GroupID: parts[0], ArtifactID: parts[1], Version: parts[2]})
		}

		propertiesPatches := map[string]string{}
		for _, prop := range properties {
			if prop == "" {
				continue
			}
			fmt.Printf("CHECKING PROP: %s\n", prop)
			parts := strings.Split(prop, "@")
			if len(parts) != 2 {
				return fmt.Errorf("invalid properties format. Each dependency should be in the format <groupID@artifactID@version>. Usage: pombump --properties=\"<property@version> <property@version>\" ...\"")
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
		out, err := xml.Marshal(newPom)
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
	flagSet.StringVar(&rootFlags.dependencies, "dependencies", "", "A space-separated list of dependencies to update")
	flagSet.StringVar(&rootFlags.properties, "properties", "", "A space-separated list of properties to update")
}
