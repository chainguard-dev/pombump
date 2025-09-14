package pkg

import (
	"context"
	"os"
	"path/filepath"
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

func TestBOMDetection(t *testing.T) {
	tests := []struct {
		name         string
		project      *gopom.Project
		expectedBOMs int
		bomArtifacts []string
	}{
		{
			name: "project with multiple BOMs",
			project: &gopom.Project{
				DependencyManagement: &gopom.DependencyManagement{
					Dependencies: &[]gopom.Dependency{
						{
							GroupID:    "io.projectreactor",
							ArtifactID: "reactor-bom",
							Version:    "2023.0.10",
							Type:       "pom",
							Scope:      "import",
						},
						{
							GroupID:    "io.netty",
							ArtifactID: "netty-bom",
							Version:    "4.1.115.Final",
							Type:       "pom",
							Scope:      "import",
						},
						{
							GroupID:    "org.apache.lucene",
							ArtifactID: "lucene-core",
							Version:    "9.11.1",
						},
					},
				},
			},
			expectedBOMs: 2,
			bomArtifacts: []string{"reactor-bom", "netty-bom"},
		},
		{
			name: "project with no BOMs",
			project: &gopom.Project{
				DependencyManagement: &gopom.DependencyManagement{
					Dependencies: &[]gopom.Dependency{
						{
							GroupID:    "org.apache.lucene",
							ArtifactID: "lucene-core",
							Version:    "9.11.1",
						},
					},
				},
			},
			expectedBOMs: 0,
			bomArtifacts: []string{},
		},
		{
			name: "project with BOM using property",
			project: &gopom.Project{
				Properties: &gopom.Properties{
					Entries: map[string]string{
						"awssdk.version": "2.21.29",
					},
				},
				DependencyManagement: &gopom.DependencyManagement{
					Dependencies: &[]gopom.Dependency{
						{
							GroupID:    "software.amazon.awssdk",
							ArtifactID: "bom",
							Version:    "${awssdk.version}",
							Type:       "pom",
							Scope:      "import",
						},
						{
							GroupID:    "junit",
							ArtifactID: "junit",
							Version:    "4.13.2",
						},
					},
				},
			},
			expectedBOMs: 1,
			bomArtifacts: []string{"bom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := AnalyzeProject(ctx, tt.project)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedBOMs, len(result.BOMs), "unexpected number of BOMs detected")

			actualArtifacts := make([]string, 0, len(result.BOMs))
			for _, bom := range result.BOMs {
				actualArtifacts = append(actualArtifacts, bom.ArtifactID)
			}

			for _, expectedArtifact := range tt.bomArtifacts {
				assert.Contains(t, actualArtifacts, expectedArtifact, "expected BOM artifact not found: %s", expectedArtifact)
			}
		})
	}
}

func TestComplexProjectAnalysis(t *testing.T) {
	ctx := context.Background()

	project := &gopom.Project{
		Properties: &gopom.Properties{
			Entries: map[string]string{
				"awssdk.version":  "2.21.29",
				"jackson.version": "2.18.0",
				"junit.version":   "5.11.2",
				"lucene.version":  "9.11.1",
			},
		},
		DependencyManagement: &gopom.DependencyManagement{
			Dependencies: &[]gopom.Dependency{
				{
					GroupID:    "io.netty",
					ArtifactID: "netty-bom",
					Version:    "4.1.115.Final",
					Type:       "pom",
					Scope:      "import",
				},
				{
					GroupID:    "software.amazon.awssdk",
					ArtifactID: "bom",
					Version:    "${awssdk.version}",
					Type:       "pom",
					Scope:      "import",
				},
				{
					GroupID:    "com.fasterxml.jackson.core",
					ArtifactID: "jackson-databind",
					Version:    "${jackson.version}",
				},
				{
					GroupID:    "org.apache.lucene",
					ArtifactID: "lucene-core",
					Version:    "${lucene.version}",
				},
			},
		},
		Dependencies: &[]gopom.Dependency{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
			},
			{
				GroupID:    "software.amazon.awssdk",
				ArtifactID: "s3",
			},
			{
				GroupID:    "com.fasterxml.jackson.core",
				ArtifactID: "jackson-databind",
			},
			{
				GroupID:    "org.apache.lucene",
				ArtifactID: "lucene-core",
			},
		},
	}

	result, err := AnalyzeProject(ctx, project)
	require.NoError(t, err)

	assert.Equal(t, 2, len(result.BOMs), "should detect 2 BOMs")

	bomArtifacts := make([]string, len(result.BOMs))
	for i, bom := range result.BOMs {
		bomArtifacts[i] = bom.ArtifactID
	}
	assert.Contains(t, bomArtifacts, "netty-bom")
	assert.Contains(t, bomArtifacts, "bom")

	assert.Equal(t, 4, len(result.Properties), "should detect 4 properties")
	assert.Contains(t, result.Properties, "awssdk.version")
	assert.Contains(t, result.Properties, "jackson.version")
	assert.Contains(t, result.Properties, "junit.version")
	assert.Contains(t, result.Properties, "lucene.version")

	assert.Greater(t, len(result.Dependencies), 0, "should detect dependencies")
}

func TestBOMRecommendationStrategy(t *testing.T) {
	ctx := context.Background()

	result := &AnalysisResult{
		Dependencies: map[string]*DependencyInfo{
			"io.netty:netty-handler": {
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.100.Final",
			},
			"io.netty:netty-codec": {
				GroupID:    "io.netty",
				ArtifactID: "netty-codec",
				Version:    "4.1.100.Final",
			},
			"io.netty:netty-transport": {
				GroupID:    "io.netty",
				ArtifactID: "netty-transport",
				Version:    "4.1.100.Final",
			},
		},
		BOMs: []BOMInfo{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.100.Final",
			},
		},
	}

	patches := []Patch{
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-handler",
			Version:    "4.1.115.Final",
		},
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-codec",
			Version:    "4.1.115.Final",
		},
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-transport",
			Version:    "4.1.115.Final",
		},
	}

	directPatches, propertyPatches := PatchStrategy(ctx, result, patches)

	totalPatches := len(directPatches) + len(propertyPatches)
	assert.Greater(t, totalPatches, 0, "should have some patching strategy")
}

// TestAnalysisWithRealConfigurations removed - redundant with existing analysis tests

func TestPropertyAnalysisWithRealData(t *testing.T) {
	ctx := context.Background()
	
	// Load real properties
	properties, err := ParseProperties(ctx, "testdata/pombump-properties.yaml", "")
	require.NoError(t, err)

	// Verify properties structure
	for key, value := range properties {
		assert.NotEmpty(t, key)
		assert.NotEmpty(t, value)
		assert.Contains(t, key, ".")
		assert.NotContains(t, value, " ")
	}

	// Test specific properties we expect in real configuration
	expectedProperties := []string{"netty.version", "commons.beanutils.version"}
	for _, expected := range expectedProperties {
		value, exists := properties[expected]
		if exists {
			assert.NotEmpty(t, value)
			t.Logf("Found real property: %s = %s", expected, value)
		}
	}
}

func TestAnalyzeBOMs(t *testing.T) {
	project := &gopom.Project{
		DependencyManagement: &gopom.DependencyManagement{
			Dependencies: &[]gopom.Dependency{
				{
					GroupID:    "io.netty",
					ArtifactID: "netty-bom",
					Version:    "4.1.115.Final",
					Type:       "pom",
					Scope:      "import",
				},
				{
					GroupID:    "software.amazon.awssdk",
					ArtifactID: "bom",
					Version:    "2.21.29",
					Type:       "pom",
					Scope:      "import",
				},
				{
					GroupID:    "junit",
					ArtifactID: "junit",
					Version:    "4.13.2",
				},
			},
		},
	}

	ctx := context.Background()
	boms := AnalyzeBOMs(ctx, project)

	assert.Len(t, boms, 2, "should find 2 BOMs")
	assert.Equal(t, "io.netty", boms[0].GroupID)
	assert.Equal(t, "netty-bom", boms[0].ArtifactID)
	assert.Equal(t, "software.amazon.awssdk", boms[1].GroupID)
	assert.Equal(t, "bom", boms[1].ArtifactID)
}

func TestIsBOM(t *testing.T) {
	tests := []struct {
		name     string
		bom      BOMInfo
		expected bool
	}{
		{
			name: "valid BOM",
			bom: BOMInfo{
				Type:  "pom",
				Scope: "import",
			},
			expected: true,
		},
		{
			name: "not BOM - wrong type",
			bom: BOMInfo{
				Type:  "jar",
				Scope: "import",
			},
			expected: false,
		},
		{
			name: "not BOM - wrong scope",
			bom: BOMInfo{
				Type:  "pom",
				Scope: "compile",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bom.IsBOM()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToAnalysisOutput(t *testing.T) {
	result := &AnalysisResult{
		Dependencies: map[string]*DependencyInfo{
			"io.netty:netty-handler": {
				GroupID:      "io.netty",
				ArtifactID:   "netty-handler",
				Version:      "4.1.100.Final",
				UsesProperty: false,
			},
		},
		Properties: map[string]string{
			"netty.version": "4.1.100.Final",
		},
		BOMs: []BOMInfo{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.100.Final",
			},
		},
	}

	patches := []Patch{
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-handler",
			Version:    "4.1.115.Final",
		},
	}

	propertyUpdates := map[string]string{
		"netty.version": "4.1.115.Final",
	}

	output := result.ToAnalysisOutput("test.xml", patches, propertyUpdates)

	assert.Equal(t, "test.xml", output.POMFile)
	assert.Equal(t, 1, output.Dependencies.Total)
	assert.Len(t, output.Properties.Defined, 1)
	assert.Len(t, output.BOMs, 1)
	assert.Len(t, output.Patches, 1)
	assert.Len(t, output.PropertyUpdates, 1)
}

func TestDetectVersionConflicts(t *testing.T) {
	patches := []Patch{
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-handler",
			Version:    "4.1.115.Final",
		},
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-codec",
			Version:    "4.1.110.Final", // Different version - conflict!
		},
		{
			GroupID:    "junit",
			ArtifactID: "junit",
			Version:    "4.13.2",
		},
	}

	boms := []BOMInfo{
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-bom",
			Version:    "4.1.115.Final",
		},
	}

	result := &AnalysisResult{
		BOMs: boms,
	}

	ctx := context.Background()
	conflicts := detectVersionConflicts(ctx, result, patches)

	assert.Greater(t, len(conflicts), 0, "should detect version conflicts")
}

func TestAnalysisReport(t *testing.T) {
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
				GroupID:    "junit",
				ArtifactID: "junit",
				Version:    "4.13.2",
			},
		},
		Properties: map[string]string{
			"netty.version": "4.1.115.Final",
			"unused.prop":  "1.0.0",
		},
		PropertyUsageCounts: map[string]int{
			"netty.version": 1,
		},
	}

	report := result.AnalysisReport()

	assert.Contains(t, report, "POM Analysis Report")
	assert.Contains(t, report, "Total dependencies: 2")
	assert.Contains(t, report, "Dependencies using properties: 1")
	assert.Contains(t, report, "Total properties defined: 2")
	assert.Contains(t, report, "netty.version = 4.1.115.Final (used by 1 dependencies)")
	assert.Contains(t, report, "io.netty:netty-handler -> ${netty.version}")
	assert.Contains(t, report, "Property Usage:")
	assert.Contains(t, report, "Dependencies Using Properties:")
}

func TestCalculateOptimalBOMVersion(t *testing.T) {
	tests := []struct {
		name      string
		versions  map[string]string
		expected  string
	}{
		{
			name:     "single version",
			versions: map[string]string{"artifact1": "1.0.0"},
			expected: "1.0.0",
		},
		{
			name: "multiple versions - lexicographic highest",
			versions: map[string]string{
				"artifact1": "1.0.0",
				"artifact2": "2.0.0",
				"artifact3": "1.5.0",
			},
			expected: "2.0.0",
		},
		{
			name:     "empty versions",
			versions: map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateOptimalBOMVersion(tt.versions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindBOMForGroup(t *testing.T) {
	result := &AnalysisResult{
		BOMs: []BOMInfo{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.115.Final",
			},
			{
				GroupID:    "org.springframework",
				ArtifactID: "spring-bom",
				Version:    "5.3.0",
			},
			{
				GroupID:    "com.example",
				ArtifactID: "netty-bom", // Different group but netty-bom name
				Version:    "1.0.0",
			},
		},
	}

	tests := []struct {
		name          string
		groupID       string
		expectedFound bool
		expectedBOM   *BOMInfo
	}{
		{
			name:          "direct match",
			groupID:       "io.netty",
			expectedFound: true,
			expectedBOM: &BOMInfo{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.115.Final",
			},
		},
		{
			name:          "spring match",
			groupID:       "org.springframework",
			expectedFound: true,
			expectedBOM: &BOMInfo{
				GroupID:    "org.springframework",
				ArtifactID: "spring-bom",
				Version:    "5.3.0",
			},
		},
		{
			name:          "no match",
			groupID:       "com.unknown",
			expectedFound: false,
			expectedBOM:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bom := findBOMForGroup(result, tt.groupID)
			if tt.expectedFound {
				assert.NotNil(t, bom)
				assert.Equal(t, tt.expectedBOM.GroupID, bom.GroupID)
				assert.Equal(t, tt.expectedBOM.ArtifactID, bom.ArtifactID)
				assert.Equal(t, tt.expectedBOM.Version, bom.Version)
			} else {
				assert.Nil(t, bom)
			}
		})
	}
}

func TestAnalyzeProjectEdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		project     *gopom.Project
		expectError bool
	}{
		{
			name:        "nil project",
			project:     nil,
			expectError: true,
		},
		{
			name:        "empty project",
			project:     &gopom.Project{},
			expectError: false,
		},
		{
			name: "project with nil dependencies",
			project: &gopom.Project{
				Dependencies: nil,
			},
			expectError: false,
		},
		{
			name: "project with nil properties",
			project: &gopom.Project{
				Properties: nil,
			},
			expectError: false,
		},
		{
			name: "project with nil dependency management",
			project: &gopom.Project{
				DependencyManagement: nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AnalyzeProject(ctx, tt.project)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestPatchStrategyEdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                  string
		result                *AnalysisResult
		patches               []Patch
		expectedDirectPatches int
		expectedPropertyCount int
	}{
		{
			name: "empty patches",
			result: &AnalysisResult{
				Dependencies: make(map[string]*DependencyInfo),
				Properties:   make(map[string]string),
				BOMs:         []BOMInfo{},
			},
			patches:               []Patch{},
			expectedDirectPatches: 0,
			expectedPropertyCount: 0,
		},
		{
			name: "patch with missing property",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{
					"io.netty:netty-handler": {
						GroupID:      "io.netty",
						ArtifactID:   "netty-handler",
						UsesProperty: true,
						PropertyName: "netty.version",
					},
				},
				Properties: map[string]string{}, // Property not defined
				BOMs:       []BOMInfo{},
			},
			patches: []Patch{
				{
					GroupID:    "io.netty",
					ArtifactID: "netty-handler",
					Version:    "4.1.115.Final",
				},
			},
			expectedDirectPatches: 0,
			expectedPropertyCount: 1,
		},
		{
			name: "duplicate property patches",
			result: &AnalysisResult{
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
				},
				Properties: map[string]string{
					"netty.version": "4.1.100.Final",
				},
				BOMs: []BOMInfo{},
			},
			patches: []Patch{
				{
					GroupID:    "io.netty",
					ArtifactID: "netty-handler",
					Version:    "4.1.115.Final",
				},
				{
					GroupID:    "io.netty",
					ArtifactID: "netty-codec",
					Version:    "4.1.110.Final", // Different version should cause warning
				},
			},
			expectedDirectPatches: 0,
			expectedPropertyCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directPatches, propertyPatches := PatchStrategy(ctx, tt.result, tt.patches)
			assert.Equal(t, tt.expectedDirectPatches, len(directPatches))
			assert.Equal(t, tt.expectedPropertyCount, len(propertyPatches))
		})
	}
}

func TestToAnalysisOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		result *AnalysisResult
	}{
		{
			name: "empty result",
			result: &AnalysisResult{
				Dependencies: make(map[string]*DependencyInfo),
				Properties:   make(map[string]string),
				BOMs:         []BOMInfo{},
			},
		},
		{
			name: "result with transitive dependencies",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{
					"io.netty:netty-handler": {
						GroupID:    "io.netty",
						ArtifactID: "netty-handler",
						Version:    "4.1.115.Final",
					},
				},
				Properties: make(map[string]string),
				BOMs:       []BOMInfo{},
				TransitiveDependencies: []TransitiveDependency{
					{GroupID: "junit", ArtifactID: "junit", Version: "4.13.2"},
				},
			},
		},
		{
			name: "result with properties but no usage",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{
					"junit:junit": {
						GroupID:      "junit",
						ArtifactID:   "junit",
						Version:      "4.13.2",
						UsesProperty: false,
					},
				},
				Properties: map[string]string{
					"unused.version": "1.0.0",
				},
				BOMs: []BOMInfo{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.result.ToAnalysisOutput("test.xml", []Patch{}, map[string]string{})
			assert.NotNil(t, output)
			assert.Equal(t, "test.xml", output.POMFile)
			assert.Equal(t, len(tt.result.TransitiveDependencies), output.Dependencies.Transitive)
		})
	}
}

func TestAnalyzeProjectPathEdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		pomPath     string
		expectError bool
	}{
		{
			name:        "nonexistent file",
			pomPath:     "/nonexistent/path/pom.xml",
			expectError: true,
		},
		{
			name:        "invalid POM file - not XML",
			pomPath:     "testdata/patches.yaml", // YAML file, not XML
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AnalyzeProjectPath(ctx, tt.pomPath)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestPatchStrategyBOMConflictsPaths(t *testing.T) {
	ctx := context.Background()

	// Test case to trigger BOM conflict detection and recommendation paths
	result := &AnalysisResult{
		Dependencies: map[string]*DependencyInfo{
			"io.netty:netty-handler": {
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.100.Final",
			},
			"io.netty:netty-codec": {
				GroupID:    "io.netty", 
				ArtifactID: "netty-codec",
				Version:    "4.1.100.Final",
			},
		},
		Properties: make(map[string]string),
		BOMs: []BOMInfo{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.100.Final",
			},
		},
	}

	// Patches with version conflicts for the same group
	patches := []Patch{
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-handler",
			Version:    "4.1.115.Final",
		},
		{
			GroupID:    "io.netty",
			ArtifactID: "netty-codec",
			Version:    "4.1.110.Final", // Different version - should trigger conflict resolution
		},
	}

	directPatches, propertyPatches := PatchStrategy(ctx, result, patches)

	// Should have some patches - exact behavior depends on conflict resolution
	totalPatches := len(directPatches) + len(propertyPatches)
	assert.Greater(t, totalPatches, 0, "should have some patching strategy")
}

func TestSearchForPropertiesErrorPaths(t *testing.T) {
	ctx := context.Background()

	// Test with a directory that contains files that cause parsing errors
	tempDir := t.TempDir()
	
	// Create a file that looks like XML but isn't a valid POM
	invalidXMLPath := filepath.Join(tempDir, "invalid.xml")
	err := os.WriteFile(invalidXMLPath, []byte("<invalid>not a pom</invalid>"), 0644)
	require.NoError(t, err)

	properties := searchForProperties(ctx, tempDir, "")
	// Should handle the error gracefully and continue
	assert.NotNil(t, properties)
}

func TestFindBOMForGroupEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		result   *AnalysisResult
		groupID  string
		expected *BOMInfo
	}{
		{
			name: "spring framework BOM pattern",
			result: &AnalysisResult{
				BOMs: []BOMInfo{
					{
						GroupID:    "org.springframework",
						ArtifactID: "spring-framework-bom",
						Version:    "5.3.0",
					},
				},
			},
			groupID: "org.springframework",
			expected: &BOMInfo{
				GroupID:    "org.springframework",
				ArtifactID: "spring-framework-bom",
				Version:    "5.3.0",
			},
		},
		{
			name: "netty BOM with different group but matching pattern",
			result: &AnalysisResult{
				BOMs: []BOMInfo{
					{
						GroupID:    "com.example",
						ArtifactID: "netty-bom",
						Version:    "1.0.0",
					},
				},
			},
			groupID:  "io.netty",
			expected: &BOMInfo{
				GroupID:    "com.example",
				ArtifactID: "netty-bom",
				Version:    "1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bom := findBOMForGroup(tt.result, tt.groupID)
			if tt.expected != nil {
				assert.NotNil(t, bom)
				assert.Equal(t, tt.expected.GroupID, bom.GroupID)
				assert.Equal(t, tt.expected.ArtifactID, bom.ArtifactID)
				assert.Equal(t, tt.expected.Version, bom.Version)
			} else {
				assert.Nil(t, bom)
			}
		})
	}
}

func TestToAnalysisOutputMoreEdgeCases(t *testing.T) {
	// Test with complex property usage mapping
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
		},
		Properties: map[string]string{
			"netty.version": "4.1.115.Final",
		},
		BOMs: []BOMInfo{},
	}

	output := result.ToAnalysisOutput("test.xml", []Patch{}, map[string]string{})
	assert.Equal(t, 2, len(output.Properties.UsedBy["netty.version"]))
	assert.Equal(t, 0, output.Dependencies.Direct) // Both use properties
}

func TestSearchForPropertiesProjectRootError(t *testing.T) {
	ctx := context.Background()

	// Test error handling in filepath operations
	tempDir := t.TempDir()
	invalidPath := filepath.Join(tempDir, "nonexistent")
	
	properties := searchForProperties(ctx, invalidPath, "")
	assert.NotNil(t, properties)
	assert.Equal(t, 0, len(properties)) // Should handle error gracefully
}

func TestAnalysisReportEdgeCases(t *testing.T) {
	// Test with properties that are not defined (undefined property case)
	result := &AnalysisResult{
		Dependencies: map[string]*DependencyInfo{
			"test:lib": {
				GroupID:      "test",
				ArtifactID:   "lib",
				Version:      "${undefined.version}",
				UsesProperty: true,
				PropertyName: "undefined.version",
			},
		},
		Properties: map[string]string{
			"defined.version": "1.0.0",
		},
		PropertyUsageCounts: map[string]int{
			"undefined.version": 1,
		},
	}

	report := result.AnalysisReport()
	assert.Contains(t, report, "undefined.version (used by 1 dependencies) - NOT DEFINED")
	assert.Contains(t, report, "defined.version = 1.0.0 (used by 0 dependencies)")
}
