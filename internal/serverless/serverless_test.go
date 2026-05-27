package serverless_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/serverless"
)

func withMockCLI(t *testing.T, fn func(apiKey string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := serverless.RunpodCtlFn
	t.Cleanup(func() { serverless.RunpodCtlFn = orig })
	serverless.RunpodCtlFn = fn
}

func newTestClient() serverless.Client {
	return serverless.NewRunPodServerlessClient("test-api-key")
}

func TestFindEndpointByName_ReturnsMatch(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal([]map[string]interface{}{
			{"id": "ep-123", "name": "test-endpoint", "workersMin": 0, "workersMax": 3},
		})
		return out, nil
	})

	client := newTestClient()
	ep, err := client.FindEndpointByName("test-endpoint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep == nil {
		t.Fatal("expected endpoint, got nil")
	}
	if ep.ID != "ep-123" || ep.Name != "test-endpoint" {
		t.Errorf("endpoint mismatch: got %+v", ep)
	}
	if ep.WorkersMax != 3 {
		t.Errorf("workersMax: got %d, want 3", ep.WorkersMax)
	}
}

func TestFindEndpointByName_ReturnsNilWhenNotFound(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal([]map[string]interface{}{
			{"id": "ep-999", "name": "other-endpoint"},
		})
		return out, nil
	})

	client := newTestClient()
	ep, err := client.FindEndpointByName("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}

func TestFindEndpointByName_EmptyList(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})

	client := newTestClient()
	ep, err := client.FindEndpointByName("any")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}

func TestCreateTemplate_ReturnsID(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		// Handle template list (checking for existing templates)
		if len(args) > 0 && args[0] == "template" && len(args) > 1 && args[1] == "list" {
			return json.Marshal([]map[string]interface{}{})
		}
		// Handle template create
		out, _ := json.Marshal(map[string]interface{}{"id": "tpl-abc123", "name": "test"})
		return out, nil
	})

	client := newTestClient()
	id, err := client.CreateTemplate("test", "runpod/worker-vllm:latest", "meta/llama3:8b", "hf-key", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "tpl-abc123" {
		t.Errorf("expected 'tpl-abc123', got %q", id)
	}
}

func TestCreateTemplate_ArgsContainServerless(t *testing.T) {
	var capturedArgs []string
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		// Handle template list (checking for existing templates)
		if len(args) > 0 && args[0] == "template" && len(args) > 1 && args[1] == "list" {
			return json.Marshal([]map[string]interface{}{})
		}
		// Capture create args
		capturedArgs = args
		out, _ := json.Marshal(map[string]interface{}{"id": "tpl-123"})
		return out, nil
	})

	client := newTestClient()
	_, _ = client.CreateTemplate("test", "image", "model", "key", 50)

	found := false
	for _, a := range capturedArgs {
		if a == "--serverless" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("args should contain '--serverless': %v", capturedArgs)
	}
}

func TestCreateEndpoint_ReturnsID(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal(map[string]interface{}{"id": "ep-xyz789", "name": "test"})
		return out, nil
	})

	client := newTestClient()
	id, err := client.CreateEndpoint("test-endpoint", "AMPERE_16", "tpl-123", 3, 5, 4, "QUEUE_DELAY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "ep-xyz789" {
		t.Errorf("expected 'ep-xyz789', got %q", id)
	}
}

func TestCreateEndpoint_ArgsContainFlashBoot(t *testing.T) {
	var capturedArgs []string
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		capturedArgs = args
		out, _ := json.Marshal(map[string]interface{}{"id": "ep-123"})
		return out, nil
	})

	client := newTestClient()
	_, _ = client.CreateEndpoint("test", "GPU_ID", "tpl-123", 3, 5, 4, "QUEUE_DELAY")

	found := false
	for _, a := range capturedArgs {
		if a == "--flash-boot" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("args should contain '--flash-boot': %v", capturedArgs)
	}
}

func TestScaleToZero_ArgsContainWorkersZero(t *testing.T) {
	var capturedArgs []string
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		capturedArgs = args
		out, _ := json.Marshal(map[string]interface{}{"id": "ep-abc"})
		return out, nil
	})

	client := newTestClient()
	err := client.ScaleToZero("ep-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "ep-abc") {
		t.Errorf("args should contain endpoint id: %v", capturedArgs)
	}
	if !strings.Contains(joined, "--workers-max 0") {
		t.Errorf("args should contain '--workers-max 0': %v", capturedArgs)
	}
}

func TestDeleteEndpoint_ArgsContainDelete(t *testing.T) {
	var capturedArgs []string
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte("{}"), nil
	})

	client := newTestClient()
	err := client.DeleteEndpoint("ep-del123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "serverless delete") {
		t.Errorf("args should contain 'serverless delete': %v", capturedArgs)
	}
	if !strings.Contains(joined, "ep-del123") {
		t.Errorf("args should contain endpoint id: %v", capturedArgs)
	}
}

func TestCLIError_SurfacesAsError(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		return nil, errors.New("exit status 1")
	})

	client := newTestClient()
	_, err := client.CreateTemplate("test", "image", "model", "key", 50)
	if err == nil {
		t.Fatal("expected error from CLI failure")
	}
}
