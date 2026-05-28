package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// UpdateConfig reads the OpenCode JSON config file at path, updates the
// provider.runpod configuration (baseURL, api_key, and modelName), and writes
// it back atomically. If the file does not exist, it creates it with the
// provided values. The path may use ~ for the home directory, which is expanded.
// If the parent directory does not exist, an error is returned.
func UpdateConfig(path, baseURL, apiKey, modelName string) error {
	return UpdateConfigWithProvider(path, baseURL, apiKey, modelName, "runpod")
}

// UpdateConfigWithProvider is like UpdateConfig but allows specifying the provider name.
// This enables updating either the "runpod" provider (for pods) or "runpod-serverless"
// provider (for serverless endpoints) in the same config file.
func UpdateConfigWithProvider(path, baseURL, apiKey, modelName, providerName string) error {
	return updateConfigWithProvider(path, baseURL, apiKey, modelName, providerName, "")
}

// UpdateConfigWithProviderAndEnv is like UpdateConfigWithProvider but uses an environment
// variable reference for the API key instead of the literal key. This keeps sensitive
// credentials out of the config file while still allowing it to be committed to version control.
// The apiKeyEnvVar should be the environment variable name (e.g. "RUNPOD_API_KEY").
func UpdateConfigWithProviderAndEnv(path, baseURL, modelName, providerName, apiKeyEnvVar string) error {
	return updateConfigWithProvider(path, baseURL, "", modelName, providerName, apiKeyEnvVar)
}

func updateConfigWithProvider(path, baseURL, apiKey, modelName, providerName, apiKeyEnvVar string) error {
	// Expand ~ to home directory
	var expandedPath string
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		expandedPath = filepath.Join(homeDir, path[1:])
	} else {
		expandedPath = path
	}

	// Check that parent directory exists
	parentDir := filepath.Dir(expandedPath)
	if _, err := os.Stat(parentDir); err != nil {
		return err
	}

	// Read existing file or start with empty map
	var config map[string]interface{}
	data, err := os.ReadFile(expandedPath)
	if err == nil {
		err = json.Unmarshal(data, &config)
		if err != nil {
			// File exists but is invalid JSON — start fresh to avoid failing the entire operation
			config = make(map[string]interface{})
		}
	} else if os.IsNotExist(err) {
		config = make(map[string]interface{})
	} else {
		return err
	}

	// Ensure provider map exists
	provider, ok := config["provider"].(map[string]interface{})
	if !ok {
		provider = make(map[string]interface{})
		config["provider"] = provider
	}

	// Ensure provider config exists
	providerCfg, ok := provider[providerName].(map[string]interface{})
	if !ok {
		providerCfg = make(map[string]interface{})
		provider[providerName] = providerCfg
	}

	// Set name and npm so OpenCode recognizes this as a valid provider
	if _, ok := providerCfg["name"]; !ok {
		providerCfg["name"] = providerDisplayName(providerName)
	}
	if _, ok := providerCfg["npm"]; !ok {
		providerCfg["npm"] = "@ai-sdk/openai-compatible"
	}

	// Ensure models map exists (required for OpenCode to recognize the provider)
	models, ok := providerCfg["models"].(map[string]interface{})
	if !ok {
		models = make(map[string]interface{})
		providerCfg["models"] = models
	}

	// Add the model if provided
	if modelName != "" {
		models[modelName] = map[string]interface{}{"name": modelName}
	}

	// Ensure options map exists
	options, ok := providerCfg["options"].(map[string]interface{})
	if !ok {
		options = make(map[string]interface{})
		providerCfg["options"] = options
	}

	// Set baseURL and apiKey (both under options)
	// Note: apiKey uses camelCase (not snake_case) per OpenCode provider spec
	options["baseURL"] = baseURL

	// Use either the literal API key or an environment variable reference
	if apiKeyEnvVar != "" {
		options["apiKey"] = "{env:" + apiKeyEnvVar + "}"
	} else {
		options["apiKey"] = apiKey
	}

	// Marshal with indent
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically via temp file + rename
	tmpFile := expandedPath + ".tmp"
	err = os.WriteFile(tmpFile, jsonData, 0o644)
	if err != nil {
		return err
	}

	err = os.Rename(tmpFile, expandedPath)
	if err != nil {
		os.Remove(tmpFile)
		return err
	}

	return nil
}

func providerDisplayName(providerName string) string {
	switch providerName {
	case "runpod-serverless":
		return "RunPod Serverless"
	case "runpod":
		return "RunPod"
	default:
		return providerName
	}
}

// WriteEnvVar writes or updates an environment variable in the ~/.env file.
// If the variable already exists, its value is updated. Otherwise, it's appended.
func WriteEnvVar(envVarName, envVarValue string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	envPath := filepath.Join(homeDir, ".env")

	// Read existing content
	content, err := os.ReadFile(envPath)
	var lines []string
	if err == nil {
		lines = parseEnvLines(string(content))
	} else if !os.IsNotExist(err) {
		return err
	}

	// Update or add the variable
	found := false
	for i, line := range lines {
		if parseEnvVarName(line) == envVarName {
			lines[i] = envVarName + "=" + envVarValue
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, envVarName+"="+envVarValue)
	}

	// Write back
	newContent := ""
	for _, line := range lines {
		if line != "" {
			newContent += line + "\n"
		}
	}

	return os.WriteFile(envPath, []byte(newContent), 0o600)
}

// parseEnvLines splits content into individual lines, preserving comments and blank lines
func parseEnvLines(content string) []string {
	var lines []string
	for _, line := range splitLines(content) {
		lines = append(lines, line)
	}
	return lines
}

// parseEnvVarName extracts the variable name from a line like "VAR_NAME=value"
func parseEnvVarName(line string) string {
	trimmed := line
	// Skip comments and blank lines
	if trimmed == "" || trimmed[0] == '#' {
		return ""
	}
	// Find the equals sign
	for i, ch := range trimmed {
		if ch == '=' {
			return trimmed[:i]
		}
	}
	return ""
}

// splitLines splits content by newlines, preserving blank lines
func splitLines(content string) []string {
	var lines []string
	var current string
	for _, ch := range content {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
