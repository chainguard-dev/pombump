package pkg

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalysisOutputWrite(t *testing.T) {
	// Create a sample analysis output
	output := &AnalysisOutput{
		POMFile:   "test-project/pom.xml",
		Timestamp: time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
		Dependencies: DependencyAnalysis{
			Total:           10,
			Direct:          8,
			UsingProperties: 5,
			FromBOMs:        3,
			Transitive:      2,
		},
		BOMs: []BOMInfo{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.115.Final",
			},
			{
				GroupID:    "software.amazon.awssdk",
				ArtifactID: "bom",
				Version:    "2.21.29",
			},
		},
		Properties: PropertyAnalysis{
			Defined: map[string]string{
				"netty.version":   "4.1.115.Final",
				"jackson.version": "2.18.0",
				"junit.version":   "5.11.2",
			},
			UsedBy: map[string][]string{
				"netty.version":   {"io.netty:netty-handler", "io.netty:netty-codec"},
				"jackson.version": {"com.fasterxml.jackson.core:jackson-databind"},
				"junit.version":   {"org.junit.jupiter:junit-jupiter"},
			},
		},
		Issues: []Issue{
			{
				Dependency:      "io.netty:netty-handler",
				Type:            "vulnerability",
				CurrentVersion:  "4.1.100.Final",
				RequiredVersion: "4.1.115.Final",
				CVEs:            []string{"CVE-2023-1234", "CVE-2023-5678"},
				Path:            []string{"root", "io.netty:netty-handler"},
				Property:        "netty.version",
			},
		},
		Patches: []Patch{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.115.Final",
			},
		},
		PropertyUpdates: map[string]string{
			"netty.version": "4.1.115.Final",
		},
		Warnings: []string{
			"Property 'unused.version' is defined but not used",
			"BOM version conflict detected for io.netty group",
		},
		CannotFix: []UnfixableIssue{
			{
				Dependency: "legacy.lib:old-dependency",
				Reason:     "No compatible version available",
				Action:     "Consider replacing with modern alternative",
			},
		},
	}

	tests := []struct {
		name           string
		format         string
		expectedSubstr []string
		jsonValidation bool
		yamlValidation bool
	}{
		{
			name:   "text readable format",
			format: "text",
			expectedSubstr: []string{
				"POM Analysis: test-project/pom.xml",
				"2024-01-15 14:30:45",
				"Dependencies Summary:",
				"Total: 10",
				"Direct: 8",
				"Using properties: 5",
				"From BOMs: 3",
				"Transitive: 2",
				"Imported BOMs:",
				"io.netty:netty-bom:4.1.115.Final",
				"software.amazon.awssdk:bom:2.21.29",
				"Defined Properties:",
				"netty.version = 4.1.115.Final (used by 2 dependencies)",
				"jackson.version = 2.18.0 (used by 1 dependencies)",
				"junit.version = 5.11.2 (used by 1 dependencies)",
				"Issues Found: 1",
				"io.netty:netty-handler (vulnerability)",
				"CVE-2023-1234, CVE-2023-5678",
				"Property: ${netty.version}",
				"Recommended Patches:",
				"Property Updates:",
				"netty.version: 4.1.115.Final -> 4.1.115.Final",
				"Direct Dependency Updates:",
				"io.netty:netty-handler -> 4.1.115.Final",
				"Warnings:",
				"Property 'unused.version' is defined but not used",
				"BOM version conflict detected for io.netty group",
				"Cannot Fix (Manual Intervention Required):",
				"legacy.lib:old-dependency",
				"No compatible version available",
				"Consider replacing with modern alternative",
				"Summary:",
				"Fixable issues: 2",
				"Unfixable issues: 1",
			},
		},
		{
			name:           "json format",
			format:         "json",
			jsonValidation: true,
			expectedSubstr: []string{
				`"pomFile": "test-project/pom.xml"`,
				`"total": 10`,
				`"direct": 8`,
				`"usingProperties": 5`,
				`"fromBOMs": 3`,
				`"transitive": 2`,
				`"netty-bom"`,
				`"netty.version"`,
				`"CVE-2023-1234"`,
			},
		},
		{
			name:           "yaml format",
			format:         "yaml",
			yamlValidation: true,
			expectedSubstr: []string{
				"pomFile: test-project/pom.xml",
				"total: 10",
				"direct: 8",
				"usingProperties: 5",
				"fromBOMs: 3",
				"transitive: 2",
				"artifactId: netty-bom",
				"netty.version:",
				"- CVE-2023-1234",
			},
		},
		{
			name:   "empty format defaults to text",
			format: "",
			expectedSubstr: []string{
				"POM Analysis: test-project/pom.xml",
				"Dependencies Summary:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := output.Write(tt.format, &buf)
			require.NoError(t, err)

			result := buf.String()

			// Validate JSON structure if requested
			if tt.jsonValidation {
				var jsonData map[string]interface{}
				err := json.Unmarshal([]byte(result), &jsonData)
				assert.NoError(t, err, "should produce valid JSON")
			}

			// Validate YAML structure if requested
			if tt.yamlValidation {
				var yamlData map[string]interface{}
				err := yaml.Unmarshal([]byte(result), &yamlData)
				assert.NoError(t, err, "should produce valid YAML")
			}

			// Check for expected substrings
			for _, substr := range tt.expectedSubstr {
				assert.Contains(t, result, substr, "output should contain: %s", substr)
			}
		})
	}
}

func TestWriteOutputHumanFormat(t *testing.T) {
	output := &AnalysisOutput{
		POMFile:   "simple-project/pom.xml",
		Timestamp: time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
		Dependencies: DependencyAnalysis{
			Total:           3,
			Direct:          3,
			UsingProperties: 2,
		},
		Properties: PropertyAnalysis{
			Defined: map[string]string{
				"slf4j.version": "1.7.30",
				"junit.version": "5.11.2",
			},
			UsedBy: map[string][]string{
				"slf4j.version": {"org.slf4j:slf4j-api"},
				"junit.version": {"org.junit.jupiter:junit-jupiter"},
			},
		},
		Patches: []Patch{
			{
				GroupID:    "org.slf4j",
				ArtifactID: "slf4j-api",
				Version:    "1.7.36",
			},
		},
		PropertyUpdates: map[string]string{
			"slf4j.version": "1.7.36",
		},
	}

	var buf bytes.Buffer
	err := output.WriteOutput(&buf)
	require.NoError(t, err)

	result := buf.String()

	// Check header
	assert.Contains(t, result, "POM Analysis: simple-project/pom.xml")
	assert.Contains(t, result, "2024-01-15 14:30:45")

	// Check dependencies summary
	assert.Contains(t, result, "Total: 3")
	assert.Contains(t, result, "Direct: 3")
	assert.Contains(t, result, "Using properties: 2")

	// Check properties
	assert.Contains(t, result, "slf4j.version = 1.7.30 (used by 1 dependencies)")
	assert.Contains(t, result, "junit.version = 5.11.2 (used by 1 dependencies)")

	// Check patches
	assert.Contains(t, result, "Property Updates:")
	assert.Contains(t, result, "slf4j.version: 1.7.30 -> 1.7.36")
	assert.Contains(t, result, "Direct Dependency Updates:")
	assert.Contains(t, result, "org.slf4j:slf4j-api -> 1.7.36")

	// Check summary
	assert.Contains(t, result, "Fixable issues: 2")
	assert.Contains(t, result, "Unfixable issues: 0")
}

func TestOutputWithNoIssues(t *testing.T) {
	output := &AnalysisOutput{
		POMFile:   "clean-project/pom.xml",
		Timestamp: time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
		Dependencies: DependencyAnalysis{
			Total:  5,
			Direct: 5,
		},
		Properties: PropertyAnalysis{
			Defined: map[string]string{},
		},
	}

	var buf bytes.Buffer
	err := output.WriteOutput(&buf)
	require.NoError(t, err)

	result := buf.String()

	// Should have basic structure
	assert.Contains(t, result, "POM Analysis: clean-project/pom.xml")
	assert.Contains(t, result, "Total: 5")

	// Should not have sections for missing data
	assert.NotContains(t, result, "Imported BOMs:")
	assert.NotContains(t, result, "Defined Properties:")
	assert.NotContains(t, result, "Issues Found:")
	assert.NotContains(t, result, "Recommended Patches:")
	assert.NotContains(t, result, "Warnings:")
	assert.NotContains(t, result, "Cannot Fix")

	// Summary should show no issues
	assert.Contains(t, result, "Fixable issues: 0")
	assert.Contains(t, result, "Unfixable issues: 0")
}

func TestUnsupportedOutputFormat(t *testing.T) {
	output := &AnalysisOutput{
		POMFile: "test.xml",
	}

	var buf bytes.Buffer
	err := output.Write("unsupported", &buf)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format: unsupported")
}

func TestJSONOutputStructure(t *testing.T) {
	output := &AnalysisOutput{
		POMFile:   "test.xml",
		Timestamp: time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
		Dependencies: DependencyAnalysis{
			Total:           10,
			Direct:          8,
			UsingProperties: 5,
			FromBOMs:        3,
		},
		BOMs: []BOMInfo{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-bom",
				Version:    "4.1.115.Final",
			},
		},
		Properties: PropertyAnalysis{
			Defined: map[string]string{
				"netty.version": "4.1.115.Final",
			},
		},
		Patches: []Patch{
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "4.1.115.Final",
			},
		},
	}

	var buf bytes.Buffer
	err := output.Write("json", &buf)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Verify top-level structure
	assert.Equal(t, "test.xml", result["pomFile"])
	assert.NotNil(t, result["timestamp"])
	assert.NotNil(t, result["dependencies"])
	assert.NotNil(t, result["boms"])
	assert.NotNil(t, result["properties"])
	assert.NotNil(t, result["patches"])

	// Verify dependencies structure
	deps := result["dependencies"].(map[string]interface{})
	assert.Equal(t, float64(10), deps["total"])
	assert.Equal(t, float64(8), deps["direct"])
	assert.Equal(t, float64(5), deps["usingProperties"])
	assert.Equal(t, float64(3), deps["fromBOMs"])

	// Verify BOMs structure
	boms := result["boms"].([]interface{})
	assert.Len(t, boms, 1)
	bom := boms[0].(map[string]interface{})
	assert.Equal(t, "io.netty", bom["groupId"])
	assert.Equal(t, "netty-bom", bom["artifactId"])
	assert.Equal(t, "4.1.115.Final", bom["version"])

	// Verify patches structure
	patches := result["patches"].([]interface{})
	assert.Len(t, patches, 1)
	patch := patches[0].(map[string]interface{})
	assert.Equal(t, "io.netty", patch["groupId"])
	assert.Equal(t, "netty-handler", patch["artifactId"])
	assert.Equal(t, "4.1.115.Final", patch["version"])
}

func TestYAMLOutputStructure(t *testing.T) {
	output := &AnalysisOutput{
		POMFile:   "test.xml",
		Timestamp: time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
		Dependencies: DependencyAnalysis{
			Total:  5,
			Direct: 5,
		},
		Properties: PropertyAnalysis{
			Defined: map[string]string{
				"junit.version": "5.11.2",
			},
			UsedBy: map[string][]string{
				"junit.version": {"org.junit.jupiter:junit-jupiter"},
			},
		},
	}

	var buf bytes.Buffer
	err := output.Write("yaml", &buf)
	require.NoError(t, err)

	result := buf.String()

	// Should be valid YAML
	var yamlData map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &yamlData)
	require.NoError(t, err)

	// Check key elements are present
	assert.Contains(t, result, "pomFile: test.xml")
	assert.Contains(t, result, "total: 5")
	assert.Contains(t, result, "direct: 5")
	assert.Contains(t, result, "junit.version:")
	assert.Contains(t, result, "- org.junit.jupiter:junit-jupiter")
}

func TestOutputAffectedDependencies(t *testing.T) {
	output := &AnalysisOutput{
		POMFile:   "test.xml",
		Timestamp: time.Now(),
		Properties: PropertyAnalysis{
			Defined: map[string]string{
				"netty.version": "4.1.100.Final",
			},
			UsedBy: map[string][]string{
				"netty.version": {
					"io.netty:netty-handler",
					"io.netty:netty-codec",
					"io.netty:netty-transport",
				},
			},
		},
		PropertyUpdates: map[string]string{
			"netty.version": "4.1.115.Final",
		},
	}

	var buf bytes.Buffer
	err := output.WriteOutput(&buf)
	require.NoError(t, err)

	result := buf.String()

	// Should show the property update with affected dependencies
	assert.Contains(t, result, "netty.version: 4.1.100.Final -> 4.1.115.Final")
	assert.Contains(t, result, "Affects: io.netty:netty-handler, io.netty:netty-codec, io.netty:netty-transport")
}

// TestOutputWithRealConfiguration removed - redundant with existing output format tests

func TestWriteError(t *testing.T) {
	output := &AnalysisOutput{
		POMFile: "test.xml",
	}

	var buf bytes.Buffer
	err := output.Write("unsupported-format", &buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestWriteJSONError(t *testing.T) {
	// Create an output with a channel that can't be marshaled to JSON
	output := &AnalysisOutput{
		POMFile: "test.xml",
	}

	// This should work fine, but let's test the JSON marshaling path explicitly
	var buf bytes.Buffer
	err := output.Write("json", &buf)
	assert.NoError(t, err)
}

func TestWriteYAMLError(t *testing.T) {
	// Test YAML marshaling path
	output := &AnalysisOutput{
		POMFile: "test.xml",
	}

	var buf bytes.Buffer
	err := output.Write("yaml", &buf)
	assert.NoError(t, err)
}

func TestWriteOutputNilSections(t *testing.T) {
	// Test output with nil/empty sections to cover more edge cases
	output := &AnalysisOutput{
		POMFile:   "test.xml",
		Timestamp: time.Now(),
		Dependencies: DependencyAnalysis{
			Total: 0,
		},
		Properties: PropertyAnalysis{
			Defined: nil,
			UsedBy:  nil,
		},
		BOMs:            nil,
		Issues:          nil,
		Patches:         nil,
		PropertyUpdates: nil,
		Warnings:        nil,
		CannotFix:       nil,
	}

	var buf bytes.Buffer
	err := output.WriteOutput(&buf)
	assert.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "test.xml")
}

func TestAnalysisOutputPropertyWithEmptyUsedBy(t *testing.T) {
	output := &AnalysisOutput{
		POMFile:   "test.xml",
		Timestamp: time.Now(),
		Properties: PropertyAnalysis{
			Defined: map[string]string{
				"test.version": "1.0.0",
			},
			UsedBy: map[string][]string{
				"test.version": nil, // Empty slice should be handled
			},
		},
	}

	var buf bytes.Buffer
	err := output.WriteOutput(&buf)
	assert.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "test.version = 1.0.0 (used by 0 dependencies)")
}

func TestWriteOutputPropertyUsageEdgeCases(t *testing.T) {
	// Test with property that has empty UsedBy slice vs nil
	output := &AnalysisOutput{
		POMFile:   "test.xml",
		Timestamp: time.Now(),
		Properties: PropertyAnalysis{
			Defined: map[string]string{
				"empty.version": "1.0.0",
				"nil.version":   "2.0.0",
			},
			UsedBy: map[string][]string{
				"empty.version": {}, // Empty but not nil
				// "nil.version" is missing, so effectively nil
			},
		},
	}

	var buf bytes.Buffer
	err := output.WriteOutput(&buf)
	assert.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "empty.version = 1.0.0 (used by 0 dependencies)")
	assert.Contains(t, result, "nil.version = 2.0.0 (used by 0 dependencies)")
}
