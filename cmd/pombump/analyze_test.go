package pombump

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainguard-dev/pombump/pkg"
)

func TestAnalyzeCommand(t *testing.T) {
	// Store original logger and stdout
	originalLogger := slog.Default()
	originalStdout := os.Stdout
	defer func() {
		slog.SetDefault(originalLogger)
		os.Stdout = originalStdout
	}()

	tests := []struct {
		name        string
		args        []string
		expectError bool
		expectOut   string
	}{
		{
			name:        "basic analysis without patches",
			args:        []string{"analyze", "../../testdata/simple.pom.xml"},
			expectError: false,
			expectOut:   "POM Analysis Report",
		},
		{
			name:        "analysis with patches flag",
			args:        []string{"analyze", "../../testdata/complex.pom.xml", "--patches", "junit@junit@4.13.2"},
			expectError: false,
			expectOut:   "Patch Recommendations",
		},
		{
			name:        "analysis with patch-file",
			args:        []string{"analyze", "../../testdata/complex.pom.xml", "--patch-file", "../../testdata/test-patches.yaml"},
			expectError: false,
			expectOut:   "Patch Recommendations",
		},
		{
			name:        "analysis with search-properties",
			args:        []string{"analyze", "../../testdata/complex.pom.xml", "--search-properties"},
			expectError: false,
			expectOut:   "POM Analysis Report",
		},
		{
			name:        "analysis with yaml output",
			args:        []string{"analyze", "../../testdata/complex.pom.xml", "--patches", "junit@junit@4.13.2", "--output", "yaml"},
			expectError: false,
			expectOut:   "properties:",
		},
		{
			name:        "invalid POM file",
			args:        []string{"analyze", "nonexistent.pom"},
			expectError: true,
			expectOut:   "failed to parse POM file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd := New()
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			// Close write end and read output
			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = originalStdout

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.expectOut) {
					t.Errorf("Expected error message '%s' but got '%s'", tt.expectOut, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !strings.Contains(string(output), tt.expectOut) {
					t.Errorf("Expected output to contain '%s' but got '%s'", tt.expectOut, string(output))
				}
			}
		})
	}
}

func TestAnalyzeCommandFileOutput(t *testing.T) {
	// Store original logger and stdout
	originalLogger := slog.Default()
	originalStdout := os.Stdout
	defer func() {
		slog.SetDefault(originalLogger)
		os.Stdout = originalStdout
	}()

	tempDir := t.TempDir()
	depsFile := filepath.Join(tempDir, "deps.yaml")
	propsFile := filepath.Join(tempDir, "props.yaml")

	// Capture stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := New()
	cmd.SetArgs([]string{
		"analyze", 
		"../../testdata/complex.pom.xml",
		"--patches", "junit@junit@4.13.2 io.netty@netty-handler@4.1.118.Final",
		"--output-deps", depsFile,
		"--output-properties", propsFile,
	})

	err := cmd.Execute()

	// Close write end and read output
	w.Close()
	output, _ := io.ReadAll(r)
	os.Stdout = originalStdout

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Check that properties file was created (deps file only created if there are direct patches)
	if _, err := os.Stat(propsFile); os.IsNotExist(err) {
		t.Errorf("Properties file was not created: %s", propsFile)
	}

	// Check that output mentions the files
	outputStr := string(output)
	if !strings.Contains(outputStr, "Wrote") {
		t.Errorf("Expected output to mention file writing. Got: %s", outputStr)
	}
}

func TestOutputAnalysisReport(t *testing.T) {
	// Store original stdout
	originalStdout := os.Stdout
	defer func() {
		os.Stdout = originalStdout
	}()

	// Create test analysis result
	analysis := &pkg.AnalysisResult{
		Properties: map[string]string{
			"junit.version": "4.12",
		},
		Dependencies: map[string]*pkg.DependencyInfo{
			"junit:junit": {
				GroupID:    "junit",
				ArtifactID: "junit",
				Version:    "4.12",
			},
		},
	}

	directPatches := []pkg.Patch{
		{
			GroupID:    "org.slf4j",
			ArtifactID: "slf4j-api",
			Version:    "1.7.36",
		},
	}

	propertyPatches := map[string]string{
		"junit.version": "4.13.2",
	}

	// Capture stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputAnalysisReport(analysis, directPatches, propertyPatches)

	// Close write end and read output
	w.Close()
	output, _ := io.ReadAll(r)

	outputStr := string(output)

	// Check for expected sections
	expectedSections := []string{
		"Patch Recommendations",
		"Property Updates:",
		"junit.version: 4.12 -> 4.13.2",
		"Direct Dependency Updates:",
		"org.slf4j:slf4j-api: (new) -> 1.7.36",
		"Summary:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(outputStr, section) {
			t.Errorf("Expected output to contain '%s'. Got: %s", section, outputStr)
		}
	}
}

func TestOutputYAML(t *testing.T) {
	// Store original stdout
	originalStdout := os.Stdout
	defer func() {
		os.Stdout = originalStdout
	}()

	directPatches := []pkg.Patch{
		{
			GroupID:    "junit",
			ArtifactID: "junit",
			Version:    "4.13.2",
			Scope:      "test",
			Type:       "jar",
		},
	}

	propertyPatches := map[string]string{
		"junit.version": "4.13.2",
	}

	// Capture stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputYAML(directPatches, propertyPatches)

	// Close write end and read output
	w.Close()
	output, _ := io.ReadAll(r)

	outputStr := string(output)

	// Check for YAML format
	expectedContent := []string{
		"patches:",
		"artifactId: junit",
		"groupId: junit",
		"version: 4.13.2",
		"properties:",
		"property: junit.version",
		"value: 4.13.2",
	}

	for _, content := range expectedContent {
		if !strings.Contains(outputStr, content) {
			t.Errorf("Expected YAML output to contain '%s'. Got: %s", content, outputStr)
		}
	}
}

func TestWriteDepsFile(t *testing.T) {
	tempDir := t.TempDir()
	depsFile := filepath.Join(tempDir, "deps.yaml")

	patches := []pkg.Patch{
		{
			GroupID:    "junit",
			ArtifactID: "junit",
			Version:    "4.13.2",
			Scope:      "test",
			Type:       "jar",
		},
		{
			GroupID:    "org.slf4j",
			ArtifactID: "slf4j-api",
			Version:    "1.7.36",
		},
	}

	// Test writing to new file
	err := writeDepsFile(depsFile, patches)
	if err != nil {
		t.Fatalf("Failed to write deps file: %v", err)
	}

	// Check file was created
	if _, err := os.Stat(depsFile); os.IsNotExist(err) {
		t.Errorf("Dependencies file was not created: %s", depsFile)
	}

	// Read and verify content
	content, err := os.ReadFile(depsFile)
	if err != nil {
		t.Fatalf("Failed to read deps file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "junit") {
		t.Errorf("Expected deps file to contain 'junit'. Got: %s", contentStr)
	}

	// Test appending to existing file (should update existing patch)
	updatedPatches := []pkg.Patch{
		{
			GroupID:    "junit",
			ArtifactID: "junit",
			Version:    "4.13.3", // Updated version
			Scope:      "test",
			Type:       "jar",
		},
	}

	err = writeDepsFile(depsFile, updatedPatches)
	if err != nil {
		t.Fatalf("Failed to update deps file: %v", err)
	}

	// Read and verify updated content
	updatedContent, err := os.ReadFile(depsFile)
	if err != nil {
		t.Fatalf("Failed to read updated deps file: %v", err)
	}

	updatedContentStr := string(updatedContent)
	if !strings.Contains(updatedContentStr, "4.13.3") {
		t.Errorf("Expected updated deps file to contain '4.13.3'. Got: %s", updatedContentStr)
	}
}

func TestWritePropertiesFile(t *testing.T) {
	tempDir := t.TempDir()
	propsFile := filepath.Join(tempDir, "props.yaml")

	properties := map[string]string{
		"junit.version":  "4.13.2",
		"slf4j.version":  "1.7.36",
	}

	// Test writing to new file
	err := writePropertiesFile(propsFile, properties)
	if err != nil {
		t.Fatalf("Failed to write properties file: %v", err)
	}

	// Check file was created
	if _, err := os.Stat(propsFile); os.IsNotExist(err) {
		t.Errorf("Properties file was not created: %s", propsFile)
	}

	// Read and verify content
	content, err := os.ReadFile(propsFile)
	if err != nil {
		t.Fatalf("Failed to read properties file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "junit.version") {
		t.Errorf("Expected properties file to contain 'junit.version'. Got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "4.13.2") {
		t.Errorf("Expected properties file to contain '4.13.2'. Got: %s", contentStr)
	}

	// Test appending to existing file (should update existing property)
	updatedProperties := map[string]string{
		"junit.version":  "4.13.3", // Updated version
		"netty.version": "4.1.118.Final", // New property
	}

	err = writePropertiesFile(propsFile, updatedProperties)
	if err != nil {
		t.Fatalf("Failed to update properties file: %v", err)
	}

	// Read and verify updated content
	updatedContent, err := os.ReadFile(propsFile)
	if err != nil {
		t.Fatalf("Failed to read updated properties file: %v", err)
	}

	updatedContentStr := string(updatedContent)
	if !strings.Contains(updatedContentStr, "4.13.3") {
		t.Errorf("Expected updated properties file to contain '4.13.3'. Got: %s", updatedContentStr)
	}
	if !strings.Contains(updatedContentStr, "netty.version") {
		t.Errorf("Expected updated properties file to contain 'netty.version'. Got: %s", updatedContentStr)
	}
}

func TestWriteFileErrors(t *testing.T) {
	// Test write to invalid directory
	err := writeDepsFile("/invalid/path/deps.yaml", []pkg.Patch{})
	if err == nil {
		t.Error("Expected error writing to invalid path but got none")
	}

	err = writePropertiesFile("/invalid/path/props.yaml", map[string]string{})
	if err == nil {
		t.Error("Expected error writing to invalid path but got none")
	}
}

func TestAnalyzeCommandWithInvalidPatches(t *testing.T) {
	cmd := New()
	
	// Test with invalid patch format
	cmd.SetArgs([]string{"analyze", "../../testdata/simple.pom.xml", "--patches", "invalid-format"})
	
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid patch format but got none")
	}
	
	if !strings.Contains(err.Error(), "failed to parse patches") {
		t.Errorf("Expected patch parsing error but got: %v", err)
	}
}

func TestAnalyzeProjectPathError(t *testing.T) {
	cmd := New()
	
	// Test with invalid project path for search-properties
	cmd.SetArgs([]string{"analyze", "/invalid/project/path/pom.xml", "--search-properties"})
	
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid project path but got none")
	}
	
	if !strings.Contains(err.Error(), "failed to analyze project") {
		t.Errorf("Expected project analysis error but got: %v", err)
	}
}

func TestAnalyzeCommandValidation(t *testing.T) {
	cmd := New()
	
	// Test with no arguments
	cmd.SetArgs([]string{"analyze"})
	
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for no arguments but got none")
	}
	
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Errorf("Expected argument validation error but got: %v", err)
	}
}

func TestOutputAnalysisReportWithEmptyPatches(t *testing.T) {
	// Store original stdout
	originalStdout := os.Stdout
	defer func() {
		os.Stdout = originalStdout
	}()

	// Create test analysis result
	analysis := &pkg.AnalysisResult{
		Properties: map[string]string{},
		Dependencies: map[string]*pkg.DependencyInfo{
			"test:test": {
				GroupID:    "test",
				ArtifactID: "test",
				Version:    "1.0",
			},
		},
	}

	// Test with no patches
	var directPatches []pkg.Patch
	propertyPatches := map[string]string{}

	// Capture stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputAnalysisReport(analysis, directPatches, propertyPatches)

	// Close write end and read output
	w.Close()
	output, _ := io.ReadAll(r)

	outputStr := string(output)

	// Check for summary with 0 updates
	if !strings.Contains(outputStr, "Summary: 0 property updates, 0 direct dependency updates") {
		t.Errorf("Expected summary with 0 updates. Got: %s", outputStr)
	}
}