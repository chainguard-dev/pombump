package pkg

import (
	"strings"
	"testing"
)

func TestValidatePatch(t *testing.T) {
	tests := []struct {
		name    string
		patch   Patch
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid patch",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "my-artifact",
				Version:    "1.0.0",
				Scope:      "compile",
				Type:       "jar",
			},
			wantErr: false,
		},
		{
			name: "valid patch with complex version",
			patch: Patch{
				GroupID:    "org.springframework.boot",
				ArtifactID: "spring-boot-starter",
				Version:    "[2.7.0,3.0.0)",
				Scope:      "import",
				Type:       "pom",
			},
			wantErr: false,
		},
		{
			name: "empty groupId",
			patch: Patch{
				GroupID:    "",
				ArtifactID: "artifact",
				Version:    "1.0",
			},
			wantErr: true,
			errMsg:  "groupId cannot be empty",
		},
		{
			name: "empty artifactId",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "",
				Version:    "1.0",
			},
			wantErr: true,
			errMsg:  "artifactId cannot be empty",
		},
		{
			name: "empty version",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "artifact",
				Version:    "",
			},
			wantErr: true,
			errMsg:  "version cannot be empty",
		},
		{
			name: "groupId with invalid characters",
			patch: Patch{
				GroupID:    "com.example;rm -rf /",
				ArtifactID: "artifact",
				Version:    "1.0",
			},
			wantErr: true,
			errMsg:  "groupId contains invalid characters",
		},
		{
			name: "artifactId with spaces",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "my artifact",
				Version:    "1.0",
			},
			wantErr: true,
			errMsg:  "artifactId contains invalid characters",
		},
		{
			name: "version with script tag",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "artifact",
				Version:    "1.0<script>alert('xss')</script>",
			},
			wantErr: true,
			errMsg:  "version contains invalid characters",
		},
		{
			name: "extremely long groupId",
			patch: Patch{
				GroupID:    strings.Repeat("a", 257),
				ArtifactID: "artifact",
				Version:    "1.0",
			},
			wantErr: true,
			errMsg:  "groupId too long",
		},
		{
			name: "invalid scope",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "artifact",
				Version:    "1.0",
				Scope:      "invalid-scope",
			},
			wantErr: true,
			errMsg:  "invalid scope",
		},
		{
			name: "valid custom type",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "artifact",
				Version:    "1.0",
				Type:       "custom-type",
			},
			wantErr: false, // We allow custom types that pass validation
		},
		{
			name: "type with invalid characters",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "artifact",
				Version:    "1.0",
				Type:       "jar<script>alert('xss')</script>",
			},
			wantErr: true,
			errMsg:  "type contains invalid characters",
		},
		{
			name: "type too long",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "artifact",
				Version:    "1.0",
				Type:       strings.Repeat("a", 65),
			},
			wantErr: true,
			errMsg:  "type too long",
		},
		{
			name: "type with spaces",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "artifact",
				Version:    "1.0",
				Type:       "my type",
			},
			wantErr: true,
			errMsg:  "type contains invalid characters",
		},
		{
			name: "known type test-jar",
			patch: Patch{
				GroupID:    "com.example",
				ArtifactID: "artifact",
				Version:    "1.0",
				Type:       "test-jar",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePatch(tt.patch)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePatch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidatePatch() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestValidatePropertyPatch(t *testing.T) {
	tests := []struct {
		name     string
		property string
		value    string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid property",
			property: "spring.version",
			value:    "5.3.23",
			wantErr:  false,
		},
		{
			name:     "valid complex property",
			property: "maven.compiler.source",
			value:    "11",
			wantErr:  false,
		},
		{
			name:     "empty property name",
			property: "",
			value:    "value",
			wantErr:  true,
			errMsg:   "property name cannot be empty",
		},
		{
			name:     "empty property value",
			property: "prop",
			value:    "",
			wantErr:  true,
			errMsg:   "property value cannot be empty",
		},
		{
			name:     "property with spaces",
			property: "my property",
			value:    "value",
			wantErr:  true,
			errMsg:   "property name contains invalid characters",
		},
		{
			name:     "property with special chars",
			property: "prop;echo pwned",
			value:    "value",
			wantErr:  true,
			errMsg:   "property name contains invalid characters",
		},
		{
			name:     "extremely long property name",
			property: strings.Repeat("a", 257),
			value:    "value",
			wantErr:  true,
			errMsg:   "property name too long",
		},
		{
			name:     "extremely long value",
			property: "prop",
			value:    strings.Repeat("a", 1025),
			wantErr:  true,
			errMsg:   "property value too long",
		},
		{
			name:     "value with XML entity",
			property: "prop",
			value:    "value<!ENTITY xxe SYSTEM 'file:///etc/passwd'>",
			wantErr:  true,
			errMsg:   "potentially dangerous XML",
		},
		{
			name:     "value with script tag",
			property: "prop",
			value:    "<script>alert('xss')</script>",
			wantErr:  true,
			errMsg:   "potentially dangerous XML",
		},
		{
			name:     "value with CDATA",
			property: "prop",
			value:    "<![CDATA[some data]]>",
			wantErr:  true,
			errMsg:   "potentially dangerous XML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePropertyPatch(tt.property, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePropertyPatch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidatePropertyPatch() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			path:    "pom.xml",
			wantErr: false,
		},
		{
			name:    "valid path with directory",
			path:    "src/main/pom.xml",
			wantErr: false,
		},
		{
			name:    "valid absolute path",
			path:    "/home/user/project/pom.xml",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  "file path cannot be empty",
		},
		{
			name:    "path with traversal",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "invalid path pattern",
		},
		{
			name:    "path with hidden traversal",
			path:    "test/../../etc/passwd",
			wantErr: true,
			errMsg:  "invalid path pattern",
		},
		{
			name:    "path with null byte",
			path:    "file.xml\x00.txt",
			wantErr: true,
			errMsg:  "null byte in path",
		},
		{
			name:    "whitespace only",
			path:    "   ",
			wantErr: true,
			errMsg:  "file path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateFilePath() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestContainsXMLInjection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "clean string",
			input:    "1.0.0-SNAPSHOT",
			expected: false,
		},
		{
			name:     "entity injection",
			input:    "<!ENTITY xxe SYSTEM 'file:///etc/passwd'>",
			expected: true,
		},
		{
			name:     "doctype injection",
			input:    "<!DOCTYPE foo [<!ENTITY xxe SYSTEM 'file:///etc/passwd'>]>",
			expected: true,
		},
		{
			name:     "script tag",
			input:    "<script>alert('xss')</script>",
			expected: true,
		},
		{
			name:     "javascript protocol",
			input:    "javascript:alert('xss')",
			expected: true,
		},
		{
			name:     "event handler",
			input:    "onerror=alert('xss')",
			expected: true,
		},
		{
			name:     "mixed case attempt",
			input:    "<!EnTiTy xxe SYSTEM 'file:///etc/passwd'>",
			expected: true,
		},
		{
			name:     "CDATA section",
			input:    "<![CDATA[some data]]>",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsXMLInjection(tt.input); got != tt.expected {
				t.Errorf("containsXMLInjection() = %v, want %v", got, tt.expected)
			}
		})
	}
}
