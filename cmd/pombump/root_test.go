package pombump

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		name        string
		logLevel    string
		expectError bool
		expectedMsg string
	}{
		{
			name:        "valid debug level",
			logLevel:    "debug",
			expectError: false,
		},
		{
			name:        "valid info level",
			logLevel:    "info",
			expectError: false,
		},
		{
			name:        "valid warn level",
			logLevel:    "warn",
			expectError: false,
		},
		{
			name:        "valid error level",
			logLevel:    "error",
			expectError: false,
		},
		{
			name:        "invalid log level defaults to info",
			logLevel:    "invalid",
			expectError: false, // Implementation defaults to info for invalid levels
		},
		{
			name:        "empty log level",
			logLevel:    "",
			expectError: false, // defaults to info
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original logger to restore after test
			originalLogger := slog.Default()
			defer slog.SetDefault(originalLogger)

			cmd := New()
			
			// Parse flags manually to set up the test environment
			if tt.logLevel != "" {
				cmd.SetArgs([]string{"--log-level", tt.logLevel, "dummy.pom"})
				err := cmd.ParseFlags([]string{"--log-level", tt.logLevel})
				if err != nil {
					t.Fatalf("Failed to parse flags: %v", err)
				}
			} else {
				cmd.SetArgs([]string{"dummy.pom"})
			}

			// Run only the PersistentPreRunE to test logging setup
			err := cmd.PersistentPreRunE(cmd, []string{"dummy.pom"})

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedMsg != "" && !strings.Contains(err.Error(), tt.expectedMsg) {
					t.Errorf("Expected error message '%s' but got '%s'", tt.expectedMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLogPolicyFileCreation(t *testing.T) {
	// Store original logger to restore after test
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	cmd := New()
	cmd.SetArgs([]string{"--log-policy", logFile, "--log-level", "info", "dummy.pom"})
	
	// Parse flags to set up the environment
	err := cmd.ParseFlags([]string{"--log-policy", logFile, "--log-level", "info"})
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Run PersistentPreRunE to test file creation
	err = cmd.PersistentPreRunE(cmd, []string{"dummy.pom"})
	if err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}

	// Check if file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", logFile)
	}

	// Test logging to the file
	slog.Info("test message")

	// Read file contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Errorf("Log message not found in file. Content: %s", string(content))
	}
}

func TestLogPolicyInvalidPath(t *testing.T) {
	// Store original logger to restore after test
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	cmd := New()
	cmd.SetArgs([]string{"--log-policy", "/invalid/path/that/cannot/exist.log", "dummy.pom"})
	
	// Parse flags
	err := cmd.ParseFlags([]string{"--log-policy", "/invalid/path/that/cannot/exist.log"})
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Run PersistentPreRunE - should fail
	err = cmd.PersistentPreRunE(cmd, []string{"dummy.pom"})
	if err == nil {
		t.Error("Expected error for invalid log file path but got none")
		return
	}

	if !strings.Contains(err.Error(), "failed to create log writer") {
		t.Errorf("Expected 'failed to create log writer' error but got: %v", err)
	}
}

func TestLogPolicyMultiplePolicies(t *testing.T) {
	// Store original logger to restore after test  
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "multi.log")

	cmd := New()
	cmd.SetArgs([]string{"--log-policy", "builtin:stderr", "--log-policy", logFile, "dummy.pom"})
	
	// Parse flags
	err := cmd.ParseFlags([]string{"--log-policy", "builtin:stderr", "--log-policy", logFile})
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Run PersistentPreRunE 
	err = cmd.PersistentPreRunE(cmd, []string{"dummy.pom"})
	if err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}

	// Should use the first non-stderr policy (the file)
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", logFile)
	}

	// Test logging to the file
	slog.Info("multi policy test")

	// Read file contents  
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "multi policy test") {
		t.Errorf("Log message not found in file. Content: %s", string(content))
	}
}

func TestDefaultLogPolicy(t *testing.T) {
	// Store original logger and stderr
	originalLogger := slog.Default()
	originalStderr := os.Stderr
	defer func() {
		slog.SetDefault(originalLogger)
		os.Stderr = originalStderr
	}()

	// Capture stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cmd := New()
	cmd.SetArgs([]string{"--log-level", "info", "dummy.pom"}) // No log-policy, should default to stderr

	// Run PersistentPreRunE
	err := cmd.PersistentPreRunE(cmd, []string{"dummy.pom"})
	if err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}

	// Log a test message
	slog.Info("default policy test")

	// Close write end and read from stderr
	_ = w.Close() // Ignore error in test cleanup
	output, _ := io.ReadAll(r)

	if !strings.Contains(string(output), "default policy test") {
		t.Errorf("Log message not found in stderr. Output: %s", string(output))
	}
}

func TestCommandFlags(t *testing.T) {
	cmd := New()

	// Test that flags are properly defined
	logLevelFlag := cmd.PersistentFlags().Lookup("log-level")
	if logLevelFlag == nil {
		t.Error("log-level flag not found")
	} else {
		if logLevelFlag.DefValue != "info" {
			t.Errorf("Expected log-level default 'info', got '%s'", logLevelFlag.DefValue)
		}
	}

	logPolicyFlag := cmd.PersistentFlags().Lookup("log-policy")
	if logPolicyFlag == nil {
		t.Error("log-policy flag not found")
	} else {
		// Should default to builtin:stderr
		if !strings.Contains(logPolicyFlag.DefValue, "builtin:stderr") {
			t.Errorf("Expected log-policy to contain 'builtin:stderr', got '%s'", logPolicyFlag.DefValue)
		}
	}
}

func TestLoggerIntegration(t *testing.T) {
	// Store original logger
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "integration.log")

	cmd := New()
	cmd.SetArgs([]string{"--log-policy", logFile, "--log-level", "debug", "dummy.pom"})
	
	// Parse flags
	err := cmd.ParseFlags([]string{"--log-policy", logFile, "--log-level", "debug"})
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Run PersistentPreRunE
	err = cmd.PersistentPreRunE(cmd, []string{"dummy.pom"})
	if err != nil {
		t.Fatalf("PersistentPreRunE failed: %v", err)
	}

	// Test different log levels
	slog.Debug("debug message")
	slog.Info("info message") 
	slog.Warn("warn message")
	slog.Error("error message")

	// Read file contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	
	// With debug level, all messages should appear
	messages := []string{"debug message", "info message", "warn message", "error message"}
	for _, msg := range messages {
		if !strings.Contains(contentStr, msg) {
			t.Errorf("Expected message '%s' not found in log. Content: %s", msg, contentStr)
		}
	}

	// Verify charmlog format (should have timestamp and level like "DEBU", "INFO", etc.)
	if !strings.Contains(contentStr, "DEBU") {
		t.Errorf("Expected charmlog format with 'DEBU' level. Content: %s", contentStr)
	}
	if !strings.Contains(contentStr, "2025/") { // Check for timestamp
		t.Errorf("Expected charmlog format with timestamp. Content: %s", contentStr)
	}
}

func TestMainCommandExecution(t *testing.T) {
	// Store original logger
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	cmd := New()
	
	// Test successful execution with dependencies flag
	cmd.SetArgs([]string{"../../testdata/simple.pom.xml", "--dependencies", "junit@junit@4.13.2"})
	
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}
}

func TestMainCommandValidation(t *testing.T) {
	cmd := New()
	
	// Test validation - no flags provided
	cmd.SetArgs([]string{"../../testdata/simple.pom.xml"})
	
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for missing dependencies/properties flags but got none")
	}
	
	if !strings.Contains(err.Error(), "no dependencies or properties provides") {
		t.Errorf("Expected validation error message but got: %v", err)
	}
}

func TestMainCommandMutuallyExclusive(t *testing.T) {
	cmd := New()
	
	// Test mutually exclusive flags - both dependencies and patch-file
	cmd.SetArgs([]string{"../../testdata/simple.pom.xml", "--dependencies", "test@test@1.0", "--patch-file", "patches.yaml"})
	
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for mutually exclusive flags but got none")
	}
	
	if !strings.Contains(err.Error(), "use either --dependencies or --patch-file") {
		t.Errorf("Expected mutually exclusive error message but got: %v", err)
	}
}

func TestMainCommandInvalidFile(t *testing.T) {
	cmd := New()
	
	// Test with non-existent POM file
	cmd.SetArgs([]string{"nonexistent.pom", "--dependencies", "test@test@1.0"})
	
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for non-existent file but got none")
	}
}

func TestMainCommandWithProperties(t *testing.T) {
	// Store original logger
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	cmd := New()
	
	// Test successful execution with properties flag
	cmd.SetArgs([]string{"../../testdata/complex.pom.xml", "--properties", "junit.version@4.13.2"})
	
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution with properties failed: %v", err)
	}
}

func TestMainCommandWithPropertiesFile(t *testing.T) {
	// Store original logger
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	// Create a temporary properties file
	tempDir := t.TempDir()
	propsFile := filepath.Join(tempDir, "props.yaml")
	propsContent := `properties:
  - property: junit.version
    value: 4.13.2`
	
	err := os.WriteFile(propsFile, []byte(propsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test properties file: %v", err)
	}

	cmd := New()
	cmd.SetArgs([]string{"../../testdata/complex.pom.xml", "--properties-file", propsFile})
	
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Command execution with properties file failed: %v", err)
	}
}

func TestMainCommandMutuallyExclusiveProperties(t *testing.T) {
	cmd := New()
	
	// Test mutually exclusive flags - both properties and properties-file
	cmd.SetArgs([]string{"../../testdata/simple.pom.xml", "--properties", "test@1.0", "--properties-file", "props.yaml"})
	
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for mutually exclusive properties flags but got none")
	}
	
	if !strings.Contains(err.Error(), "use either --properties or --properties-file") {
		t.Errorf("Expected mutually exclusive properties error message but got: %v", err)
	}
}