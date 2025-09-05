package pkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeProjectPath(t *testing.T) {
	// Create a temporary directory structure with multiple POMs
	tmpDir := t.TempDir()
	
	// Create project structure:
	// /
	// ├── pom.xml (root)
	// ├── parent/
	// │   └── pom.xml (defines properties)
	// ├── module1/
	// │   └── pom.xml (uses properties)
	// └── module2/
	//     └── submodule/
	//         └── pom.xml
	
	// Root POM
	rootPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <groupId>com.example</groupId>
    <artifactId>root</artifactId>
    <version>1.0.0</version>
    <packaging>pom</packaging>
    
    <properties>
        <project.version>1.0.0</project.version>
    </properties>
    
    <modules>
        <module>parent</module>
        <module>module1</module>
        <module>module2</module>
    </modules>
</project>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(rootPom), 0644))
	
	// Parent POM with properties
	parentDir := filepath.Join(tmpDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0755))
	parentPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <parent>
        <groupId>com.example</groupId>
        <artifactId>root</artifactId>
        <version>1.0.0</version>
        <relativePath>../pom.xml</relativePath>
    </parent>
    
    <artifactId>parent</artifactId>
    
    <properties>
        <netty.version>4.1.94.Final</netty.version>
        <jackson.version>2.15.2</jackson.version>
        <slf4j.version>1.7.30</slf4j.version>
    </properties>
    
    <dependencyManagement>
        <dependencies>
            <dependency>
                <groupId>io.netty</groupId>
                <artifactId>netty-handler</artifactId>
                <version>${netty.version}</version>
            </dependency>
        </dependencies>
    </dependencyManagement>
</project>`
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "pom.xml"), []byte(parentPom), 0644))
	
	// Module1 POM using properties
	module1Dir := filepath.Join(tmpDir, "module1")
	require.NoError(t, os.MkdirAll(module1Dir, 0755))
	module1Pom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <parent>
        <groupId>com.example</groupId>
        <artifactId>parent</artifactId>
        <version>1.0.0</version>
        <relativePath>../parent/pom.xml</relativePath>
    </parent>
    
    <artifactId>module1</artifactId>
    
    <dependencies>
        <dependency>
            <groupId>io.netty</groupId>
            <artifactId>netty-handler</artifactId>
            <version>${netty.version}</version>
        </dependency>
        <dependency>
            <groupId>com.fasterxml.jackson.core</groupId>
            <artifactId>jackson-databind</artifactId>
            <version>${jackson.version}</version>
        </dependency>
        <dependency>
            <groupId>junit</groupId>
            <artifactId>junit</artifactId>
            <version>4.13.2</version>
        </dependency>
    </dependencies>
</project>`
	require.NoError(t, os.WriteFile(filepath.Join(module1Dir, "pom.xml"), []byte(module1Pom), 0644))
	
	// Module2 with submodule
	module2Dir := filepath.Join(tmpDir, "module2", "submodule")
	require.NoError(t, os.MkdirAll(module2Dir, 0755))
	module2Pom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <artifactId>submodule</artifactId>
    <version>1.0.0</version>
    
    <properties>
        <local.property>local-value</local.property>
    </properties>
    
    <dependencies>
        <dependency>
            <groupId>org.slf4j</groupId>
            <artifactId>slf4j-api</artifactId>
            <version>${slf4j.version}</version>
        </dependency>
    </dependencies>
</project>`
	require.NoError(t, os.WriteFile(filepath.Join(module2Dir, "pom.xml"), []byte(module2Pom), 0644))
	
	// Test analyzing module1 with property search
	ctx := context.Background()
	module1PomPath := filepath.Join(module1Dir, "pom.xml")
	
	t.Run("analyze with property search", func(t *testing.T) {
		result, err := AnalyzeProjectPath(ctx, module1PomPath)
		require.NoError(t, err)
		
		// Should find dependencies
		assert.Equal(t, 3, len(result.Dependencies))
		
		// Should find properties from parent POM
		assert.Contains(t, result.Properties, "netty.version")
		assert.Equal(t, "4.1.94.Final", result.Properties["netty.version"])
		assert.Contains(t, result.Properties, "jackson.version")
		assert.Equal(t, "2.15.2", result.Properties["jackson.version"])
		
		// Should also find properties from root POM
		assert.Contains(t, result.Properties, "project.version")
		
		// Should detect property usage
		usesProp, propName := result.ShouldUseProperty("io.netty", "netty-handler")
		assert.True(t, usesProp)
		assert.Equal(t, "netty.version", propName)
	})
	
	t.Run("patch strategy with found properties", func(t *testing.T) {
		result, err := AnalyzeProjectPath(ctx, module1PomPath)
		require.NoError(t, err)
		
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
		}
		
		directPatches, propertyPatches := PatchStrategy(ctx, result, patches)
		
		// netty should use property
		assert.Len(t, propertyPatches, 1)
		assert.Equal(t, "4.1.118.Final", propertyPatches["netty.version"])
		
		// junit should be direct
		assert.Len(t, directPatches, 1)
		assert.Equal(t, "junit", directPatches[0].ArtifactID)
	})
}

func TestFindProjectRoot(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create nested structure
	// /tmp/
	// ├── project/
	// │   ├── pom.xml
	// │   └── module/
	// │       ├── pom.xml
	// │       └── submodule/
	// │           └── pom.xml
	// └── other/
	
	projectDir := filepath.Join(tmpDir, "project")
	moduleDir := filepath.Join(projectDir, "module")
	submoduleDir := filepath.Join(moduleDir, "submodule")
	otherDir := filepath.Join(tmpDir, "other")
	
	require.NoError(t, os.MkdirAll(submoduleDir, 0755))
	require.NoError(t, os.MkdirAll(otherDir, 0755))
	
	// Create POM files
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <artifactId>test</artifactId>
    <version>1.0.0</version>
</project>`
	
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "pom.xml"), []byte(pomContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "pom.xml"), []byte(pomContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(submoduleDir, "pom.xml"), []byte(pomContent), 0644))
	
	tests := []struct {
		name     string
		startDir string
		wantRoot string
	}{
		{
			name:     "from project root",
			startDir: projectDir,
			wantRoot: projectDir,
		},
		{
			name:     "from module",
			startDir: moduleDir,
			wantRoot: projectDir,
		},
		{
			name:     "from submodule",
			startDir: submoduleDir,
			wantRoot: projectDir,
		},
		{
			name:     "from directory without pom",
			startDir: otherDir,
			wantRoot: otherDir,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findProjectRoot(tt.startDir)
			assert.Equal(t, tt.wantRoot, got)
		})
	}
}

func TestFindPropertyLocation(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create project structure
	parentDir := filepath.Join(tmpDir, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0755))
	
	// Root POM with some properties
	rootPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <properties>
        <root.property>root-value</root.property>
    </properties>
</project>`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(rootPom), 0644))
	
	// Parent POM with different properties
	parentPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <properties>
        <parent.property>parent-value</parent.property>
        <shared.property>shared-value</shared.property>
    </properties>
</project>`
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "pom.xml"), []byte(parentPom), 0644))
	
	ctx := context.Background()
	
	t.Run("find property in root", func(t *testing.T) {
		path, value, err := FindPropertyLocation(ctx, parentDir, "root.property")
		require.NoError(t, err)
		assert.Contains(t, path, "pom.xml")
		assert.Equal(t, "root-value", value)
	})
	
	t.Run("find property in parent", func(t *testing.T) {
		path, value, err := FindPropertyLocation(ctx, parentDir, "parent.property")
		require.NoError(t, err)
		assert.Contains(t, path, filepath.Join("parent", "pom.xml"))
		assert.Equal(t, "parent-value", value)
	})
	
	t.Run("property not found", func(t *testing.T) {
		_, _, err := FindPropertyLocation(ctx, parentDir, "nonexistent.property")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in project")
		assert.Contains(t, err.Error(), "external parent POM")
	})
}

func TestSearchForPropertiesSkipsHiddenAndBuildDirs(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create directories that should be skipped
	hiddenDir := filepath.Join(tmpDir, ".git")
	targetDir := filepath.Join(tmpDir, "target")
	nodeDir := filepath.Join(tmpDir, "node_modules")
	validDir := filepath.Join(tmpDir, "src")
	
	for _, dir := range []string{hiddenDir, targetDir, nodeDir, validDir} {
		require.NoError(t, os.MkdirAll(dir, 0755))
	}
	
	// Create POMs in each directory
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <properties>
        <test.property>%s</test.property>
    </properties>
</project>`
	
	// This should be skipped
	require.NoError(t, os.WriteFile(
		filepath.Join(hiddenDir, "pom.xml"),
		[]byte(fmt.Sprintf(pomContent, "hidden")),
		0644))
	
	// This should be skipped
	require.NoError(t, os.WriteFile(
		filepath.Join(targetDir, "pom.xml"),
		[]byte(fmt.Sprintf(pomContent, "target")),
		0644))
	
	// This should be skipped
	require.NoError(t, os.WriteFile(
		filepath.Join(nodeDir, "pom.xml"),
		[]byte(fmt.Sprintf(pomContent, "node")),
		0644))
	
	// This should be found
	require.NoError(t, os.WriteFile(
		filepath.Join(validDir, "pom.xml"),
		[]byte(fmt.Sprintf(pomContent, "valid")),
		0644))
	
	ctx := context.Background()
	props := searchForProperties(ctx, tmpDir, "")
	
	// Should only find the property from the valid directory
	assert.Equal(t, "valid", props["test.property"])
	assert.Len(t, props, 1)
}

func TestAnalyzeProjectPathWithNonPomXMLFiles(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create various XML files with different names
	files := map[string]string{
		"pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <artifactId>main</artifactId>
    <version>1.0.0</version>
    <properties>
        <main.property>main-value</main.property>
    </properties>
</project>`,
		"parent-pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <artifactId>parent</artifactId>
    <properties>
        <parent.property>parent-value</parent.property>
    </properties>
</project>`,
		"dependencies.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <artifactId>deps</artifactId>
    <properties>
        <deps.property>deps-value</deps.property>
    </properties>
</project>`,
		"not-a-pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <setting>value</setting>
</configuration>`,
		"build.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project name="ant-project">
    <target name="build"/>
</project>`,
	}
	
	for filename, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644))
	}
	
	ctx := context.Background()
	result, err := AnalyzeProjectPath(ctx, filepath.Join(tmpDir, "pom.xml"))
	require.NoError(t, err)
	
	// Should find properties from all valid POM files regardless of name
	assert.Contains(t, result.Properties, "main.property")
	assert.Contains(t, result.Properties, "parent.property")
	assert.Contains(t, result.Properties, "deps.property")
	
	// Should not have properties from non-POM XML files
	assert.NotContains(t, result.Properties, "setting")
}