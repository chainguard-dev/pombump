package pkg

import (
	"fmt"
	"testing"

	"github.com/vifraa/gopom"
)

func TestPatches(t *testing.T) {
	testCases := []struct {
		name       string
		in         string
		patches    []Patch
		props      map[string]string
		wantDeps   []Patch
		wantDMDeps []Patch
		wantProps  map[string]string
	}{{
		// This adds new dependencies to the project. They end up in
		// DependencyManagement.dependencies. <- Is that right?
		// There's one patch for existing one that's not in our current patches
		// but test with it.
		name:    "trino - dependency patch - add new ones and replace existing",
		in:      "trino.pom.xml",
		patches: []Patch{{GroupID: "io.projectreactor.netty", ArtifactID: "reactor-netty-http", Version: "1.0.39"}, {GroupID: "org.json", ArtifactID: "json", Version: "20231013"}, {ArtifactID: "ch.qos.logback", GroupID: "logback-core", Version: "[1.4.12,2.0.0)"}, {GroupID: "com.azure", ArtifactID: "azure-sdk-bom", Version: "1.2.19"}},

		wantDMDeps: []Patch{{GroupID: "io.projectreactor.netty", ArtifactID: "reactor-netty-http", Version: "1.0.39"}, {GroupID: "org.json", ArtifactID: "json", Version: "20231013"}, {ArtifactID: "ch.qos.logback", GroupID: "logback-core", Version: "[1.4.12,2.0.0)"}, {GroupID: "com.azure", ArtifactID: "azure-sdk-bom", Version: "1.2.19"}},
	}, {
		// This patches existing dependencies in a project, but they are
		// specified in the 'properties' section.
		name:      "zookeeper - properties patch",
		in:        "zookeeper.pom.xml",
		props:     map[string]string{"logback-version": "1.2.13", "jetty.version": "9.4.53.v20231009"},
		wantProps: map[string]string{"logback-version": "1.2.13", "jetty.version": "9.4.53.v20231009"},
	}, {
		// This patches existing dependency in a project
		name:     "cloudwatch-exporter - dependency patch - existing",
		in:       "cloudwatch-exporter.pom.xml",
		patches:  []Patch{{GroupID: "org.eclipse.jetty", ArtifactID: "jetty-servlet", Version: "11.0.16"}},
		wantDeps: []Patch{{GroupID: "org.eclipse.jetty", ArtifactID: "jetty-servlet", Version: "11.0.16"}},
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedPom, err := gopom.Parse(fmt.Sprintf("testdata/%s", tc.in))
			if err != nil {
				t.Fatal(err)
			}
			got, err := PatchProject(parsedPom, tc.patches, tc.props)
			if err != nil {
				t.Errorf("%s: Failed to parse %s: %v", tc.name, tc.in, err)
			}

			checkDependencies(t, got, tc.wantDeps)
			checkDMDependencies(t, got, tc.wantDMDeps)
			checkProps(t, got, tc.wantProps)
		})
	}
}

// This is a helper function to check dependencies in a list of dependencies.
// Because the deps can live in 'explicit depencies' or
// 'dependencyManagement.dependencies', this just shares the main loop.
func checkDeps(t *testing.T, indeps *[]gopom.Dependency, wantdeps []Patch) {
	if indeps == nil {
		if len(wantdeps) > 0 {
			t.Errorf("dependencies is nil but there are (%d) expected dependencies", len(wantdeps))
		}
		return
	}

	// In addition to version mismatches, make sure we are not missing any
	// dependencies that should be there. Knock them off of this when we find
	// them, regardless of whether the version is matched or not.
	missing := make(map[Patch]Patch, len(wantdeps))
	for _, p := range wantdeps {
		missing[p] = p
	}
	for _, dep := range *indeps {
		for _, patch := range wantdeps {
			if *dep.ArtifactID == patch.ArtifactID &&
				*dep.GroupID == patch.GroupID {
				if *dep.Version != patch.Version {
					t.Errorf("dep %s.%s version %s != %s", patch.GroupID, patch.ArtifactID, *dep.Version, patch.Version)
				}
				delete(missing, patch)
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("missing dependencies: %+v", missing)
	}
}

func checkDependencies(t *testing.T, project *gopom.Project, deps []Patch) {
	checkDeps(t, project.Dependencies, deps)
}

func checkDMDependencies(t *testing.T, project *gopom.Project, deps []Patch) {
	checkDeps(t, project.DependencyManagement.Dependencies, deps)
}

func checkProps(t *testing.T, project *gopom.Project, props map[string]string) {
	for k, v := range props {
		if project.Properties.Entries[k] != v {
			t.Errorf("property %s value %s != %s", k, project.Properties.Entries[k], v)
		}
	}
}
