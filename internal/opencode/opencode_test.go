package opencode

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateConfigCreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	err := UpdateConfig(configPath, "https://api.example.com/v1", "test-key")
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	// Verify structure
	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	provider, ok := config["provider"].(map[string]interface{})
	if !ok {
		t.Fatal("provider key not found or not a map")
	}

	runpod, ok := provider["runpod"].(map[string]interface{})
	if !ok {
		t.Fatal("runpod key not found or not a map")
	}

	options, ok := runpod["options"].(map[string]interface{})
	if !ok {
		t.Fatal("options key not found or not a map")
	}

	baseURL, ok := options["baseURL"].(string)
	if !ok || baseURL != "https://api.example.com/v1" {
		t.Errorf("baseURL mismatch: got %q, want %q", baseURL, "https://api.example.com/v1")
	}

	apiKey, ok := options["api_key"].(string)
	if !ok || apiKey != "test-key" {
		t.Errorf("api_key mismatch: got %q, want %q", apiKey, "test-key")
	}
}

func TestUpdateConfigUpdatesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create initial config with some other provider
	initialConfig := map[string]interface{}{
		"provider": map[string]interface{}{
			"runpod": map[string]interface{}{
				"options": map[string]interface{}{
					"baseURL": "https://old.example.com/v1",
					"api_key": "old-key",
				},
			},
			"other": map[string]interface{}{
				"setting": "value",
			},
		},
	}

	data, _ := json.MarshalIndent(initialConfig, "", "  ")
	os.WriteFile(configPath, data, 0o644)

	// Update the config
	err := UpdateConfig(configPath, "https://new.example.com/v1", "new-key")
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify file was updated
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	provider := config["provider"].(map[string]interface{})
	runpod := provider["runpod"].(map[string]interface{})
	options := runpod["options"].(map[string]interface{})

	baseURL, _ := options["baseURL"].(string)
	if baseURL != "https://new.example.com/v1" {
		t.Errorf("baseURL not updated: got %q", baseURL)
	}

	apiKey, _ := options["api_key"].(string)
	if apiKey != "new-key" {
		t.Errorf("api_key not updated: got %q", apiKey)
	}

	// Verify other providers preserved
	other, ok := provider["other"].(map[string]interface{})
	if !ok {
		t.Fatal("other provider not preserved")
	}

	setting, ok := other["setting"].(string)
	if !ok || setting != "value" {
		t.Errorf("other provider setting not preserved: got %q", setting)
	}
}


func TestUpdateConfigReturnsErrorForMissingParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a path with a non-existent parent directory
	configPath := filepath.Join(tmpDir, "nonexistent", "subdir", "config.json")

	err := UpdateConfig(configPath, "https://api.example.com/v1", "test-key")
	if err == nil {
		t.Fatal("Expected error for missing parent directory, got nil")
	}
}

func TestUpdateConfigWithJSONFormatting(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	err := UpdateConfig(configPath, "https://api.example.com/v1", "test-key")
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Read the file and verify it's valid JSON
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify indentation starts with 2 spaces (base level)
	if !bytes.Contains(data, []byte("\n  ")) {
		t.Error("Expected 2-space indentation in JSON output")
	}
}

func TestUpdateConfigAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create initial file
	err := UpdateConfig(configPath, "https://api.example.com/v1", "test-key")
	if err != nil {
		t.Fatalf("Initial UpdateConfig failed: %v", err)
	}

	// Read original content
	originalData, _ := os.ReadFile(configPath)

	// Update the config
	err = UpdateConfig(configPath, "https://new.example.com/v1", "new-key")
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify .tmp file doesn't exist (atomic write completed)
	tmpFile := configPath + ".tmp"
	_, err = os.Stat(tmpFile)
	if err == nil {
		t.Error("Temporary file still exists after atomic write")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Unexpected error checking temp file: %v", err)
	}

	// Verify new content is different
	newData, _ := os.ReadFile(configPath)
	if string(newData) == string(originalData) {
		t.Error("File content not updated")
	}
}

// bytes.Contains is not available in test scope, let's define a helper
func TestUpdateConfigCreatesNestedStructure(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	err := UpdateConfig(configPath, "https://api.example.com/v1", "test-key")
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify the exact structure
	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	json.Unmarshal(data, &config)

	// Verify all levels exist
	if _, ok := config["provider"]; !ok {
		t.Fatal("provider key not found")
	}

	provider := config["provider"].(map[string]interface{})
	if _, ok := provider["runpod"]; !ok {
		t.Fatal("runpod key not found under provider")
	}

	runpod := provider["runpod"].(map[string]interface{})
	if _, ok := runpod["options"]; !ok {
		t.Fatal("options key not found under provider.runpod")
	}
	options := runpod["options"].(map[string]interface{})
	if _, ok := options["baseURL"]; !ok {
		t.Fatal("baseURL key not found under provider.runpod.options")
	}
	if _, ok := options["api_key"]; !ok {
		t.Fatal("api_key key not found under provider.runpod.options")
	}
}

func TestUpdateConfigExpandsTilde(t *testing.T) {
	// Create a test file in the home directory using ~ expansion
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	testFileName := "test-opencode-config.json"
	tildePathWithFile := filepath.Join("~", testFileName)
	expandedPath := filepath.Join(homeDir, testFileName)

	// Clean up any existing test file
	defer os.Remove(expandedPath)

	// Call UpdateConfig with tilde path
	err = UpdateConfig(tildePathWithFile, "https://api.example.com/v1", "test-key-tilde")
	if err != nil {
		t.Fatalf("UpdateConfig with tilde path failed: %v", err)
	}

	// Verify file was created at the actual home directory location
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		t.Fatalf("Failed to read file at expanded path %q: %v", expandedPath, err)
	}

	// Verify JSON content is correct
	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	provider, ok := config["provider"].(map[string]interface{})
	if !ok {
		t.Fatal("provider key not found or not a map")
	}

	runpod, ok := provider["runpod"].(map[string]interface{})
	if !ok {
		t.Fatal("runpod key not found or not a map")
	}

	options, ok := runpod["options"].(map[string]interface{})
	if !ok {
		t.Fatal("options key not found or not a map")
	}

	baseURL, ok := options["baseURL"].(string)
	if !ok || baseURL != "https://api.example.com/v1" {
		t.Errorf("baseURL mismatch: got %q, want %q", baseURL, "https://api.example.com/v1")
	}

	apiKey, ok := options["api_key"].(string)
	if !ok || apiKey != "test-key-tilde" {
		t.Errorf("api_key mismatch: got %q, want %q", apiKey, "test-key-tilde")
	}
}


