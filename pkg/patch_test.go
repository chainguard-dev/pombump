package pkg

import (
	"context"
	"fmt"
	"testing"

	"github.com/2000Slash/gopom"
)

func makeDep(groupID, artifactID, version string) gopom.Dependency {
	return gopom.Dependency{GroupID: groupID, ArtifactID: artifactID, Version: version}
}

func TestSimplePoms(t *testing.T) {
	testCases := []struct {
		name    string
		in      *gopom.Project
		patches []Patch
		props   map[string]string
		want    *gopom.Project
	}{{
		name:    "simple dependency, bumped inline",
		in:      &gopom.Project{Dependencies: &[]gopom.Dependency{makeDep("a1", "b1", "1.0.0")}},
		patches: []Patch{{"a1", "b1", "1.0.1", "import"}},
		want:    &gopom.Project{Dependencies: &[]gopom.Dependency{makeDep("a1", "b1", "1.0.1")}},
	}, {
		name:    "simple dependencymanagement, bumped inline",
		in:      &gopom.Project{DependencyManagement: &gopom.DependencyManagement{Dependencies: &[]gopom.Dependency{makeDep("a2", "b2", "2.0.0")}}},
		patches: []Patch{{"a2", "b2", "2.0.1", "import"}},
		want:    &gopom.Project{DependencyManagement: &gopom.DependencyManagement{Dependencies: &[]gopom.Dependency{makeDep("a2", "b2", "2.0.1")}}},
	}, {
		name:    "dependencymanagement, added to dependency management",
		in:      &gopom.Project{DependencyManagement: &gopom.DependencyManagement{Dependencies: &[]gopom.Dependency{makeDep("other", "b3", "2.0.0")}}},
		patches: []Patch{{"added", "b", "2.0.1", "import"}},
		want:    &gopom.Project{DependencyManagement: &gopom.DependencyManagement{Dependencies: &[]gopom.Dependency{makeDep("added", "b", "2.0.1")}}},
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			got, err := PatchProject(context.Background(), in, tc.patches, tc.props)
			if err != nil {
				t.Errorf("%s: Failed to patch %+v: %v", tc.name, tc.in, err)
			}
			diffProject(t, got, tc.want)
		})
	}
}

func TestPatchesFromPomFiles(t *testing.T) {
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
		// DependencyManagement.dependencies.
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
			got, err := PatchProject(context.Background(), parsedPom, tc.patches, tc.props)
			if err != nil {
				t.Errorf("%s: Failed to patch %s: %v", tc.name, tc.in, err)
			}

			checkDependencies(t, got, tc.wantDeps)
			checkDMDependencies(t, got, tc.wantDMDeps)
			checkProps(t, got, tc.wantProps)
			t.Logf("Doing the second checks!!!")
			checkDependencies(t, parsedPom, tc.wantDeps)
			checkDMDependencies(t, parsedPom, tc.wantDMDeps)
			checkProps(t, parsedPom, tc.wantProps)
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
			if dep.ArtifactID == patch.ArtifactID &&
				dep.GroupID == patch.GroupID {
				if dep.Version != patch.Version {
					t.Errorf("dep %s.%s version %s != %s", patch.GroupID, patch.ArtifactID, dep.Version, patch.Version)
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

func diffDeps(t *testing.T, got, want *[]gopom.Dependency) {
	if got == nil && want == nil {
		return
	}
	if got == nil && want != nil {
		t.Errorf("dependencies is nil but there are (%d) expected dependencies", len(*want))
		return
	}
	if want == nil && got != nil {
		t.Errorf("want dependencies is nil but there are (%d) expected dependencies", len(*got))
		return
	}
	// In addition to version mismatches, make sure we are not missing any
	// dependencies that should be there. Knock them off of this when we find
	// them, regardless of whether the version is matched or not.
	missing := make(map[gopom.Dependency]gopom.Dependency, len(*want))
	for _, p := range *want {
		missing[p] = p
	}
	for _, dep := range *got {
		for _, patch := range *want {
			if dep.ArtifactID == patch.ArtifactID &&
				dep.GroupID == patch.GroupID {
				if dep.Version != patch.Version {
					t.Errorf("dep %s.%s version %s != %s", patch.GroupID, patch.ArtifactID, dep.Version, patch.Version)
				}
				delete(missing, patch)
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("missing dependencies: %+v", missing)
	}
}

func diffDMs(t *testing.T, got, want *gopom.DependencyManagement) {
	switch {
	case got == nil && want == nil:
		return
	case got == nil && want != nil:
		if want.Dependencies != nil {
			t.Errorf("dependencies is nil but there are (%d) expected dependencies", len(*want.Dependencies))
		}
		return
	case got != nil && want == nil:
		if got.Dependencies != nil {
			t.Errorf("expected is nil but there are (%d) dependencies", len(*got.Dependencies))
		}
		return
	case got.Dependencies == nil && want.Dependencies == nil:
		return
	case got.Dependencies == nil && want.Dependencies != nil:
		t.Errorf("depe ndencies is nil but there are (%d) expected dependencies", len(*want.Dependencies))
		return
	case got.Dependencies != nil && want.Dependencies == nil:
		t.Errorf("expected is nil but there are (%d) dependencies", len(*got.Dependencies))
		return
	}
	diffDeps(t, got.Dependencies, want.Dependencies)
}

func diffProject(t *testing.T, got, want *gopom.Project) {
	t.Logf("Diffing %+v %+v", got, want)
	diffDeps(t, got.Dependencies, want.Dependencies)
	diffDMs(t, got.DependencyManagement, want.DependencyManagement)
}
