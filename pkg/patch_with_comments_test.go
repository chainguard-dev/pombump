package pkg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainguard-dev/gopom"
)

func TestPatchProjectWithCommentPreservation(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "pombump-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			panic(err)
		}
	}()

	// Test POM content with comments
	inputPOM := `<?xml version="1.0" encoding="UTF-8"?>
<!--

    Copyright DataStax, Inc.

    Please see the included license file for details.

-->
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>test</groupId>
    <artifactId>test</artifactId>
    <version>1.0</version>
    <properties>
        <driver.version>4.15.0</driver.version>
        <slf4j.version>2.0.9</slf4j.version>
        <prometheus.version>0.16.0</prometheus.version>
        <!-- This old version is used by Cassandra 4.x -->
        <dropwizard-metrics.version>3.1.5</dropwizard-metrics.version>
    </properties>
    <dependencies>
        <dependency>
            <groupId>org.slf4j</groupId>
            <artifactId>slf4j-api</artifactId>
            <version>${slf4j.version}</version>
        </dependency>
    </dependencies>
</project>`

	// Create input file
	inputPath := filepath.Join(tmpDir, "test-pom.xml")
	if err := os.WriteFile(inputPath, []byte(inputPOM), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	// Parse the POM
	parsedPom, err := gopom.Parse(inputPath)
	if err != nil {
		t.Fatalf("Failed to parse POM: %v", err)
	}

	// Define patches
	patches := []Patch{
		{
			GroupID:    "org.json",
			ArtifactID: "json",
			Version:    "20231013",
			Scope:      "import",
			Type:       "jar",
		},
	}

	// Define property patches
	propertyPatches := map[string]string{
		"driver.version": "4.17.0",
		"slf4j.version":  "2.0.16",
	}

	// Apply patches
	ctx := context.Background()
	patchedPom, err := PatchProject(ctx, parsedPom, patches, propertyPatches)
	if err != nil {
		t.Fatalf("Failed to patch project: %v", err)
	}

	// Marshal the patched POM
	marshalledPom, err := patchedPom.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal POM: %v", err)
	}

	// Apply comment preservation
	result, err := PreserveCommentsInPOMUpdate(inputPath, marshalledPom)
	if err != nil {
		t.Fatalf("Failed to preserve comments: %v", err)
	}

	resultStr := string(result)

	// Verify the result contains the preserved comments
	if !strings.Contains(resultStr, "Copyright DataStax, Inc.") {
		t.Errorf("Copyright comment not preserved in result")
	}

	if !strings.Contains(resultStr, "This old version is used by Cassandra 4.x") {
		t.Errorf("Inline Cassandra comment not preserved in result")
	}

	// Verify the patches were applied
	if !strings.Contains(resultStr, "<driver.version>4.17.0</driver.version>") {
		t.Errorf("driver.version property not updated")
	}

	if !strings.Contains(resultStr, "<slf4j.version>2.0.16</slf4j.version>") {
		t.Errorf("slf4j.version property not updated")
	}

	// Verify the new dependency was added
	if !strings.Contains(resultStr, "org.json") {
		t.Errorf("New dependency org.json not added")
	}

	// Verify the header comment structure is preserved with blank lines
	expectedHeaderPattern := `<?xml version="1.0" encoding="UTF-8"?>
<!--

    Copyright DataStax, Inc.

    Please see the included license file for details.

-->
<project`
	if !strings.Contains(resultStr, expectedHeaderPattern) {
		t.Errorf("Header comment structure not preserved correctly")
	}
}

func TestPatchProjectWithCommentPreservation_PropertiesOnly(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "pombump-props-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			panic(err)
		}
	}()

	// Test POM content with comments
	inputPOM := `<?xml version="1.0" encoding="UTF-8"?>
<!-- Simple copyright notice -->
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>test</groupId>
    <artifactId>test</artifactId>
    <version>1.0</version>
    <properties>
        <java.version>11</java.version>
        <!-- Important: This must match the Spring Boot version -->
        <spring.version>2.7.0</spring.version>
    </properties>
</project>`

	// Create input file
	inputPath := filepath.Join(tmpDir, "test-props-pom.xml")
	if err := os.WriteFile(inputPath, []byte(inputPOM), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	// Parse the POM
	parsedPom, err := gopom.Parse(inputPath)
	if err != nil {
		t.Fatalf("Failed to parse POM: %v", err)
	}

	// Define property patches only
	propertyPatches := map[string]string{
		"spring.version": "2.7.14", // Security update
		"java.version":   "17",     // Update Java version
	}

	// Apply patches with no dependency patches
	ctx := context.Background()
	patchedPom, err := PatchProject(ctx, parsedPom, []Patch{}, propertyPatches)
	if err != nil {
		t.Fatalf("Failed to patch project: %v", err)
	}

	// Marshal the patched POM
	marshalledPom, err := patchedPom.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal POM: %v", err)
	}

	// Apply comment preservation
	result, err := PreserveCommentsInPOMUpdate(inputPath, marshalledPom)
	if err != nil {
		t.Fatalf("Failed to preserve comments: %v", err)
	}

	resultStr := string(result)

	// Verify comments are preserved
	if !strings.Contains(resultStr, "Simple copyright notice") {
		t.Errorf("Header comment not preserved")
	}

	if !strings.Contains(resultStr, "Important: This must match the Spring Boot version") {
		t.Errorf("Inline property comment not preserved")
	}

	// Verify properties were updated
	if !strings.Contains(resultStr, "<spring.version>2.7.14</spring.version>") {
		t.Errorf("spring.version property not updated")
	}

	if !strings.Contains(resultStr, "<java.version>17</java.version>") {
		t.Errorf("java.version property not updated")
	}
}
