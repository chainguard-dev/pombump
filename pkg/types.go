package pkg

import (
	"time"
)

// AnalysisOutput represents the complete output structure for both analyze and plan commands
type AnalysisOutput struct {
	// Metadata
	POMFile   string    `json:"pom_file" yaml:"pom_file"`
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`

	// Analysis results
	Dependencies DependencyAnalysis `json:"dependencies" yaml:"dependencies"`
	Properties   PropertyAnalysis   `json:"properties" yaml:"properties"`
	BOMs         []BOMInfo          `json:"boms,omitempty" yaml:"boms,omitempty"`
	Issues       []Issue            `json:"issues,omitempty" yaml:"issues,omitempty"`

	// Patch recommendations
	Patches         []Patch           `json:"patches,omitempty" yaml:"patches,omitempty"`
	PropertyUpdates map[string]string `json:"property_updates,omitempty" yaml:"property_updates,omitempty"`

	// Actions needed
	Warnings  []string         `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	CannotFix []UnfixableIssue `json:"cannot_fix,omitempty" yaml:"cannot_fix,omitempty"`
}

// DependencyAnalysis contains dependency statistics
type DependencyAnalysis struct {
	Total           int `json:"total" yaml:"total"`
	Direct          int `json:"direct" yaml:"direct"`
	UsingProperties int `json:"using_properties" yaml:"using_properties"`
	FromBOMs        int `json:"from_boms" yaml:"from_boms"`
	Transitive      int `json:"transitive,omitempty" yaml:"transitive,omitempty"`
}

// PropertyAnalysis contains property information
type PropertyAnalysis struct {
	Defined map[string]string   `json:"defined" yaml:"defined"`
	UsedBy  map[string][]string `json:"used_by,omitempty" yaml:"used_by,omitempty"`
}

// BOMInfo represents an imported BOM
type BOMInfo struct {
	GroupID    string `json:"group_id" yaml:"group_id"`
	ArtifactID string `json:"artifact_id" yaml:"artifact_id"`
	Version    string `json:"version" yaml:"version"`
	Type       string `json:"type,omitempty" yaml:"type,omitempty"`
	Scope      string `json:"scope,omitempty" yaml:"scope,omitempty"`
}

// Issue represents a dependency issue (vulnerability, outdated version, etc.)
type Issue struct {
	Type            string   `json:"type" yaml:"type"` // "direct", "transitive", "shaded", "property"
	Dependency      string   `json:"dependency" yaml:"dependency"`
	CurrentVersion  string   `json:"current_version" yaml:"current_version"`
	RequiredVersion string   `json:"required_version,omitempty" yaml:"required_version,omitempty"`
	CVEs            []string `json:"cves,omitempty" yaml:"cves,omitempty"`
	Path            []string `json:"path,omitempty" yaml:"path,omitempty"`         // For transitive dependencies
	Property        string   `json:"property,omitempty" yaml:"property,omitempty"` // For property-based deps
}

// VersionConflict represents a version inconsistency in patches
type VersionConflict struct {
	GroupID           string
	RequestedVersions map[string]string // artifactId -> version
	RecommendedAction string            // "update_bom" or "resolve_manually"
	BOMCandidate      *BOMInfo          // Suggested BOM to update instead
}

// UnfixableIssue represents an issue that cannot be automatically fixed
type UnfixableIssue struct {
	Dependency string `json:"dependency" yaml:"dependency"`
	Reason     string `json:"reason" yaml:"reason"`
	Action     string `json:"action" yaml:"action"`
}

// TransitiveDependency represents a transitive dependency
type TransitiveDependency struct {
	GroupID    string   `json:"group_id" yaml:"group_id"`
	ArtifactID string   `json:"artifact_id" yaml:"artifact_id"`
	Version    string   `json:"version" yaml:"version"`
	Path       []string `json:"path" yaml:"path"` // Path from root to this dependency
	Depth      int      `json:"depth" yaml:"depth"`
}

// IsBOM returns true if this dependency is being used as a BOM import
func (b *BOMInfo) IsBOM() bool {
	return b.Scope == "import" && b.Type == "pom"
}
