package pkg

import (
	"context"
	"testing"

	"github.com/chainguard-dev/gopom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeProject(t *testing.T) {
	tests := []struct {
		name               string
		project            *gopom.Project
		expectedDeps       int
		expectedPropDeps   int
		expectedProps      int
		expectedPropCounts map[string]int
	}{
		{
			name: "project with property-based dependencies",
			project: &gopom.Project{
				Properties: &gopom.Properties{
					Entries: map[string]string{
						"netty.version": "4.1.94.Final",
						"slf4j.version": "1.7.30",
					},
				},
				Dependencies: &[]gopom.Dependency{
					{
						GroupID:    "io.netty",
						ArtifactID: "netty-handler",
						Version:    "${netty.version}",
					},
					{
						GroupID:    "io.netty",
						ArtifactID: "netty-codec",
						Version:    "${netty.version}",
					},
					{
						GroupID:    "org.slf4j",
						ArtifactID: "slf4j-api",
						Version:    "${slf4j.version}",
					},
					{
						GroupID:    "junit",
						ArtifactID: "junit",
						Version:    "4.13.2",
					},
				},
			},
			expectedDeps:     4,
			expectedPropDeps: 3,
			expectedProps:    2,
			expectedPropCounts: map[string]int{
				"netty.version": 2,
				"slf4j.version": 1,
			},
		},
		{
			name: "project with dependency management",
			project: &gopom.Project{
				Properties: &gopom.Properties{
					Entries: map[string]string{
						"jackson.version": "2.15.2",
					},
				},
				DependencyManagement: &gopom.DependencyManagement{
					Dependencies: &[]gopom.Dependency{
						{
							GroupID:    "com.fasterxml.jackson.core",
							ArtifactID: "jackson-databind",
							Version:    "${jackson.version}",
						},
						{
							GroupID:    "com.fasterxml.jackson.core",
							ArtifactID: "jackson-core",
							Version:    "${jackson.version}",
						},
					},
				},
			},
			expectedDeps:     2,
			expectedPropDeps: 2,
			expectedProps:    1,
			expectedPropCounts: map[string]int{
				"jackson.version": 2,
			},
		},
		{
			name: "project with no properties",
			project: &gopom.Project{
				Dependencies: &[]gopom.Dependency{
					{
						GroupID:    "org.apache.commons",
						ArtifactID: "commons-lang3",
						Version:    "3.12.0",
					},
				},
			},
			expectedDeps:       1,
			expectedPropDeps:   0,
			expectedProps:      0,
			expectedPropCounts: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := AnalyzeProject(ctx, tt.project)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedDeps, len(result.Dependencies), "unexpected number of dependencies")

			propDepsCount := 0
			for _, dep := range result.Dependencies {
				if dep.UsesProperty {
					propDepsCount++
				}
			}
			assert.Equal(t, tt.expectedPropDeps, propDepsCount, "unexpected number of property-based dependencies")

			assert.Equal(t, tt.expectedProps, len(result.Properties), "unexpected number of properties")

			for prop, count := range tt.expectedPropCounts {
				assert.Equal(t, count, result.PropertyUsageCounts[prop], "unexpected usage count for property %s", prop)
			}
		})
	}
}

func TestShouldUseProperty(t *testing.T) {
	result := &AnalysisResult{
		Dependencies: map[string]*DependencyInfo{
			"io.netty:netty-handler": {
				GroupID:      "io.netty",
				ArtifactID:   "netty-handler",
				Version:      "${netty.version}",
				UsesProperty: true,
				PropertyName: "netty.version",
			},
			"junit:junit": {
				GroupID:      "junit",
				ArtifactID:   "junit",
				Version:      "4.13.2",
				UsesProperty: false,
			},
		},
	}

	tests := []struct {
		name           string
		groupID        string
		artifactID     string
		expectProperty bool
		propertyName   string
	}{
		{
			name:           "dependency with property",
			groupID:        "io.netty",
			artifactID:     "netty-handler",
			expectProperty: true,
			propertyName:   "netty.version",
		},
		{
			name:           "dependency without property",
			groupID:        "junit",
			artifactID:     "junit",
			expectProperty: false,
			propertyName:   "",
		},
		{
			name:           "non-existent dependency",
			groupID:        "org.example",
			artifactID:     "not-found",
			expectProperty: false,
			propertyName:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useProperty, propName := result.ShouldUseProperty(tt.groupID, tt.artifactID)
			assert.Equal(t, tt.expectProperty, useProperty)
			assert.Equal(t, tt.propertyName, propName)
		})
	}
}

func TestPatchStrategy(t *testing.T) {
	ctx := context.Background()

	result := &AnalysisResult{
		Dependencies: map[string]*DependencyInfo{
			"io.netty:netty-handler": {
				GroupID:      "io.netty",
				ArtifactID:   "netty-handler",
				Version:      "${netty.version}",
				UsesProperty: true,
				PropertyName: "netty.version",
			},
			"io.netty:netty-codec": {
				GroupID:      "io.netty",
				ArtifactID:   "netty-codec",
				Version:      "${netty.version}",
				UsesProperty: true,
				PropertyName: "netty.version",
			},
			"junit:junit": {
				GroupID:      "junit",
				ArtifactID:   "junit",
				Version:      "4.13.2",
				UsesProperty: false,
			},
		},
		Properties: map[string]string{
			"netty.version": "4.1.94.Final",
		},
	}

	patches := []Patch{
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-handler",
			Version:    "4.1.118.Final",
		},
		{
			GroupID:    "junit",
			ArtifactID: "junit",
			Version:    "4.13.3",
		},
		{
			GroupID:    "org.example",
			ArtifactID: "new-dep",
			Version:    "1.0.0",
		},
	}

	directPatches, propertyPatches := PatchStrategy(ctx, result, patches)

	// Should have 2 direct patches (junit update and new dependency)
	assert.Len(t, directPatches, 2)

	// Should have 1 property update (netty.version)
	assert.Len(t, propertyPatches, 1)
	assert.Equal(t, "4.1.118.Final", propertyPatches["netty.version"])

	// Verify direct patches
	foundJunit := false
	foundNewDep := false
	for _, p := range directPatches {
		if p.GroupID == "junit" && p.ArtifactID == "junit" {
			foundJunit = true
			assert.Equal(t, "4.13.3", p.Version)
		}
		if p.GroupID == "org.example" && p.ArtifactID == "new-dep" {
			foundNewDep = true
			assert.Equal(t, "1.0.0", p.Version)
		}
	}
	assert.True(t, foundJunit, "junit patch not found")
	assert.True(t, foundNewDep, "new dependency patch not found")
}

func TestGetAffectedDependencies(t *testing.T) {
	result := &AnalysisResult{
		Dependencies: map[string]*DependencyInfo{
			"io.netty:netty-handler": {
				GroupID:      "io.netty",
				ArtifactID:   "netty-handler",
				UsesProperty: true,
				PropertyName: "netty.version",
			},
			"io.netty:netty-codec": {
				GroupID:      "io.netty",
				ArtifactID:   "netty-codec",
				UsesProperty: true,
				PropertyName: "netty.version",
			},
			"org.slf4j:slf4j-api": {
				GroupID:      "org.slf4j",
				ArtifactID:   "slf4j-api",
				UsesProperty: true,
				PropertyName: "slf4j.version",
			},
		},
	}

	affected := result.GetAffectedDependencies("netty.version")
	assert.Len(t, affected, 2)

	for _, dep := range affected {
		assert.Equal(t, "io.netty", dep.GroupID)
		assert.Contains(t, []string{"netty-handler", "netty-codec"}, dep.ArtifactID)
	}

	affectedSlf4j := result.GetAffectedDependencies("slf4j.version")
	assert.Len(t, affectedSlf4j, 1)
	assert.Equal(t, "org.slf4j", affectedSlf4j[0].GroupID)

	affectedNone := result.GetAffectedDependencies("non.existent")
	assert.Len(t, affectedNone, 0)
}
