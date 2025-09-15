package pkg

import (
	"strings"
	"testing"
)

// FuzzValidatePatch tests ValidatePatch with random inputs
func FuzzValidatePatch(f *testing.F) {
	// Add seed corpus - valid and invalid examples
	f.Add("com.example", "my-artifact", "1.0.0", "compile", "jar")
	f.Add("org.springframework.boot", "spring-boot-starter", "2.7.0", "import", "pom")
	f.Add("", "", "", "", "")
	f.Add("com.example;rm -rf /", "artifact", "1.0", "test", "jar")
	f.Add("a", "b", "c", "d", "e")
	f.Add("com.example", "artifact", "1.0.0<script>alert('xss')</script>", "compile", "jar")
	
	f.Fuzz(func(t *testing.T, groupID, artifactID, version, scope, patchType string) {
		patch := Patch{
			GroupID:    groupID,
			ArtifactID: artifactID,
			Version:    version,
			Scope:      scope,
			Type:       patchType,
		}
		
		// The function should not panic on any input
		err := ValidatePatch(patch)
		
		// If no error, verify the patch meets basic requirements
		if err == nil {
			// These should never be empty if validation passed
			if groupID == "" || artifactID == "" || version == "" {
				t.Errorf("ValidatePatch allowed empty required field: groupID=%q, artifactID=%q, version=%q", groupID, artifactID, version)
			}
			
			// Check scope is valid if present
			if scope != "" {
				validScopes := map[string]bool{
					"compile": true, "provided": true, "runtime": true,
					"test": true, "system": true, "import": true,
				}
				if !validScopes[scope] {
					t.Errorf("ValidatePatch allowed invalid scope: %q", scope)
				}
			}
			
			// Check length limits
			if len(groupID) > 256 || len(artifactID) > 256 || len(version) > 128 {
				t.Error("ValidatePatch allowed values exceeding length limits")
			}
			
			// Type length limit
			if patchType != "" && len(patchType) > 64 {
				t.Errorf("ValidatePatch allowed type exceeding length limit: %d chars", len(patchType))
			}
		}
	})
}

// FuzzValidatePropertyPatch tests ValidatePropertyPatch with random inputs
func FuzzValidatePropertyPatch(f *testing.F) {
	// Add seed corpus
	f.Add("spring.version", "5.3.23")
	f.Add("", "")
	f.Add("prop", "value")
	f.Add("my.property", "my value with spaces")
	f.Add("prop", "<script>alert('xss')</script>")
	f.Add("prop", "value\n<malicious>tag</malicious>")
	f.Add("very.long.property.name", "very long value")
	
	f.Fuzz(func(t *testing.T, property, value string) {
		// The function should not panic on any input
		err := ValidatePropertyPatch(property, value)
		
		// If no error, verify the values meet requirements
		if err == nil {
			if property == "" || value == "" {
				t.Errorf("ValidatePropertyPatch allowed empty values: property=%q, value=%q", property, value)
			}
			
			if len(property) > 256 || len(value) > 1024 {
				t.Error("ValidatePropertyPatch allowed values exceeding length limits")
			}
			
			// Check for XML injection patterns that should be rejected
			if containsXMLInjection(value) {
				t.Errorf("ValidatePropertyPatch allowed value with XML injection: %q", value)
			}
		}
	})
}

// FuzzValidateFilePath tests ValidateFilePath with random inputs
func FuzzValidateFilePath(f *testing.F) {
	// Add seed corpus
	f.Add("test.xml")
	f.Add("/absolute/path/file.xml")
	f.Add("relative/path/file.xml")
	f.Add("")
	f.Add("../../../etc/passwd")
	f.Add("file\x00.xml")
	f.Add("   ")
	f.Add("./file.xml")
	f.Add("file/../../../etc/passwd")
	
	f.Fuzz(func(t *testing.T, path string) {
		// The function should not panic on any input
		err := ValidateFilePath(path)
		
		// If no error, verify the path is safe
		if err == nil {
			trimmed := strings.TrimSpace(path)
			if trimmed == "" {
				t.Errorf("ValidateFilePath allowed empty or whitespace-only path: %q", path)
			}
			
			if strings.Contains(path, "..") {
				t.Errorf("ValidateFilePath allowed path traversal: %q", path)
			}
			
			if strings.Contains(path, "\x00") {
				t.Errorf("ValidateFilePath allowed null byte: %q", path)
			}
		}
	})
}

// FuzzContainsXMLInjection tests the XML injection detection
func FuzzContainsXMLInjection(f *testing.F) {
	// Add seed corpus - both safe and unsafe strings
	f.Add("normal text")
	f.Add("<!ENTITY xxe SYSTEM \"file:///etc/passwd\">")
	f.Add("<script>alert('xss')</script>")
	f.Add("text with < and > symbols")
	f.Add("CDATA[[test]]")
	f.Add("javascript:alert(1)")
	f.Add("")
	f.Add("<?xml version=\"1.0\"?>")
	
	f.Fuzz(func(t *testing.T, input string) {
		// The function should not panic on any input
		result := containsXMLInjection(input)
		
		// Verify detection logic
		if result {
			// If detected as injection, it should contain dangerous patterns
			lower := strings.ToLower(input)
			hasReason := false
			
			// Check for known dangerous patterns
			dangerous := []string{
				"<!entity", "<!doctype", "<![cdata[", "<?xml",
				"<script", "javascript:", "onerror=", "onclick=",
			}
			
			for _, pattern := range dangerous {
				if strings.Contains(lower, pattern) {
					hasReason = true
					break
				}
			}
			
			// Or has both < and >
			if strings.Contains(input, "<") && strings.Contains(input, ">") {
				hasReason = true
			}
			
			if !hasReason {
				t.Errorf("containsXMLInjection returned true without dangerous pattern: %q", input)
			}
		}
	})
}
