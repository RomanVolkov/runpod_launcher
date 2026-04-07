package startup

import (
	"strings"
	"testing"
)

func TestBuildStartupScript_DoesNotContainCloudflared(t *testing.T) {
	script, err := BuildStartupScript("mistralai/Mistral-7B-Instruct-v0.2", "test-key", 8000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(script, "cloudflared") {
		t.Errorf("expected script NOT to contain 'cloudflared', got:\n%s", script)
	}
}

func TestBuildStartupScript_ContainsAPIKey(t *testing.T) {
	script, err := BuildStartupScript("mistralai/Mistral-7B-Instruct-v0.2", "my-secret-key", 8000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, "my-secret-key") {
		t.Errorf("expected script to contain API key, got:\n%s", script)
	}
}

func TestBuildStartupScript_ContainsModelName(t *testing.T) {
	script, err := BuildStartupScript("mistralai/Mistral-7B-Instruct-v0.2", "test-key", 8000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The model name is embedded in the script as a literal value
	if !strings.Contains(script, "mistralai/Mistral-7B-Instruct-v0.2") {
		t.Errorf("expected script to contain model name, got:\n%s", script)
	}
}

func TestBuildStartupScript_ContainsHostAndPort(t *testing.T) {
	script, err := BuildStartupScript("mistralai/Mistral-7B-Instruct-v0.2", "test-key", 8000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, "--host") || !strings.Contains(script, "--port") {
		t.Errorf("expected script to contain --host and --port, got:\n%s", script)
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
		_, err := BuildStartupScript(name, "test-key", 8000, 0)
		if err == nil {
			t.Errorf("expected error for model name %q, but got none", name)
		}
	}
}

func TestBuildStartupScript_ValidModelNames(t *testing.T) {
	cases := []string{
		"mistralai/Mistral-7B-Instruct-v0.2",
		"meta-llama/Llama-2-13b-chat-hf",
		"simple_model",
		"model.v2",
		"A/B/C",
	}
	for _, name := range cases {
		script, err := BuildStartupScript(name, "test-key", 8000, 0)
		if err != nil {
			t.Errorf("unexpected error for model name %q: %v", name, err)
			continue
		}
		// The model name is embedded in the script as a literal value
		if !strings.Contains(script, name) {
			t.Errorf("expected script to contain model name %q, got:\n%s", name, script)
		}
	}
}

func TestBuildStartupScript_ServicePort(t *testing.T) {
	script, err := BuildStartupScript("some/model", "test-key", 9999, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, "9999") {
		t.Errorf("expected script to contain port 9999, got:\n%s", script)
	}
}

func TestBuildStartupScript_InvalidServicePort_ReturnsError(t *testing.T) {
	invalidPorts := []int{0, -1, 65536, -1000, 100000}
	for _, port := range invalidPorts {
		_, err := BuildStartupScript("some/model", "test-key", port, 0)
		if err == nil {
			t.Errorf("expected error for servicePort %d, but got none", port)
		}
	}
}

func TestBuildStartupScript_ValidServicePort_Succeeds(t *testing.T) {
	validPorts := []int{1, 8000, 65535}
	for _, port := range validPorts {
		_, err := BuildStartupScript("some/model", "test-key", port, 0)
		if err != nil {
			t.Errorf("unexpected error for servicePort %d: %v", port, err)
		}
	}
}
