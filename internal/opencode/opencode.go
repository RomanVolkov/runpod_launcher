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
			return err
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

	// Ensure runpod map exists
	runpod, ok := provider["runpod"].(map[string]interface{})
	if !ok {
		runpod = make(map[string]interface{})
		provider["runpod"] = runpod
	}

	// Set name and npm so OpenCode recognizes this as a valid provider
	if _, ok := runpod["name"]; !ok {
		runpod["name"] = "RunPod"
	}
	if _, ok := runpod["npm"]; !ok {
		runpod["npm"] = "@ai-sdk/openai-compatible"
	}

	// Ensure models map exists (required for OpenCode to recognize the provider)
	models, ok := runpod["models"].(map[string]interface{})
	if !ok {
		models = make(map[string]interface{})
		runpod["models"] = models
	}

	// Add the model if provided
	if modelName != "" {
		models[modelName] = map[string]interface{}{"name": modelName}
	}

	// Ensure options map exists
	options, ok := runpod["options"].(map[string]interface{})
	if !ok {
		options = make(map[string]interface{})
		runpod["options"] = options
	}

	// Set baseURL and apiKey (both under options)
	// Note: apiKey uses camelCase (not snake_case) per OpenCode provider spec
	options["baseURL"] = baseURL
	options["apiKey"] = apiKey

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
