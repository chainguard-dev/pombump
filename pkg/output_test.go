package pkg

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalysisOutputWrite(t *testing.T) {
	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	baseOutput := &AnalysisOutput{
		POMFile:   "/test/pom.xml",
		Timestamp: testTime,
		Dependencies: DependencyAnalysis{
			Total:           5,
			Direct:          2,
			UsingProperties: 3,
		},
		Properties: PropertyAnalysis{
			Defined: map[string]string{
				"jackson.version": "2.15.2",
			},
			UsedBy: map[string][]string{
				"jackson.version": {"com.fasterxml:jackson-core"},
			},
		},
	}

	tests := []struct {
		name           string
		format         string
		expectedFormat string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "json format",
			format:         "json",
			expectedFormat: "json",
			expectError:    false,
		},
		{
			name:           "yaml format",
			format:         "yaml",
			expectedFormat: "yaml",
			expectError:    false,
		},
		{
			name:           "yml format (alias)",
			format:         "yml",
			expectedFormat: "yaml",
			expectError:    false,
		},
		{
			name:           "human format",
			format:         "human",
			expectedFormat: "human",
			expectError:    false,
		},
		{
			name:           "invalid format",
			format:         "xml",
			expectedFormat: "",
			expectError:    true,
			errorContains:  "unsupported output format: xml",
		},
		{
			name:           "empty format defaults to json for non-terminal",
			format:         "",
			expectedFormat: "json",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := baseOutput.Write(tt.format, &buf)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				output := buf.String()

				switch tt.expectedFormat {
				case "json":
					assert.Contains(t, output, `"pom_file"`)
					assert.Contains(t, output, `"/test/pom.xml"`)
				case "yaml":
					assert.Contains(t, output, "pom_file:")
					assert.Contains(t, output, "/test/pom.xml")
				case "human":
					assert.Contains(t, output, "POM Analysis:")
					assert.Contains(t, output, "Dependencies Summary:")
				}
			}
		})
	}
}

func TestAnalysisOutputWithIssues(t *testing.T) {
	output := &AnalysisOutput{
		POMFile:   "/test/pom.xml",
		Timestamp: time.Now(),
		Issues: []Issue{
			{
				Type:            "direct",
				Dependency:      "log4j:log4j",
				CurrentVersion:  "1.2.17",
				RequiredVersion: "1.2.17.redhat-00001",
				CVEs:            []string{"CVE-2022-23305"},
			},
			{
				Type:           "transitive",
				Dependency:     "commons-collections",
				CurrentVersion: "3.2.1",
				Path:           []string{"kafka", "zookeeper", "commons-collections"},
			},
		},
		CannotFix: []UnfixableIssue{
			{
				Dependency: "shaded-jar",
				Reason:     "Contains shaded vulnerable code",
				Action:     "Upgrade to version 2.0+",
			},
		},
	}

	// Test JSON output
	var jsonBuf bytes.Buffer
	err := output.Write("json", &jsonBuf)
	require.NoError(t, err)

	jsonOutput := jsonBuf.String()
	assert.Contains(t, jsonOutput, "CVE-2022-23305")
	assert.Contains(t, jsonOutput, "shaded-jar")
	assert.Contains(t, jsonOutput, "cannot_fix")

	// Test human-readable output
	var humanBuf bytes.Buffer
	err = output.Write("human", &humanBuf)
	require.NoError(t, err)

	humanOutput := humanBuf.String()
	assert.Contains(t, humanOutput, "Issues Found: 2")
	assert.Contains(t, humanOutput, "Cannot Fix")
	assert.Contains(t, humanOutput, "Manual Intervention Required")
}

func TestAnalysisOutputWithWarnings(t *testing.T) {
	output := &AnalysisOutput{
		POMFile:   "/test/pom.xml",
		Timestamp: time.Now(),
		Warnings: []string{
			"Property netty.version is referenced but not found in project",
			"BOM spring-boot-dependencies may override patches",
		},
	}

	var buf bytes.Buffer
	err := output.Write("human", &buf)
	require.NoError(t, err)

	outputStr := buf.String()
	assert.Contains(t, outputStr, "Warnings:")
	assert.Contains(t, outputStr, "Property netty.version")
	assert.Contains(t, outputStr, "BOM spring-boot-dependencies")
}

func TestAnalysisOutputEmptyData(t *testing.T) {
	// Test with completely empty output
	output := &AnalysisOutput{
		POMFile:   "",
		Timestamp: time.Time{},
	}

	var buf bytes.Buffer
	err := output.Write("json", &buf)
	require.NoError(t, err)

	// Should still produce valid JSON
	jsonOutput := buf.String()
	assert.Contains(t, jsonOutput, "{")
	assert.Contains(t, jsonOutput, "}")
	assert.Contains(t, jsonOutput, `"dependencies"`)
}

func TestAnalysisOutputWithNilMaps(t *testing.T) {
	// Test with nil maps (shouldn't panic)
	output := &AnalysisOutput{
		POMFile:   "/test/pom.xml",
		Timestamp: time.Now(),
		Properties: PropertyAnalysis{
			Defined: nil,
			UsedBy:  nil,
		},
		PropertyUpdates: nil,
	}

	// Should not panic for any format
	formats := []string{"json", "yaml", "human"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			err := output.Write(format, &buf)
			require.NoError(t, err, "Should handle nil maps gracefully")
		})
	}
}

func TestAnalysisOutputLargeData(t *testing.T) {
	// Test with large amount of data
	output := &AnalysisOutput{
		POMFile:   "/test/pom.xml",
		Timestamp: time.Now(),
		Properties: PropertyAnalysis{
			Defined: make(map[string]string),
			UsedBy:  make(map[string][]string),
		},
	}

	// Add many properties
	for i := 0; i < 100; i++ {
		propName := strings.Repeat("a", 100) + string(rune(i))
		output.Properties.Defined[propName] = "value" + string(rune(i))
		output.Properties.UsedBy[propName] = []string{"dep1", "dep2", "dep3"}
	}

	// Add many BOMs
	for i := 0; i < 50; i++ {
		output.BOMs = append(output.BOMs, BOMInfo{
			GroupID:    "com.example" + string(rune(i)),
			ArtifactID: "bom" + string(rune(i)),
			Version:    "1.0.0",
			Type:       "pom",
			Scope:      "import",
		})
	}

	// Should handle large data without issues
	var buf bytes.Buffer
	err := output.Write("json", &buf)
	require.NoError(t, err)

	// Output should be substantial
	assert.Greater(t, buf.Len(), 10000, "Large data should produce substantial output")
}

func TestBOMInfoIsBOM(t *testing.T) {
	tests := []struct {
		name     string
		bom      BOMInfo
		expected bool
	}{
		{
			name: "valid BOM",
			bom: BOMInfo{
				Scope: "import",
				Type:  "pom",
			},
			expected: true,
		},
		{
			name: "wrong scope",
			bom: BOMInfo{
				Scope: "compile",
				Type:  "pom",
			},
			expected: false,
		},
		{
			name: "wrong type",
			bom: BOMInfo{
				Scope: "import",
				Type:  "jar",
			},
			expected: false,
		},
		{
			name: "empty values",
			bom: BOMInfo{
				Scope: "",
				Type:  "",
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
