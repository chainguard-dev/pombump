// Package pkg provides core functionality for analyzing and patching Maven POM files.
package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainguard-dev/clog"
	"github.com/chainguard-dev/gopom"
)

// DependencyInfo contains information about how a dependency is defined
type DependencyInfo struct {
	GroupID            string
	ArtifactID         string
	Version            string
	UsesProperty       bool
	PropertyName       string
	PropertyUsageCount int
}

// AnalysisResult contains the analysis of a POM project
type AnalysisResult struct {
	// Dependencies maps groupId:artifactId to dependency info
	Dependencies map[string]*DependencyInfo
	// PropertyUsageCounts tracks how many times each property is used
	PropertyUsageCounts map[string]int
	// Properties contains the actual property values from the POM
	Properties map[string]string
	// BOMs contains imported BOM information
	BOMs []BOMInfo
	// TransitiveDependencies contains transitive dependency information (if analyzed)
	TransitiveDependencies []TransitiveDependency
}

// AnalyzeProject analyzes a POM project to understand how dependencies are defined
func AnalyzeProject(ctx context.Context, project *gopom.Project) (*AnalysisResult, error) {
	log := clog.FromContext(ctx)

	if project == nil {
		return nil, fmt.Errorf("project is nil")
	}

	result := &AnalysisResult{
		Dependencies:        make(map[string]*DependencyInfo),
		PropertyUsageCounts: make(map[string]int),
		Properties:          make(map[string]string),
	}

	// Extract existing properties
	result.Properties = extractPropertiesFromProject(project)

	// Analyze regular dependencies
	if project.Dependencies != nil {
		for _, dep := range *project.Dependencies {
			analyzeDependency(ctx, dep, result)
		}
	}

	// Analyze dependency management section
	if project.DependencyManagement != nil && project.DependencyManagement.Dependencies != nil {
		for _, dep := range *project.DependencyManagement.Dependencies {
			// Check if this is a BOM import
			if isBOMImport(dep) {
				result.BOMs = append(result.BOMs, BOMInfo{
					GroupID:    dep.GroupID,
					ArtifactID: dep.ArtifactID,
					Version:    dep.Version,
					Type:       dep.Type,
					Scope:      dep.Scope,
				})
				log.Debugf("Found BOM import: %s:%s:%s", dep.GroupID, dep.ArtifactID, dep.Version)
			} else {
				analyzeDependency(ctx, dep, result)
			}
		}
	}

	log.Infof("Analysis complete: found %d dependencies, %d using properties, %d BOMs",
		len(result.Dependencies), countPropertiesUsage(result), len(result.BOMs))

	return result, nil
}

// AnalyzeProjectPath analyzes a POM file and searches for properties in nearby POM files
func AnalyzeProjectPath(ctx context.Context, pomPath string) (*AnalysisResult, error) {
	log := clog.FromContext(ctx)

	// Get absolute path for consistency
	absPomPath, err := filepath.Abs(pomPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	log.Debugf("Analyzing POM with property search: %s", absPomPath)

	// First analyze the main POM
	project, err := gopom.Parse(absPomPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse POM file: %w", err)
	}

	result, err := AnalyzeProject(ctx, project)
	if err != nil {
		return nil, err
	}

	log.Debugf("Main POM analysis found %d properties, %d dependencies",
		len(result.Properties), len(result.Dependencies))

	// Search for additional properties in nearby POMs
	dir := filepath.Dir(absPomPath)
	additionalProps := searchForProperties(ctx, dir, absPomPath)

	log.Debugf("Property search found %d additional properties", len(additionalProps))

	// Merge additional properties
	mergeProperties(ctx, result.Properties, additionalProps, "nearby POM")

	log.Infof("Total after merge: %d properties, %d dependencies",
		len(result.Properties), len(result.Dependencies))

	return result, nil
}

// analyzeDependency analyzes a single dependency
func analyzeDependency(ctx context.Context, dep gopom.Dependency, result *AnalysisResult) {
	log := clog.FromContext(ctx)

	depKey := fmt.Sprintf("%s:%s", dep.GroupID, dep.ArtifactID)

	info := &DependencyInfo{
		GroupID:    dep.GroupID,
		ArtifactID: dep.ArtifactID,
		Version:    dep.Version,
	}

	// Check if version uses a property reference
	if strings.HasPrefix(dep.Version, "${") && strings.HasSuffix(dep.Version, "}") {
		propertyName := strings.TrimSuffix(strings.TrimPrefix(dep.Version, "${"), "}")
		info.UsesProperty = true
		info.PropertyName = propertyName
		result.PropertyUsageCounts[propertyName]++
		info.PropertyUsageCount = result.PropertyUsageCounts[propertyName]

		log.Debugf("Dependency %s uses property %s (total usage: %d)",
			depKey, propertyName, info.PropertyUsageCount)
	}

	result.Dependencies[depKey] = info
}

// countPropertiesUsage counts how many dependencies use properties
func countPropertiesUsage(result *AnalysisResult) int {
	count := 0
	for _, dep := range result.Dependencies {
		if dep.UsesProperty {
			count++
		}
	}
	return count
}

// ShouldUseProperty determines if a specific dependency should be updated via property
func (result *AnalysisResult) ShouldUseProperty(groupID, artifactID string) (bool, string) {
	depKey := fmt.Sprintf("%s:%s", groupID, artifactID)

	if info, exists := result.Dependencies[depKey]; exists {
		if info.UsesProperty {
			return true, info.PropertyName
		}
	}

	// No property reference found
	return false, ""
}

// PatchStrategy recommends BOM updates first, then properties, then direct patches
// Returns direct patches and property patches separately, plus BOM recommendations
func PatchStrategy(ctx context.Context, result *AnalysisResult, patches []Patch) ([]Patch, map[string]string) {
	log := clog.FromContext(ctx)

	log.Debugf("Determining BOM-first patch strategy for %d patches", len(patches))
	log.Debugf("Available properties: %d, Dependencies: %d, BOMs: %d", len(result.Properties), len(result.Dependencies), len(result.BOMs))

	// Step 1: Detect version conflicts and recommend BOM updates
	conflicts := detectVersionConflicts(ctx, result, patches)

	directPatches := []Patch{}
	propertyPatches := make(map[string]string)
	missingProperties := []string{}
	bomRecommendations := []Patch{}
	processedGroupIDs := make(map[string]bool)

	// Step 2: Handle conflicts with BOM recommendations
	for _, conflict := range conflicts {
		if conflict.BOMCandidate != nil && conflict.RecommendedAction == "update_bom" {
			// Recommend updating the BOM instead of individual patches
			optimalVersion := calculateOptimalBOMVersion(conflict.RequestedVersions)
			bomPatch := Patch{
				GroupID:    conflict.BOMCandidate.GroupID,
				ArtifactID: conflict.BOMCandidate.ArtifactID,
				Version:    optimalVersion,
				Type:       "pom",
				Scope:      "import",
			}
			bomRecommendations = append(bomRecommendations, bomPatch)
			processedGroupIDs[conflict.GroupID] = true

			log.Infof("RECOMMENDED: Update BOM %s:%s to %s instead of individual patches for group %s",
				bomPatch.GroupID, bomPatch.ArtifactID, bomPatch.Version, conflict.GroupID)
		}
	}

	// Step 3: Process remaining patches (BOM-first, then properties, then direct)
	for _, patch := range patches {
		// Skip patches that are handled by BOM recommendations
		if processedGroupIDs[patch.GroupID] {
			log.Debugf("Skipping %s:%s (handled by BOM recommendation)", patch.GroupID, patch.ArtifactID)
			continue
		}

		depKey := fmt.Sprintf("%s:%s", patch.GroupID, patch.ArtifactID)

		// Check if this could be handled by a BOM (single dependency from a group with a BOM)
		bomCandidate := findBOMForGroup(result, patch.GroupID)
		if bomCandidate != nil {
			log.Debugf("Found BOM candidate %s:%s for %s:%s",
				bomCandidate.GroupID, bomCandidate.ArtifactID, patch.GroupID, patch.ArtifactID)

			// For single dependencies, we could still recommend BOM update
			// but let's proceed with property/direct logic for now unless there are conflicts
		}

		// Check if dependency uses properties (Step 4: Properties second)
		useProperty, propertyName := result.ShouldUseProperty(patch.GroupID, patch.ArtifactID)

		log.Debugf("Checking patch for %s version %s", depKey, patch.Version)

		if useProperty && propertyName != "" {
			log.Debugf("  -> Dependency %s uses property ${%s}", depKey, propertyName)

			// Check if we already have this property
			if existingVersion, exists := propertyPatches[propertyName]; exists {
				log.Warnf("Property %s already set to %s, requested %s for %s:%s",
					propertyName, existingVersion, patch.Version, patch.GroupID, patch.ArtifactID)
				// Compare versions and use the newer one
			} else {
				propertyPatches[propertyName] = patch.Version

				// Check if this property is actually defined somewhere
				if currentValue, exists := result.Properties[propertyName]; exists {
					log.Infof("Will update property %s from %s to %s", propertyName, currentValue, patch.Version)
				} else {
					log.Warnf("Property %s is referenced but not found in project - it may be defined in an external parent POM", propertyName)
					missingProperties = append(missingProperties, propertyName)
				}
			}
		} else {
			// Step 5: Direct patches last
			if _, exists := result.Dependencies[depKey]; exists {
				log.Debugf("  -> Dependency %s found but doesn't use properties", depKey)
			} else {
				log.Debugf("  -> Dependency %s not found in POM (may be from BOM or new)", depKey)
			}
			directPatches = append(directPatches, patch)
			log.Infof("Will directly patch %s:%s to %s", patch.GroupID, patch.ArtifactID, patch.Version)
		}
	}

	// Add BOM recommendations to direct patches (they'll be handled as direct dependency updates)
	directPatches = append(directPatches, bomRecommendations...)

	if len(missingProperties) > 0 {
		log.Warnf("The following properties are referenced but not found in the project: %s", strings.Join(missingProperties, ", "))
		log.Warnf("These properties may be defined in an external parent POM or imported dependency")
	}

	if len(conflicts) > 0 {
		log.Warnf("Detected %d version conflicts - recommended %d BOM updates", len(conflicts), len(bomRecommendations))
	}

	log.Infof("Strategy: %d direct patches, %d property updates (including %d BOM recommendations)",
		len(directPatches), len(propertyPatches), len(bomRecommendations))

	return directPatches, propertyPatches
}

// GetAffectedDependencies returns all dependencies that would be affected by updating a property
func (result *AnalysisResult) GetAffectedDependencies(propertyName string) []*DependencyInfo {
	affected := []*DependencyInfo{}

	for _, dep := range result.Dependencies {
		if dep.UsesProperty && dep.PropertyName == propertyName {
			affected = append(affected, dep)
		}
	}

	return affected
}

// AnalysisReport generates a human-readable report of the analysis
func (result *AnalysisResult) AnalysisReport() string {
	var report strings.Builder

	report.WriteString("POM Analysis Report\n")
	report.WriteString("===================\n\n")

	report.WriteString(fmt.Sprintf("Total dependencies: %d\n", len(result.Dependencies)))
	report.WriteString(fmt.Sprintf("Dependencies using properties: %d\n", countPropertiesUsage(result)))
	report.WriteString(fmt.Sprintf("Total properties defined: %d\n\n", len(result.Properties)))

	if len(result.PropertyUsageCounts) > 0 || len(result.Properties) > 0 {
		report.WriteString("Property Usage:\n")
		report.WriteString("---------------\n")
		
		// First, show all used properties (from PropertyUsageCounts)
		for prop, count := range result.PropertyUsageCounts {
			currentValue := result.Properties[prop]
			if currentValue != "" {
				report.WriteString(fmt.Sprintf("  %s = %s (used by %d dependencies)\n", prop, currentValue, count))
			} else {
				report.WriteString(fmt.Sprintf("  %s (used by %d dependencies) - NOT DEFINED\n", prop, count))
			}
		}
		
		// Then, show defined properties that are not used (not in PropertyUsageCounts)
		for prop, value := range result.Properties {
			if _, used := result.PropertyUsageCounts[prop]; !used {
				report.WriteString(fmt.Sprintf("  %s = %s (used by 0 dependencies)\n", prop, value))
			}
		}
		
		report.WriteString("\n")
	}

	// List dependencies that use properties
	depsWithProps := []*DependencyInfo{}
	for _, dep := range result.Dependencies {
		if dep.UsesProperty {
			depsWithProps = append(depsWithProps, dep)
		}
	}

	if len(depsWithProps) > 0 {
		report.WriteString("Dependencies Using Properties:\n")
		report.WriteString("-------------------------------\n")
		for _, dep := range depsWithProps {
			report.WriteString(fmt.Sprintf("  %s:%s -> ${%s}\n",
				dep.GroupID, dep.ArtifactID, dep.PropertyName))
		}
	}

	return report.String()
}

// searchForProperties recursively searches for all properties in the project
func searchForProperties(ctx context.Context, startDir string, excludePath string) map[string]string {
	log := clog.FromContext(ctx)
	properties := make(map[string]string)
	pomFilesChecked := 0
	pomFilesSkipped := 0

	// First, find the project root (go up until we find the topmost pom.xml)
	projectRoot := findProjectRoot(startDir)
	log.Debugf("Starting property search from project root: %s", projectRoot)
	log.Debugf("Excluding file: %s", excludePath)

	// Recursively walk the entire project tree
	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip hidden directories and common non-source directories
		if info.IsDir() {
			if isSkippableDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process XML files (POMs can have any name)
		if !strings.HasSuffix(info.Name(), ".xml") {
			return nil
		}

		// Skip the file we're already analyzing
		if absPath, _ := filepath.Abs(path); absPath == excludePath {
			log.Debugf("Skipping excluded file: %s", path)
			pomFilesSkipped++
			return nil
		}

		// Try to parse as POM
		project, err := gopom.Parse(path)
		if err != nil {
			// Not a valid POM, skip
			log.Debugf("Not a valid POM (skipping): %s", path)
			return nil
		}

		pomFilesChecked++
		log.Debugf("Checking POM file %d: %s", pomFilesChecked, path)

		// Extract properties if they exist
		pomProperties := extractPropertiesFromProject(project)
		for k, v := range pomProperties {
			if _, exists := properties[k]; !exists {
				properties[k] = v
				relPath, _ := filepath.Rel(projectRoot, path)
				log.Infof("Found property %s = %s in %s", k, v, relPath)
			}
		}

		return nil
	})

	if err != nil {
		log.Warnf("Error walking project tree: %v", err)
	}

	log.Infof("Property search complete: checked %d POM files, skipped %d, found %d unique properties",
		pomFilesChecked, pomFilesSkipped, len(properties))

	if log.Enabled(context.Background(), slog.LevelDebug) {
		log.Debugf("Properties found: %v", properties)
	}

	return properties
}

// findProjectRoot finds the root of the Maven project by looking for the topmost pom.xml
func findProjectRoot(startDir string) string {
	current := startDir
	projectRoot := startDir
	levels := 0

	// Go up the directory tree looking for pom.xml files
	for {
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}

		parentPom := filepath.Join(parent, "pom.xml")
		if _, err := os.Stat(parentPom); err == nil {
			// Found a pom.xml in parent, this might be the project root
			projectRoot = parent
			current = parent
			levels++
		} else {
			// No pom.xml in parent, we've found the project root
			break
		}
	}

	if levels > 0 {
		// Only log if we actually traversed up
		clog.FromContext(context.Background()).Debugf("Found project root %d levels up from %s: %s",
			levels, startDir, projectRoot)
	}

	return projectRoot
}

// FindPropertyLocation searches for where a specific property is defined in the project
func FindPropertyLocation(ctx context.Context, startDir string, propertyName string) (string, string, error) {
	log := clog.FromContext(ctx)

	projectRoot := findProjectRoot(startDir)
	log.Debugf("Searching for property %s starting from project root: %s", propertyName, projectRoot)

	var foundPath string
	var foundValue string
	pomFilesChecked := 0

	// Recursively search the entire project
	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || foundPath != "" {
			return nil
		}

		// Skip hidden directories and common non-source directories
		if info.IsDir() {
			if isSkippableDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process XML files (POMs can have any name)
		if !strings.HasSuffix(info.Name(), ".xml") {
			return nil
		}

		project, err := gopom.Parse(path)
		if err != nil {
			return nil
		}

		pomFilesChecked++

		pomProperties := extractPropertiesFromProject(project)
		if value, exists := pomProperties[propertyName]; exists {
			foundPath = path
			foundValue = value
			relPath, _ := filepath.Rel(projectRoot, path)
			log.Infof("Found property %s = %s in %s", propertyName, value, relPath)
			return filepath.SkipDir // Stop searching
		}

		return nil
	})

	if err != nil {
		log.Debugf("Error searching for property: %v", err)
	}

	if foundPath != "" {
		return foundPath, foundValue, nil
	}

	// Property not found in project
	log.Warnf("Property '%s' not found after searching %d POM files in project", propertyName, pomFilesChecked)
	log.Warnf("This property may be defined in an external parent POM or imported from a dependency")

	return "", "", fmt.Errorf("property '%s' not found in project (searched %d POM files); it may be defined in an external parent POM", propertyName, pomFilesChecked)
}

// mergeProperties merges source properties into target, logging new additions
func mergeProperties(ctx context.Context, target, source map[string]string, sourceDesc string) {
	log := clog.FromContext(ctx)
	for k, v := range source {
		if _, exists := target[k]; !exists {
			target[k] = v
			log.Infof("Found property %s = %s in %s", k, v, sourceDesc)
		}
	}
}

// isSkippableDirectory checks if a directory should be skipped during traversal
func isSkippableDirectory(name string) bool {
	return strings.HasPrefix(name, ".") ||
		name == "target" ||
		name == "node_modules" ||
		name == "build" ||
		name == "dist" ||
		name == "out"
}

// extractPropertiesFromProject extracts properties from a parsed POM project
func extractPropertiesFromProject(project *gopom.Project) map[string]string {
	properties := make(map[string]string)
	if project != nil && project.Properties != nil && project.Properties.Entries != nil {
		for k, v := range project.Properties.Entries {
			properties[k] = v
		}
	}
	return properties
}

// isBOMImport checks if a dependency is being used as a BOM import
func isBOMImport(dep gopom.Dependency) bool {
	return dep.Scope == "import" && dep.Type == "pom"
}

// AnalyzeBOMs returns detailed information about imported BOMs
func AnalyzeBOMs(ctx context.Context, project *gopom.Project) []BOMInfo {
	log := clog.FromContext(ctx)
	var boms []BOMInfo

	if project.DependencyManagement != nil && project.DependencyManagement.Dependencies != nil {
		for _, dep := range *project.DependencyManagement.Dependencies {
			if isBOMImport(dep) {
				boms = append(boms, BOMInfo{
					GroupID:    dep.GroupID,
					ArtifactID: dep.ArtifactID,
					Version:    dep.Version,
					Type:       dep.Type,
					Scope:      dep.Scope,
				})
				log.Debugf("Found BOM: %s:%s:%s", dep.GroupID, dep.ArtifactID, dep.Version)
			}
		}
	}

	return boms
}

// ToAnalysisOutput converts AnalysisResult to the structured output format
func (result *AnalysisResult) ToAnalysisOutput(pomPath string, patches []Patch, propertyPatches map[string]string) *AnalysisOutput {
	output := &AnalysisOutput{
		POMFile:   pomPath,
		Timestamp: time.Now(),
		Dependencies: DependencyAnalysis{
			Total:           len(result.Dependencies),
			Direct:          0, // Will be calculated
			UsingProperties: countPropertiesUsage(result),
			FromBOMs:        0, // Would need additional tracking
			Transitive:      len(result.TransitiveDependencies),
		},
		Properties: PropertyAnalysis{
			Defined: result.Properties,
			UsedBy:  make(map[string][]string),
		},
		BOMs:            result.BOMs,
		Patches:         patches,
		PropertyUpdates: propertyPatches,
	}

	// Build the UsedBy map for properties
	for depKey, dep := range result.Dependencies {
		if dep.UsesProperty && dep.PropertyName != "" {
			if output.Properties.UsedBy[dep.PropertyName] == nil {
				output.Properties.UsedBy[dep.PropertyName] = []string{}
			}
			output.Properties.UsedBy[dep.PropertyName] = append(
				output.Properties.UsedBy[dep.PropertyName],
				depKey,
			)
		}
	}

	// Count direct dependencies (simple heuristic: those without property references could be direct)
	for _, dep := range result.Dependencies {
		if !dep.UsesProperty {
			output.Dependencies.Direct++
		}
	}

	return output
}

// detectVersionConflicts identifies version inconsistencies in patches that could be resolved with BOM updates
func detectVersionConflicts(ctx context.Context, result *AnalysisResult, patches []Patch) []VersionConflict {
	log := clog.FromContext(ctx)
	conflicts := []VersionConflict{}

	// Group patches by groupId
	patchGroups := make(map[string]map[string]string) // groupId -> artifactId -> version

	for _, patch := range patches {
		if patchGroups[patch.GroupID] == nil {
			patchGroups[patch.GroupID] = make(map[string]string)
		}
		patchGroups[patch.GroupID][patch.ArtifactID] = patch.Version
	}

	// Look for groups with multiple different versions
	for groupID, artifacts := range patchGroups {
		if len(artifacts) < 2 {
			continue // Need at least 2 artifacts to have a conflict
		}

		// Check if all versions are the same
		versions := make(map[string]bool)
		for _, version := range artifacts {
			versions[version] = true
		}

		if len(versions) > 1 {
			// We have a version conflict - check if there's a BOM for this group
			bomCandidate := findBOMForGroup(result, groupID)

			conflict := VersionConflict{
				GroupID:           groupID,
				RequestedVersions: artifacts,
				RecommendedAction: "update_bom",
				BOMCandidate:      bomCandidate,
			}

			if bomCandidate == nil {
				// No BOM found, manual resolution needed
				conflict.RecommendedAction = "resolve_manually"
				log.Warnf("Version conflict detected for %s but no BOM found - manual resolution required", groupID)
			} else {
				log.Infof("Version conflict detected for %s - recommend updating BOM %s:%s instead of individual patches",
					groupID, bomCandidate.GroupID, bomCandidate.ArtifactID)
			}

			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// findBOMForGroup looks for a BOM that manages dependencies for the given groupId
func findBOMForGroup(result *AnalysisResult, groupID string) *BOMInfo {
	// Look for BOMs that match the group or are commonly known BOMs for this group
	for _, bom := range result.BOMs {
		// Direct match (e.g., io.netty:netty-bom for io.netty dependencies)
		if bom.GroupID == groupID {
			return &bom
		}

		// Common BOM patterns
		if groupID == "io.netty" && bom.ArtifactID == "netty-bom" {
			return &bom
		}
		if groupID == "org.springframework" && (bom.ArtifactID == "spring-bom" || bom.ArtifactID == "spring-framework-bom") {
			return &bom
		}
		// Add more common BOM patterns as needed
	}

	return nil
}

// calculateOptimalBOMVersion determines the best BOM version based on requested patch versions
func calculateOptimalBOMVersion(requestedVersions map[string]string) string {
	// For simplicity, use the highest version among the requested versions
	// In practice, this could be more sophisticated (semantic versioning comparison)
	highestVersion := ""

	for _, version := range requestedVersions {
		if highestVersion == "" || version > highestVersion {
			highestVersion = version
		}
	}

	return highestVersion
}
