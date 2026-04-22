package startup

import (
	"testing"
)

func TestBuildStartupScript_ValidModelNames(t *testing.T) {
	cases := []string{
		"mistralai/Mistral-7B-Instruct-v0.2",
		"meta-llama/Llama-2-13b-chat-hf",
		"simple_model",
		"model.v2",
		"A/B/C",
		"gemma:4",     // Ollama model with tag
		"mistral:7b",  // Ollama model with tag
	}
	for _, name := range cases {
		script, err := BuildStartupScript(name, "test-key", 8000, 0, "")
		if err != nil {
			t.Errorf("unexpected error for model name %q: %v", name, err)
			continue
		}
		// Ollama just returns "serve" - no model name embedded
		if script != "serve" {
			t.Errorf("expected script to be 'serve' for Ollama, got:\n%s", script)
		}
	}
}

func TestBuildStartupScript_InvalidModelName_ReturnsError(t *testing.T) {
	cases := []string{
		"; rm -rf /",
		"model$(whoami)",
		"model`id`",
		"model name with spaces",
		"model&bad",
		"model|pipe",
		"model>redirect",
	}
	for _, name := range cases {
		_, err := BuildStartupScript(name, "test-key", 8000, 0, "")
		if err == nil {
			t.Errorf("expected error for model name %q, but got none", name)
		}
	}
}

func TestBuildStartupScript_InvalidServicePort_ReturnsError(t *testing.T) {
	invalidPorts := []int{0, -1, 65536, -1000, 100000}
	for _, port := range invalidPorts {
		_, err := BuildStartupScript("gemma:4", "test-key", port, 0, "")
		if err == nil {
			t.Errorf("expected error for servicePort %d, but got none", port)
		}
	}
}

func TestBuildStartupScript_ValidServicePort_Succeeds(t *testing.T) {
	validPorts := []int{1, 8000, 65535}
	for _, port := range validPorts {
		_, err := BuildStartupScript("gemma:4", "test-key", port, 0, "")
		if err != nil {
			t.Errorf("unexpected error for servicePort %d: %v", port, err)
		}
	}
}
