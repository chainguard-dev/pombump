package pkg

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainguard-dev/gopom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationWithZipkinServer(t *testing.T) {
	ctx := context.Background()
	pomPath := "testdata/zipkin-server.pom.xml"

	// Skip if file doesn't exist
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		t.Skip("Zipkin server POM not found")
	}

	project, err := gopom.Parse(pomPath)
	require.NoError(t, err, "Failed to parse Zipkin server POM")

	result, err := AnalyzeProject(ctx, project)
	require.NoError(t, err)

	// Zipkin server should have multiple BOMs
	assert.Greater(t, len(result.BOMs), 0, "Should detect BOMs in Zipkin server")

	// Check for specific BOMs we know are there
	bomNames := make(map[string]bool)
	for _, bom := range result.BOMs {
		bomNames[bom.ArtifactID] = true
	}

	// These BOMs should be present based on our grep results
	expectedBOMs := []string{
		"log4j-bom",
		"netty-bom",
		"brave-bom",
		"jackson-bom",
		"micrometer-bom",
	}

	for _, expected := range expectedBOMs {
		assert.True(t, bomNames[expected], "Should find %s BOM", expected)
	}

	// Test structured output
	output := result.ToAnalysisOutput(pomPath, nil, nil)
	assert.Equal(t, len(result.BOMs), len(output.BOMs))

	// Test JSON output
	var buf bytes.Buffer
	err = output.Write("json", &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "brave-bom")
}

func TestIntegrationWithTrino(t *testing.T) {
	ctx := context.Background()
	pomPath := "testdata/trino.pom.xml"

	// Skip if file doesn't exist
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		t.Skip("Trino POM not found")
	}

	project, err := gopom.Parse(pomPath)
	require.NoError(t, err, "Failed to parse Trino POM")

	result, err := AnalyzeProject(ctx, project)
	require.NoError(t, err)

	// Trino has many dependencies and properties
	assert.Greater(t, len(result.Dependencies), 10, "Trino should have many dependencies")
	assert.Greater(t, len(result.Properties), 10, "Trino should have many properties")

	// Test patch strategy with a known dependency
	if len(result.Dependencies) > 0 {
		// Create a patch for one of the dependencies
		var testPatch Patch
		for _, dep := range result.Dependencies {
			testPatch = Patch{
				GroupID:    dep.GroupID,
				ArtifactID: dep.ArtifactID,
				Version:    "999.999.999", // Dummy version for testing
			}
			break
		}

		directPatches, propertyPatches := PatchStrategy(ctx, result, []Patch{testPatch})

		// Should have at least one patch (either direct or property)
		assert.True(t, len(directPatches) > 0 || len(propertyPatches) > 0,
			"Should recommend at least one type of patch")

		// Use the variables to avoid compiler warning
		_ = directPatches
		_ = propertyPatches
	}
}

func TestIntegrationWithZookeeper(t *testing.T) {
	ctx := context.Background()
	pomPath := "testdata/zookeeper.pom.xml"

	// Skip if file doesn't exist
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		t.Skip("Zookeeper POM not found")
	}

	project, err := gopom.Parse(pomPath)
	require.NoError(t, err, "Failed to parse Zookeeper POM")

	result, err := AnalyzeProject(ctx, project)
	require.NoError(t, err)

	// Test with known Zookeeper properties
	if _, exists := result.Properties["slf4j.version"]; exists {
		// Test patching a dependency that uses this property
		patches := []Patch{
			{
				GroupID:    "org.slf4j",
				ArtifactID: "slf4j-api",
				Version:    "2.0.9",
			},
		}

		directPatches, propertyPatches := PatchStrategy(ctx, result, patches)

		// If slf4j uses properties, should recommend property update
		if dep, exists := result.Dependencies["org.slf4j:slf4j-api"]; exists && dep.UsesProperty {
			assert.Contains(t, propertyPatches, "slf4j.version",
				"Should recommend updating slf4j.version property")
			assert.Empty(t, directPatches, "Should not have direct patches for property-based dep")
		} else {
			// If not using properties, should have direct patch
			assert.NotEmpty(t, directPatches, "Should have direct patch for non-property dep")
		}
	}
}

func TestIntegrationOutputFormats(t *testing.T) {
	ctx := context.Background()

	// Use any available test POM
	testPOMs := []string{
		"testdata/zipkin-server.pom.xml",
		"testdata/trino.pom.xml",
		"testdata/zookeeper.pom.xml",
		"testdata/zipkin.pom.xml",
	}

	var pomPath string
	var project *gopom.Project

	// Find first available POM
	for _, path := range testPOMs {
		if _, err := os.Stat(path); err == nil {
			pomPath = path
			p, err := gopom.Parse(path)
			if err == nil {
				project = p
				break
			}
		}
	}

	if project == nil {
		t.Skip("No test POMs available")
	}

	result, err := AnalyzeProject(ctx, project)
	require.NoError(t, err)

	// Create some test patches
	patches := []Patch{
		{GroupID: "test", ArtifactID: "test", Version: "1.0.0"},
	}
	propertyPatches := map[string]string{
		"test.version": "2.0.0",
	}

	output := result.ToAnalysisOutput(pomPath, patches, propertyPatches)

	// Test all output formats
	formats := []string{"json", "yaml", "human"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			err := output.Write(format, &buf)
			require.NoError(t, err, "Should write %s format without error", format)

			content := buf.String()
			assert.NotEmpty(t, content, "Should produce non-empty output")

			// Format-specific checks
			switch format {
			case "json":
				assert.Contains(t, content, `"pom_file"`)
				assert.Contains(t, content, pomPath)
			case "yaml":
				assert.Contains(t, content, "pom_file:")
				assert.Contains(t, content, pomPath)
			case "human":
				assert.Contains(t, content, "POM Analysis:")
			}
		})
	}
}

func TestIntegrationWithPropertySearch(t *testing.T) {
	ctx := context.Background()

	// Create a temporary multi-module project structure
	tmpDir := t.TempDir()

	// Root POM
	rootPOM := `<?xml version="1.0"?>
<project>
	<groupId>com.test</groupId>
	<artifactId>parent</artifactId>
	<version>1.0.0</version>
	<packaging>pom</packaging>
	
	<properties>
		<project.version>1.0.0</project.version>
		<maven.compiler.source>17</maven.compiler.source>
	</properties>
	
	<modules>
		<module>module1</module>
		<module>module2</module>
	</modules>
</project>`

	// Module 1 POM with property usage
	module1POM := `<?xml version="1.0"?>
<project>
	<parent>
		<groupId>com.test</groupId>
		<artifactId>parent</artifactId>
		<version>1.0.0</version>
	</parent>
	
	<artifactId>module1</artifactId>
	
	<properties>
		<jackson.version>2.15.2</jackson.version>
		<netty.version>4.1.90.Final</netty.version>
	</properties>
	
	<dependencies>
		<dependency>
			<groupId>com.fasterxml.jackson.core</groupId>
			<artifactId>jackson-databind</artifactId>
			<version>${jackson.version}</version>
		</dependency>
	</dependencies>
</project>`

	// Module 2 POM with BOM import
	module2POM := `<?xml version="1.0"?>
<project>
	<parent>
		<groupId>com.test</groupId>
		<artifactId>parent</artifactId>
		<version>1.0.0</version>
	</parent>
	
	<artifactId>module2</artifactId>
	
	<dependencyManagement>
		<dependencies>
			<dependency>
				<groupId>org.springframework.boot</groupId>
				<artifactId>spring-boot-dependencies</artifactId>
				<version>2.7.18</version>
				<type>pom</type>
				<scope>import</scope>
			</dependency>
		</dependencies>
	</dependencyManagement>
	
	<dependencies>
		<dependency>
			<groupId>io.netty</groupId>
			<artifactId>netty-handler</artifactId>
			<version>${netty.version}</version>
		</dependency>
	</dependencies>
</project>`

	// Write POM files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(rootPOM), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "module1"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "module1", "pom.xml"), []byte(module1POM), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "module2"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "module2", "pom.xml"), []byte(module2POM), 0600))

	// Analyze module2 with property search
	result, err := AnalyzeProjectPath(ctx, filepath.Join(tmpDir, "module2", "pom.xml"))
	require.NoError(t, err)

	// Should find properties from module1
	assert.Contains(t, result.Properties, "jackson.version",
		"Should find jackson.version from module1")
	assert.Contains(t, result.Properties, "netty.version",
		"Should find netty.version from module1")

	// Should find BOM in module2
	assert.Greater(t, len(result.BOMs), 0, "Should find BOM import in module2")

	// Should find properties from parent
	assert.Contains(t, result.Properties, "project.version",
		"Should find project.version from parent")
}

func TestIntegrationComplexPatching(t *testing.T) {
	ctx := context.Background()

	// Create a POM with various dependency patterns
	complexPOM := &gopom.Project{
		GroupID:    "com.test",
		ArtifactID: "complex",
		Version:    "1.0.0",
		Properties: &gopom.Properties{
			Entries: map[string]string{
				"jackson.version": "2.14.0",
				"netty.version":   "4.1.90.Final",
				"junit.version":   "4.13.2",
				"shared.version":  "1.0.0",
			},
		},
		DependencyManagement: &gopom.DependencyManagement{
			Dependencies: &[]gopom.Dependency{
				// BOM import
				{
					GroupID:    "org.springframework.boot",
					ArtifactID: "spring-boot-dependencies",
					Version:    "2.7.14",
					Type:       "pom",
					Scope:      "import",
				},
				// Regular dependency management
				{
					GroupID:    "com.fasterxml.jackson.core",
					ArtifactID: "jackson-databind",
					Version:    "${jackson.version}",
				},
			},
		},
		Dependencies: &[]gopom.Dependency{
			// Property-based version
			{
				GroupID:    "io.netty",
				ArtifactID: "netty-handler",
				Version:    "${netty.version}",
			},
			// Direct version
			{
				GroupID:    "org.slf4j",
				ArtifactID: "slf4j-api",
				Version:    "1.7.30",
			},
			// Shared property
			{
				GroupID:    "com.example",
				ArtifactID: "lib1",
				Version:    "${shared.version}",
			},
			{
				GroupID:    "com.example",
				ArtifactID: "lib2",
				Version:    "${shared.version}",
			},
		},
	}

	result, err := AnalyzeProject(ctx, complexPOM)
	require.NoError(t, err)

	// Test various patching scenarios
	testCases := []struct {
		name               string
		patches            []Patch
		expectedDirect     int
		expectedProperties map[string]string
		expectedBOMWarning bool
	}{
		{
			name: "patch property-based dependency",
			patches: []Patch{
				{GroupID: "io.netty", ArtifactID: "netty-handler", Version: "4.1.94.Final"},
			},
			expectedDirect: 0,
			expectedProperties: map[string]string{
				"netty.version": "4.1.94.Final",
			},
		},
		{
			name: "patch direct dependency",
			patches: []Patch{
				{GroupID: "org.slf4j", ArtifactID: "slf4j-api", Version: "1.7.36"},
			},
			expectedDirect:     1,
			expectedProperties: map[string]string{},
		},
		{
			name: "patch multiple deps with shared property",
			patches: []Patch{
				{GroupID: "com.example", ArtifactID: "lib1", Version: "2.0.0"},
				{GroupID: "com.example", ArtifactID: "lib2", Version: "2.0.0"},
			},
			expectedDirect: 0,
			expectedProperties: map[string]string{
				"shared.version": "2.0.0",
			},
		},
		{
			name: "conflicting versions for shared property",
			patches: []Patch{
				{GroupID: "com.example", ArtifactID: "lib1", Version: "2.0.0"},
				{GroupID: "com.example", ArtifactID: "lib2", Version: "3.0.0"}, // Different!
			},
			expectedDirect: 0,
			expectedProperties: map[string]string{
				"shared.version": "2.0.0", // First one wins
			},
		},
		{
			name: "update BOM version",
			patches: []Patch{
				{GroupID: "org.springframework.boot", ArtifactID: "spring-boot-dependencies", Version: "2.7.18"},
			},
			expectedDirect:     1, // BOMs are treated as direct patches currently
			expectedProperties: map[string]string{},
			expectedBOMWarning: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			directPatches, propertyPatches := PatchStrategy(ctx, result, tc.patches)

			assert.Equal(t, tc.expectedDirect, len(directPatches),
				"Direct patches count mismatch")
			assert.Equal(t, len(tc.expectedProperties), len(propertyPatches),
				"Property patches count mismatch")

			for prop, version := range tc.expectedProperties {
				assert.Equal(t, version, propertyPatches[prop],
					"Property %s should have version %s", prop, version)
			}
		})
	}
}

func BenchmarkAnalyzeProject(b *testing.B) {
	// Use a complex POM for benchmarking
	pomPath := "testdata/trino.pom.xml"
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		b.Skip("Trino POM not found for benchmark")
	}

	project, err := gopom.Parse(pomPath)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = AnalyzeProject(ctx, project)
	}
}

func BenchmarkOutputFormats(b *testing.B) {
	// Create a large analysis result for benchmarking
	result := &AnalysisResult{
		Dependencies: make(map[string]*DependencyInfo),
		Properties:   make(map[string]string),
		BOMs:         []BOMInfo{},
	}

	// Add many dependencies
	for i := 0; i < 100; i++ {
		depKey := fmt.Sprintf("com.test:lib%d", i)
		result.Dependencies[depKey] = &DependencyInfo{
			GroupID:    "com.test",
			ArtifactID: fmt.Sprintf("lib%d", i),
			Version:    "1.0.0",
		}
	}

	// Add many properties
	for i := 0; i < 50; i++ {
		result.Properties[fmt.Sprintf("prop%d.version", i)] = "1.0.0"
	}

	output := result.ToAnalysisOutput("/test/pom.xml", nil, nil)

	b.Run("JSON", func(b *testing.B) {
		var buf bytes.Buffer
		for i := 0; i < b.N; i++ {
			buf.Reset()
			_ = output.Write("json", &buf)
		}
	})

	b.Run("YAML", func(b *testing.B) {
		var buf bytes.Buffer
		for i := 0; i < b.N; i++ {
			buf.Reset()
			_ = output.Write("yaml", &buf)
		}
	})

	b.Run("Human", func(b *testing.B) {
		var buf bytes.Buffer
		for i := 0; i < b.N; i++ {
			buf.Reset()
			_ = output.Write("human", &buf)
		}
	})
}
