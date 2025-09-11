package pkg

import (
	"context"
	"testing"

	"github.com/chainguard-dev/gopom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBOMDetection(t *testing.T) {
	tests := []struct {
		name               string
		project            *gopom.Project
		expectedBOMs       int
		expectedBOMDetails []BOMInfo
	}{
		{
			name: "project with Spring Boot and AWS BOMs",
			project: &gopom.Project{
				DependencyManagement: &gopom.DependencyManagement{
					Dependencies: &[]gopom.Dependency{
						{
							GroupID:    "org.springframework.boot",
							ArtifactID: "spring-boot-dependencies",
							Version:    "2.7.18",
							Type:       "pom",
							Scope:      "import",
						},
						{
							GroupID:    "com.amazonaws",
							ArtifactID: "aws-java-sdk-bom",
							Version:    "1.12.400",
							Type:       "pom",
							Scope:      "import",
						},
						{
							// Regular dependency management entry (not a BOM)
							GroupID:    "com.fasterxml.jackson.core",
							ArtifactID: "jackson-databind",
							Version:    "2.15.2",
						},
					},
				},
			},
			expectedBOMs: 2,
			expectedBOMDetails: []BOMInfo{
				{
					GroupID:    "org.springframework.boot",
					ArtifactID: "spring-boot-dependencies",
					Version:    "2.7.18",
					Type:       "pom",
					Scope:      "import",
				},
				{
					GroupID:    "com.amazonaws",
					ArtifactID: "aws-java-sdk-bom",
					Version:    "1.12.400",
					Type:       "pom",
					Scope:      "import",
				},
			},
		},
		{
			name: "project with no BOMs",
			project: &gopom.Project{
				DependencyManagement: &gopom.DependencyManagement{
					Dependencies: &[]gopom.Dependency{
						{
							GroupID:    "junit",
							ArtifactID: "junit",
							Version:    "4.13.2",
						},
					},
				},
			},
			expectedBOMs:       0,
			expectedBOMDetails: []BOMInfo{},
		},
		{
			name: "project with mixed BOMs and regular dependencies",
			project: &gopom.Project{
				DependencyManagement: &gopom.DependencyManagement{
					Dependencies: &[]gopom.Dependency{
						{
							GroupID:    "io.quarkus",
							ArtifactID: "quarkus-bom",
							Version:    "3.0.0",
							Type:       "pom",
							Scope:      "import",
						},
						{
							GroupID:    "org.slf4j",
							ArtifactID: "slf4j-api",
							Version:    "1.7.30",
						},
						{
							GroupID:    "org.apache.camel",
							ArtifactID: "camel-bom",
							Version:    "3.20.0",
							Type:       "pom",
							Scope:      "import",
						},
					},
				},
			},
			expectedBOMs: 2,
			expectedBOMDetails: []BOMInfo{
				{
					GroupID:    "io.quarkus",
					ArtifactID: "quarkus-bom",
					Version:    "3.0.0",
					Type:       "pom",
					Scope:      "import",
				},
				{
					GroupID:    "org.apache.camel",
					ArtifactID: "camel-bom",
					Version:    "3.20.0",
					Type:       "pom",
					Scope:      "import",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := AnalyzeProject(ctx, tt.project)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedBOMs, len(result.BOMs),
				"Expected %d BOMs but found %d", tt.expectedBOMs, len(result.BOMs))

			if tt.expectedBOMs > 0 {
				assert.ElementsMatch(t, tt.expectedBOMDetails, result.BOMs,
					"BOM details don't match")
			}
		})
	}
}

func TestIsBOMImport(t *testing.T) {
	tests := []struct {
		name     string
		dep      gopom.Dependency
		expected bool
	}{
		{
			name: "valid BOM import",
			dep: gopom.Dependency{
				GroupID:    "org.springframework.boot",
				ArtifactID: "spring-boot-dependencies",
				Version:    "2.7.18",
				Type:       "pom",
				Scope:      "import",
			},
			expected: true,
		},
		{
			name: "pom type but not import scope",
			dep: gopom.Dependency{
				GroupID:    "org.springframework.boot",
				ArtifactID: "spring-boot-dependencies",
				Version:    "2.7.18",
				Type:       "pom",
				Scope:      "compile",
			},
			expected: false,
		},
		{
			name: "import scope but not pom type",
			dep: gopom.Dependency{
				GroupID:    "org.springframework.boot",
				ArtifactID: "spring-boot-dependencies",
				Version:    "2.7.18",
				Type:       "jar",
				Scope:      "import",
			},
			expected: false,
		},
		{
			name: "regular dependency",
			dep: gopom.Dependency{
				GroupID:    "junit",
				ArtifactID: "junit",
				Version:    "4.13.2",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBOMImport(tt.dep)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalysisOutputConversion(t *testing.T) {
	ctx := context.Background()

	project := &gopom.Project{
		Properties: &gopom.Properties{
			Entries: map[string]string{
				"jackson.version": "2.15.2",
				"netty.version":   "4.1.90.Final",
			},
		},
		DependencyManagement: &gopom.DependencyManagement{
			Dependencies: &[]gopom.Dependency{
				{
					GroupID:    "org.springframework.boot",
					ArtifactID: "spring-boot-dependencies",
					Version:    "2.7.18",
					Type:       "pom",
					Scope:      "import",
				},
			},
		},
		Dependencies: &[]gopom.Dependency{
			{
				GroupID:    "com.fasterxml.jackson.core",
				ArtifactID: "jackson-databind",
				Version:    "${jackson.version}",
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "${netty.version}",
			},
		},
	}

	result, err := AnalyzeProject(ctx, project)
	require.NoError(t, err)

	// Create some patches
	patches := []Patch{
		{
			GroupID:    "junit",
			ArtifactID: "junit",
			Version:    "4.13.2",
		},
	}

	propertyPatches := map[string]string{
		"jackson.version": "2.15.3",
	}

	output := result.ToAnalysisOutput("/test/pom.xml", patches, propertyPatches)

	assert.Equal(t, "/test/pom.xml", output.POMFile)
	assert.Equal(t, 2, output.Dependencies.Total)
	assert.Equal(t, 2, output.Dependencies.UsingProperties)
	assert.Equal(t, 1, len(output.BOMs))
	assert.Equal(t, "org.springframework.boot", output.BOMs[0].GroupID)
	assert.Equal(t, 1, len(output.Patches))
	assert.Equal(t, 1, len(output.PropertyUpdates))
	assert.Equal(t, "2.15.3", output.PropertyUpdates["jackson.version"])

	// Check property usage mapping
	assert.Contains(t, output.Properties.UsedBy["jackson.version"],
		"com.fasterxml.jackson.core:jackson-databind")
	assert.Contains(t, output.Properties.UsedBy["netty.version"],
		"io.netty:netty-handler")
}

// TestNeo4jStyleBOMScenario tests the exact scenario described where automation
// was creating inconsistent individual patches instead of BOM updates
func TestNeo4jStyleBOMScenario(t *testing.T) {
	ctx := context.Background()

	// Simulate Neo4j project structure with netty BOM
	project := &gopom.Project{
		DependencyManagement: &gopom.DependencyManagement{
			Dependencies: &[]gopom.Dependency{
				{
					GroupID:    "io.netty",
					ArtifactID: "netty-bom",
					Version:    "4.1.94.Final",
					Type:       "pom",
					Scope:      "import",
				},
			},
		},
		Dependencies: &[]gopom.Dependency{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.94.Final", // Managed by BOM
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-codec-http2",
				Version:    "4.1.94.Final", // Managed by BOM
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-codec-compression",
				Version:    "4.1.94.Final", // Managed by BOM
			},
			{
				GroupID:    "org.junit.jupiter",
				ArtifactID: "junit-jupiter",
				Version:    "5.8.2", // Not managed by BOM
			},
		},
	}

	result, err := AnalyzeProject(ctx, project)
	require.NoError(t, err)

	// Should detect the netty BOM
	require.Len(t, result.BOMs, 1)
	assert.Equal(t, "io.netty", result.BOMs[0].GroupID)
	assert.Equal(t, "netty-bom", result.BOMs[0].ArtifactID)

	t.Run("old behavior - inconsistent individual patches", func(t *testing.T) {
		// This simulates what the automation was doing wrong:
		// Creating individual patches with different versions
		problemPatches := []Patch{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-codec-http2",
				Version:    "4.1.100.Final", // Different version!
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-codec-compression",
				Version:    "4.1.118.Final", // Different version!
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.105.Final", // Different version!
			},
			{
				GroupID:    "org.junit.jupiter",
				ArtifactID: "junit-jupiter",
				Version:    "5.9.0", // This one is fine
			},
		}

		directPatches, propertyPatches := PatchStrategy(ctx, result, problemPatches)

		// BOM-first strategy should detect the version conflicts
		// and recommend a single BOM update instead of individual patches
		assert.Len(t, propertyPatches, 0) // No property updates needed

		// Should have:
		// 1. BOM update recommendation (netty-bom -> 4.1.118.Final)
		// 2. junit direct patch (not part of netty group)
		assert.Len(t, directPatches, 2)

		// Find the BOM recommendation
		var bomPatch *Patch
		var junitPatch *Patch
		for i, patch := range directPatches {
			if patch.GroupID == "io.netty" && patch.ArtifactID == "netty-bom" {
				bomPatch = &directPatches[i]
			} else if patch.GroupID == "org.junit.jupiter" && patch.ArtifactID == "junit-jupiter" {
				junitPatch = &directPatches[i]
			}
		}

		// Should recommend BOM update with highest version
		require.NotNil(t, bomPatch, "Expected BOM update recommendation")
		assert.Equal(t, "io.netty", bomPatch.GroupID)
		assert.Equal(t, "netty-bom", bomPatch.ArtifactID)
		assert.Equal(t, "4.1.118.Final", bomPatch.Version) // Highest of the requested versions
		assert.Equal(t, "pom", bomPatch.Type)
		assert.Equal(t, "import", bomPatch.Scope)

		// Should still have junit patch
		require.NotNil(t, junitPatch, "Expected junit patch")
		assert.Equal(t, "5.9.0", junitPatch.Version)
	})

	t.Run("ideal behavior - consistent versions", func(t *testing.T) {
		// This simulates what should happen with consistent versions
		consistentPatches := []Patch{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-codec-http2",
				Version:    "4.1.118.Final", // Same version
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-codec-compression",
				Version:    "4.1.118.Final", // Same version
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.118.Final", // Same version
			},
			{
				GroupID:    "org.junit.jupiter",
				ArtifactID: "junit-jupiter",
				Version:    "5.9.0",
			},
		}

		directPatches, propertyPatches := PatchStrategy(ctx, result, consistentPatches)

		// With consistent versions, no version conflicts should be detected
		// So it should fall back to normal direct patching
		assert.Len(t, propertyPatches, 0)
		assert.Len(t, directPatches, 4) // All as direct patches

		// Should not recommend BOM update
		foundBomPatch := false
		for _, patch := range directPatches {
			if patch.ArtifactID == "netty-bom" {
				foundBomPatch = true
			}
		}
		assert.False(t, foundBomPatch, "Should not recommend BOM update for consistent versions")
	})
}

// TestDetectVersionConflicts tests the core conflict detection logic
func TestDetectVersionConflicts(t *testing.T) {
	ctx := context.Background()

	result := &AnalysisResult{
		BOMs: []BOMInfo{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.94.Final",
				Type:       "pom",
				Scope:      "import",
			},
		},
	}

	t.Run("detects version conflicts", func(t *testing.T) {
		patches := []Patch{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.100.Final",
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-codec",
				Version:    "4.1.118.Final",
			},
			{
				GroupID:    "junit",
				ArtifactID: "junit",
				Version:    "4.13.3",
			},
		}

		conflicts := detectVersionConflicts(ctx, result, patches)

		// Should detect one conflict for io.netty group
		assert.Len(t, conflicts, 1)

		conflict := conflicts[0]
		assert.Equal(t, "io.netty", conflict.GroupID)
		assert.Equal(t, "update_bom", conflict.RecommendedAction)
		assert.NotNil(t, conflict.BOMCandidate)
		assert.Equal(t, "netty-bom", conflict.BOMCandidate.ArtifactID)

		// Should have both versions in requested versions
		assert.Equal(t, "4.1.100.Final", conflict.RequestedVersions["netty-handler"])
		assert.Equal(t, "4.1.118.Final", conflict.RequestedVersions["netty-codec"])
	})

	t.Run("no conflicts with same versions", func(t *testing.T) {
		patches := []Patch{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.118.Final",
			},
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-codec",
				Version:    "4.1.118.Final",
			},
		}

		conflicts := detectVersionConflicts(ctx, result, patches)
		assert.Len(t, conflicts, 0)
	})
}

// TestFindBOMForGroup tests BOM pattern matching logic
func TestFindBOMForGroup(t *testing.T) {
	result := &AnalysisResult{
		BOMs: []BOMInfo{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.94.Final",
			},
			{
				GroupID:    "org.springframework",
				ArtifactID: "spring-framework-bom",
				Version:    "5.3.21",
			},
			{
				GroupID:    "com.example",
				ArtifactID: "netty-bom", // Different group but netty-bom artifact
				Version:    "1.0.0",
			},
		},
	}

	tests := []struct {
		name           string
		groupID        string
		expectedBomAID string
		found          bool
	}{
		{
			name:           "direct match for io.netty",
			groupID:        "io.netty",
			expectedBomAID: "netty-bom",
			found:          true,
		},
		{
			name:           "direct match for springframework",
			groupID:        "org.springframework",
			expectedBomAID: "spring-framework-bom",
			found:          true,
		},
		{
			name:    "no match for unknown group",
			groupID: "com.unknown",
			found:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bom := findBOMForGroup(result, tt.groupID)
			if tt.found {
				assert.NotNil(t, bom)
				assert.Equal(t, tt.expectedBomAID, bom.ArtifactID)
			} else {
				assert.Nil(t, bom)
			}
		})
	}
}

// TestCalculateOptimalBOMVersion tests version selection logic
func TestCalculateOptimalBOMVersion(t *testing.T) {
	tests := []struct {
		name              string
		requestedVersions map[string]string
		expectedVersion   string
	}{
		{
			name: "single version",
			requestedVersions: map[string]string{
				"netty-handler": "4.1.118.Final",
			},
			expectedVersion: "4.1.118.Final",
		},
		{
			name: "multiple versions - picks highest",
			requestedVersions: map[string]string{
				"netty-handler": "4.1.100.Final",
				"netty-codec":   "4.1.118.Final",
				"netty-common":  "4.1.105.Final",
			},
			expectedVersion: "4.1.118.Final",
		},
		{
			name: "lexicographic ordering",
			requestedVersions: map[string]string{
				"artifact1": "1.0.0",
				"artifact2": "2.0.0",
				"artifact3": "1.5.0",
			},
			expectedVersion: "2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateOptimalBOMVersion(tt.requestedVersions)
			assert.Equal(t, tt.expectedVersion, result)
		})
	}
}
