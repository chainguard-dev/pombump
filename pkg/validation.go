package pkg

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// Maximum lengths for various fields
	maxGroupIDLength    = 256
	maxArtifactIDLength = 256
	maxVersionLength    = 128
	maxPropertyLength   = 256
	maxValueLength      = 1024
)

var (
	// Valid characters for Maven coordinates
	groupIDArtifactIDRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	// Version can have more varied formats including Maven ranges
	versionRegex = regexp.MustCompile(`^[a-zA-Z0-9._\-+\[\],!()]+$`)
	// Property names should be safe
	propertyNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)

// ValidatePatch ensures the patch contains safe and valid values
func ValidatePatch(patch Patch) error {
	// Validate GroupID
	if len(patch.GroupID) == 0 {
		return fmt.Errorf("groupId cannot be empty")
	}
	if len(patch.GroupID) > maxGroupIDLength {
		return fmt.Errorf("groupId too long: %d characters (max: %d)", len(patch.GroupID), maxGroupIDLength)
	}
	if !groupIDArtifactIDRegex.MatchString(patch.GroupID) {
		return fmt.Errorf("groupId contains invalid characters: %s", patch.GroupID)
	}

	// Validate ArtifactID
	if len(patch.ArtifactID) == 0 {
		return fmt.Errorf("artifactId cannot be empty")
	}
	if len(patch.ArtifactID) > maxArtifactIDLength {
		return fmt.Errorf("artifactId too long: %d characters (max: %d)", len(patch.ArtifactID), maxArtifactIDLength)
	}
	if !groupIDArtifactIDRegex.MatchString(patch.ArtifactID) {
		return fmt.Errorf("artifactId contains invalid characters: %s", patch.ArtifactID)
	}

	// Validate Version
	if len(patch.Version) == 0 {
		return fmt.Errorf("version cannot be empty")
	}
	if len(patch.Version) > maxVersionLength {
		return fmt.Errorf("version too long: %d characters (max: %d)", len(patch.Version), maxVersionLength)
	}
	if !versionRegex.MatchString(patch.Version) {
		return fmt.Errorf("version contains invalid characters: %s", patch.Version)
	}

	// Validate Scope if present
	if patch.Scope != "" {
		validScopes := map[string]bool{
			"compile":  true,
			"provided": true,
			"runtime":  true,
			"test":     true,
			"system":   true,
			"import":   true,
		}
		if !validScopes[patch.Scope] {
			return fmt.Errorf("invalid scope: %s", patch.Scope)
		}
	}

	// Validate Type if present
	if patch.Type != "" {
		// Check if type is alphanumeric with dots, dashes, and underscores
		typeRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
		if !typeRegex.MatchString(patch.Type) {
			return fmt.Errorf("type contains invalid characters: %s", patch.Type)
		}
		// Allow custom types but validate they're safe
		if len(patch.Type) > 64 {
			return fmt.Errorf("type too long: %d characters (max: 64)", len(patch.Type))
		}
	}

	return nil
}

// ValidatePropertyPatch ensures property names and values are safe
func ValidatePropertyPatch(property, value string) error {
	// Validate property name
	if len(property) == 0 {
		return fmt.Errorf("property name cannot be empty")
	}
	if len(property) > maxPropertyLength {
		return fmt.Errorf("property name too long: %d characters (max: %d)", len(property), maxPropertyLength)
	}
	if !propertyNameRegex.MatchString(property) {
		return fmt.Errorf("property name contains invalid characters: %s", property)
	}

	// Validate value
	if len(value) == 0 {
		return fmt.Errorf("property value cannot be empty")
	}
	if len(value) > maxValueLength {
		return fmt.Errorf("property value too long: %d characters (max: %d)", len(value), maxValueLength)
	}
	
	// Check for potentially dangerous XML content
	if containsXMLInjection(value) {
		return fmt.Errorf("property value contains potentially dangerous XML characters")
	}

	return nil
}

// containsXMLInjection checks for potentially dangerous XML content
func containsXMLInjection(s string) bool {
	dangerous := []string{
		"<!ENTITY",
		"<!DOCTYPE",
		"<![CDATA[",
		"<?xml",
		"<script",
		"javascript:",
		"onerror=",
		"onclick=",
	}
	
	lower := strings.ToLower(s)
	for _, d := range dangerous {
		if strings.Contains(lower, strings.ToLower(d)) {
			return true
		}
	}
	
	// Check for any XML tags (opening or closing)
	if strings.Contains(s, "<") && strings.Contains(s, ">") {
		return true
	}
	
	return false
}

// ValidateFilePath performs basic path validation
func ValidateFilePath(path string) error {
	// Check for empty path before trimming
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	
	// Clean the path
	clean := strings.TrimSpace(path)
	
	// Check if path becomes empty after trimming
	if clean == "" {
		return fmt.Errorf("file path cannot be empty or whitespace only")
	}
	
	// Check for path traversal patterns
	if strings.Contains(clean, "..") {
		return fmt.Errorf("invalid path pattern: %s", path)
	}
	
	// Check for null bytes
	if strings.Contains(clean, "\x00") {
		return fmt.Errorf("null byte in path")
	}
	
	return nil
}
