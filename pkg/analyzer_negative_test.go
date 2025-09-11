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

func TestAnalyzeProjectNegativeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		project       *gopom.Project
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil project",
			project:       nil,
			expectError:   true,
			errorContains: "project is nil",
		},
		{
			name: "project with nil dependencies",
			project: &gopom.Project{
				Dependencies:         nil,
				DependencyManagement: nil,
			},
			expectError: false, // Should handle gracefully
		},
		{
			name: "project with empty dependencies",
			project: &gopom.Project{
				Dependencies: &[]gopom.Dependency{},
			},
			expectError: false,
		},
		{
			name: "project with malformed property reference",
			project: &gopom.Project{
				Dependencies: &[]gopom.Dependency{
					{
						GroupID:    "test",
						ArtifactID: "test",
						Version:    "${incomplete", // Missing closing brace
					},
				},
			},
			expectError: false, // Should handle as literal version
		},
		{
			name: "project with circular property reference",
			project: &gopom.Project{
				Properties: &gopom.Properties{
					Entries: map[string]string{
						"prop1": "${prop2}",
						"prop2": "${prop1}",
					},
				},
				Dependencies: &[]gopom.Dependency{
					{
						GroupID:    "test",
						ArtifactID: "test",
						Version:    "${prop1}",
					},
				},
			},
			expectError: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AnalyzeProject(ctx, tt.project)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestAnalyzeProjectPathNegativeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		pomPath       string
		expectError   bool
		errorContains string
	}{
		{
			name:          "non-existent file",
			pomPath:       "/non/existent/path/pom.xml",
			expectError:   true,
			errorContains: "failed to parse POM file",
		},
		{
			name:          "empty path",
			pomPath:       "",
			expectError:   true,
			errorContains: "failed to parse POM file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AnalyzeProjectPath(ctx, tt.pomPath)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestAnalyzeProjectPathWithInvalidXML(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create invalid XML file
	invalidPOM := filepath.Join(tmpDir, "invalid.pom.xml")
	err := os.WriteFile(invalidPOM, []byte(`
		<project>
			<groupId>test</groupId>
			<!-- Unclosed tag -->
			<artifactId>test
		</project>
	`), 0600)
	require.NoError(t, err)

	result, err := AnalyzeProjectPath(ctx, invalidPOM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse POM file")
	assert.Nil(t, result)
}

func TestPatchStrategyEdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                string
		result              *AnalysisResult
		patches             []Patch
		expectedDirectCount int
		expectedPropCount   int
	}{
		{
			name: "nil analysis result maps",
			result: &AnalysisResult{
				Dependencies:        nil,
				Properties:          nil,
				PropertyUsageCounts: nil,
			},
			patches: []Patch{
				{GroupID: "test", ArtifactID: "test", Version: "1.0"},
			},
			expectedDirectCount: 1,
			expectedPropCount:   0,
		},
		{
			name: "empty patches",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{},
				Properties:   map[string]string{},
			},
			patches:             []Patch{},
			expectedDirectCount: 0,
			expectedPropCount:   0,
		},
		{
			name: "patch for non-existent property",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{
					"test:test": {
						GroupID:      "test",
						ArtifactID:   "test",
						Version:      "${missing.prop}",
						UsesProperty: true,
						PropertyName: "missing.prop",
					},
				},
				Properties: map[string]string{}, // Property not defined
			},
			patches: []Patch{
				{GroupID: "test", ArtifactID: "test", Version: "1.0"},
			},
			expectedDirectCount: 0,
			expectedPropCount:   1, // Should still recommend property update
		},
		{
			name: "conflicting property versions",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{
					"lib1:lib1": {
						GroupID:      "lib1",
						ArtifactID:   "lib1",
						Version:      "${shared.version}",
						UsesProperty: true,
						PropertyName: "shared.version",
					},
					"lib2:lib2": {
						GroupID:      "lib2",
						ArtifactID:   "lib2",
						Version:      "${shared.version}",
						UsesProperty: true,
						PropertyName: "shared.version",
					},
				},
				Properties: map[string]string{
					"shared.version": "1.0.0",
				},
			},
			patches: []Patch{
				{GroupID: "lib1", ArtifactID: "lib1", Version: "2.0.0"},
				{GroupID: "lib2", ArtifactID: "lib2", Version: "3.0.0"}, // Different version!
			},
			expectedDirectCount: 0,
			expectedPropCount:   1, // Should handle conflict
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directPatches, propertyPatches := PatchStrategy(ctx, tt.result, tt.patches)

			assert.Equal(t, tt.expectedDirectCount, len(directPatches),
				"Direct patches count mismatch")
			assert.Equal(t, tt.expectedPropCount, len(propertyPatches),
				"Property patches count mismatch")
		})
	}
}

func TestToAnalysisOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		result *AnalysisResult
	}{
		{
			name: "nil fields in analysis result",
			result: &AnalysisResult{
				Dependencies:           nil,
				Properties:             nil,
				PropertyUsageCounts:    nil,
				BOMs:                   nil,
				TransitiveDependencies: nil,
			},
		},
		{
			name: "empty analysis result",
			result: &AnalysisResult{
				Dependencies:           map[string]*DependencyInfo{},
				Properties:             map[string]string{},
				PropertyUsageCounts:    map[string]int{},
				BOMs:                   []BOMInfo{},
				TransitiveDependencies: []TransitiveDependency{},
			},
		},
		{
			name: "analysis with special characters in values",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{
					"test:test": {
						GroupID:    "test",
						ArtifactID: "test",
						Version:    "1.0.0-SNAPSHOT",
					},
				},
				Properties: map[string]string{
					"special.chars": "value with spaces & symbols!@#$%",
					"unicode":       "测试值",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.result.ToAnalysisOutput("/test/pom.xml", nil, nil)

			assert.NotNil(t, output)
			assert.Equal(t, "/test/pom.xml", output.POMFile)
			assert.NotNil(t, output.Properties.UsedBy) // Should initialize map

			// Should not panic when accessing any fields
			_ = output.Dependencies.Total
			_ = output.Properties.Defined
			_ = output.BOMs
		})
	}
}

func TestGetAffectedDependenciesEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		result       *AnalysisResult
		propertyName string
		expectedDeps int
	}{
		{
			name: "nil dependencies map",
			result: &AnalysisResult{
				Dependencies: nil,
			},
			propertyName: "test.prop",
			expectedDeps: 0,
		},
		{
			name: "empty property name",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{
					"test:test": {
						UsesProperty: true,
						PropertyName: "test.prop",
					},
				},
			},
			propertyName: "",
			expectedDeps: 0,
		},
		{
			name: "property with no dependencies",
			result: &AnalysisResult{
				Dependencies: map[string]*DependencyInfo{
					"test:test": {
						UsesProperty: false,
					},
				},
			},
			propertyName: "unused.prop",
			expectedDeps: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			affected := tt.result.GetAffectedDependencies(tt.propertyName)
			assert.Equal(t, tt.expectedDeps, len(affected))
		})
	}
}

func TestAnalyzeDependencyWithCornerCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		dep            gopom.Dependency
		expectedProp   string
		expectProperty bool
	}{
		{
			name: "property with spaces",
			dep: gopom.Dependency{
				GroupID:    "test",
				ArtifactID: "test",
				Version:    "${ prop.with.spaces }",
			},
			expectedProp:   " prop.with.spaces ", // Should preserve spaces
			expectProperty: true,
		},
		{
			name: "nested property syntax",
			dep: gopom.Dependency{
				GroupID:    "test",
				ArtifactID: "test",
				Version:    "${${nested}}",
			},
			expectedProp:   "${nested}",
			expectProperty: true,
		},
		{
			name: "partial property reference",
			dep: gopom.Dependency{
				GroupID:    "test",
				ArtifactID: "test",
				Version:    "1.0-${suffix}",
			},
			expectedProp:   "",
			expectProperty: false, // Not a pure property reference
		},
		{
			name: "empty version",
			dep: gopom.Dependency{
				GroupID:    "test",
				ArtifactID: "test",
				Version:    "",
			},
			expectedProp:   "",
			expectProperty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &AnalysisResult{
				Dependencies:        make(map[string]*DependencyInfo),
				PropertyUsageCounts: make(map[string]int),
				Properties:          make(map[string]string),
			}

			analyzeDependency(ctx, tt.dep, result)

			depKey := "test:test"
			dep, exists := result.Dependencies[depKey]
			require.True(t, exists)

			assert.Equal(t, tt.expectProperty, dep.UsesProperty)
			if tt.expectProperty {
				assert.Equal(t, tt.expectedProp, dep.PropertyName)
			}
		})
	}
}
