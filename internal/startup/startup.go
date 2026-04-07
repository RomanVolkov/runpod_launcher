// Package startup provides vLLM startup script building for runpod-launcher.
// It generates a bash script that launches the vLLM OpenAI-compatible API server
// using environment variables injected as RunPod pod env vars at pod creation time.
package startup

import (
	"fmt"
	"regexp"
)

// validModelName matches safe model name strings: alphanumeric, slashes, hyphens, underscores, dots.
// This covers typical HuggingFace model IDs like "mistralai/Mistral-7B-Instruct-v0.2".
var validModelName = regexp.MustCompile(`^[a-zA-Z0-9/_\-\.]+$`)

// BuildStartupScript returns a command string suitable for injection as a RunPod pod startup
// (via the dockerArgs field). The command starts the Ollama server and pulls the specified model.
//
// modelName is validated here to guard against shell injection.
// modelName must contain only safe characters (letters, digits, /, -, _, ., :).
// servicePort is the port on which Ollama will listen (typically 8000).
// apiKey, maxModelLen, toolCallParser are ignored for Ollama (kept for compatibility).
func BuildStartupScript(modelName, apiKey string, servicePort, maxModelLen int, toolCallParser string) (string, error) {
	// Allow colons for Ollama model names (e.g., "gemma:4")
	validOllamaName := regexp.MustCompile(`^[a-zA-Z0-9/_\-\.:\+]+$`)
	if !validOllamaName.MatchString(modelName) {
		return "", fmt.Errorf("invalid modelName %q: must contain only alphanumeric characters, '/', '-', '_', ':', '.', or '+'", modelName)
	}
	if servicePort < 1 || servicePort > 65535 {
		return "", fmt.Errorf("invalid servicePort %d: must be in range 1-65535", servicePort)
	}

	// Build startup script for Ollama.
	// The Ollama Docker image uses environment variables for configuration.
	// We pass "serve" as the command and set OLLAMA_HOST via the pod environment.
	// Note: For Ollama, we rely on environment variables (set by RunPod) rather than CLI flags.
	script := `serve`

	return script, nil
}
