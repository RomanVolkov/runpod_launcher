package pod

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/romanvolkov/runpod-launcher/internal/config"
)

// mockPodClient implements PodClient for testing without real CLI calls.
type mockPodClient struct {
	createFn      func(*config.Config, string) (string, error)
	getStatusFn   func(string) (*PodStatus, error)
	terminateFn   func(string) error
	findByNameFn  func(string) (string, error)
	getGPUTypesFn func() ([]GPUType, error)
}

func (m *mockPodClient) CreatePod(cfg *config.Config, llmAPIKey string) (string, error) {
	if m.createFn != nil {
		return m.createFn(cfg, llmAPIKey)
	}
	return "", errors.New("createFn not set")
}

func (m *mockPodClient) GetPodStatus(podID string) (*PodStatus, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(podID)
	}
	return nil, errors.New("getStatusFn not set")
}

func (m *mockPodClient) TerminatePod(podID string) error {
	if m.terminateFn != nil {
		return m.terminateFn(podID)
	}
	return errors.New("terminateFn not set")
}

func (m *mockPodClient) FindPodByName(name string) (string, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(name)
	}
	return "", errors.New("findByNameFn not set")
}

func (m *mockPodClient) GetGPUTypes() ([]GPUType, error) {
	if m.getGPUTypesFn != nil {
		return m.getGPUTypesFn()
	}
	return nil, errors.New("getGPUTypesFn not set")
}

// withMockCLI temporarily replaces RunpodCtlFn for the duration of the test.
func withMockCLI(t *testing.T, fn func(apiKey string, args ...string) ([]byte, error)) {
	t.Helper()
	orig := RunpodCtlFn
	t.Cleanup(func() { RunpodCtlFn = orig })
	RunpodCtlFn = fn
}

const shortTick = 10 * time.Millisecond

func TestWaitForReady_SucceedsOnSecondCall(t *testing.T) {
	callCount := 0
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			callCount++
			if callCount >= 2 {
				return &PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
			}
			return &PodStatus{ID: podID, Status: "STARTING", DesiredStatus: "RUNNING"}, nil
		},
	}

	var stderr bytes.Buffer
	err := WaitForReady(mock, "pod-123", 30*time.Second, &stderr, shortTick)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 GetPodStatus calls, got %d", callCount)
	}
}

func TestWaitForReady_SucceedsImmediately(t *testing.T) {
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			return &PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}

	var stderr bytes.Buffer
	err := WaitForReady(mock, "pod-abc", 10*time.Second, &stderr, shortTick)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestWaitForReady_TimesOut(t *testing.T) {
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			return &PodStatus{ID: podID, Status: "STARTING", DesiredStatus: "RUNNING"}, nil
		},
	}

	var stderr bytes.Buffer
	err := WaitForReady(mock, "pod-xyz", 1*time.Millisecond, &stderr, shortTick)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected error to mention 'timed out', got: %v", err)
	}
}

func TestWaitForReady_PrintsToStderr(t *testing.T) {
	callCount := 0
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			callCount++
			if callCount >= 2 {
				return &PodStatus{ID: podID, Status: "RUNNING"}, nil
			}
			return &PodStatus{ID: podID, Status: "STARTING"}, nil
		},
	}

	var stderr bytes.Buffer
	_ = WaitForReady(mock, "pod-123", 30*time.Second, &stderr, shortTick)

	if stderr.Len() == 0 {
		t.Error("expected progress output on stderr, got none")
	}
}

func TestWaitForReady_ErrorContinues(t *testing.T) {
	callCount := 0
	mock := &mockPodClient{
		getStatusFn: func(podID string) (*PodStatus, error) {
			callCount++
			if callCount == 1 {
				return &PodStatus{ID: podID, Status: "STARTING"}, nil
			}
			if callCount == 2 {
				return nil, errors.New("transient API error")
			}
			return &PodStatus{ID: podID, Status: "RUNNING"}, nil
		},
	}

	var stderr bytes.Buffer
	err := WaitForReady(mock, "pod-err", 30*time.Second, &stderr, shortTick)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "e") {
		t.Errorf("expected 'e' in stderr output for error tick, got: %q", stderr.String())
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 GetPodStatus calls, got %d", callCount)
	}
}

// ---- RunPodClient CLI tests using RunpodCtlFn mocking ----

func TestRunPodClient_CreatePod_BuildsCorrectArgs(t *testing.T) {
	var capturedArgs []string
	var capturedAPIKey string

	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		capturedAPIKey = apiKey
		capturedArgs = args
		out, _ := json.Marshal(map[string]interface{}{"id": "pod-created-123", "desiredStatus": "RUNNING"})
		return out, nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	cfg := &config.Config{
		RunpodAPIKey:    "test-api-key",
		GPUTypeID:       "AMPERE_16",
		ImageName:       "vllm/vllm-openai:latest",
		ModelName:       "mistral/Mistral-7B",
		ContainerDiskGB: 50,
		VolumeMountPath: "/workspace",
	}

	id, err := client.CreatePod(cfg, "test-llm-key")
	if err != nil {
		t.Fatalf("CreatePod returned error: %v", err)
	}
	if id != "pod-created-123" {
		t.Errorf("expected pod ID 'pod-created-123', got %q", id)
	}
	if capturedAPIKey != "test-api-key" {
		t.Errorf("expected API key 'test-api-key', got %q", capturedAPIKey)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "pod create") {
		t.Errorf("expected 'pod create' in args: %v", capturedArgs)
	}
	if !strings.Contains(joined, "AMPERE_16") {
		t.Errorf("expected GPU ID in args: %v", capturedArgs)
	}
	if !strings.Contains(joined, "--image") {
		t.Errorf("expected '--image' in args: %v", capturedArgs)
	}
	if !strings.Contains(joined, "--ssh") {
		t.Errorf("expected '--ssh' in args: %v", capturedArgs)
	}
}

func TestRunPodClient_GetPodStatus_Running(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal(map[string]interface{}{
			"id":            "pod-1",
			"desiredStatus": "RUNNING",
			"runtime":       map[string]interface{}{"uptimeInSeconds": 120},
		})
		return out, nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	status, err := client.GetPodStatus("pod-1")
	if err != nil {
		t.Fatalf("GetPodStatus returned error: %v", err)
	}
	if status.Status != "RUNNING" {
		t.Errorf("expected Status=RUNNING, got %q", status.Status)
	}
	if status.DesiredStatus != "RUNNING" {
		t.Errorf("expected DesiredStatus=RUNNING, got %q", status.DesiredStatus)
	}
}

func TestRunPodClient_GetPodStatus_ExitedWithRuntime(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal(map[string]interface{}{
			"id":            "pod-2",
			"desiredStatus": "EXITED",
			"runtime":       map[string]interface{}{"uptimeInSeconds": 5},
		})
		return out, nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	status, err := client.GetPodStatus("pod-2")
	if err != nil {
		t.Fatalf("GetPodStatus returned error: %v", err)
	}
	if status.Status == "RUNNING" {
		t.Error("expected Status != RUNNING for EXITED pod with runtime, got RUNNING")
	}
	if status.DesiredStatus != "EXITED" {
		t.Errorf("expected DesiredStatus=EXITED, got %q", status.DesiredStatus)
	}
}

func TestRunPodClient_GetPodStatus_StartingNoRuntime(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal(map[string]interface{}{
			"id":            "pod-3",
			"desiredStatus": "STARTING",
			"runtime":       nil,
		})
		return out, nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	status, err := client.GetPodStatus("pod-3")
	if err != nil {
		t.Fatalf("GetPodStatus returned error: %v", err)
	}
	if status.Status != "STARTING" {
		t.Errorf("expected Status=STARTING when runtime is nil, got %q", status.Status)
	}
}

func TestRunPodClient_GetPodStatus_BlankDesiredStatusWithRuntime(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal(map[string]interface{}{
			"id":            "pod-blank",
			"desiredStatus": "",
			"runtime":       map[string]interface{}{"uptimeInSeconds": 10},
		})
		return out, nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	status, err := client.GetPodStatus("pod-blank")
	if err != nil {
		t.Fatalf("GetPodStatus returned error: %v", err)
	}
	if status.Status == "RUNNING" {
		t.Error("expected Status != RUNNING for blank desiredStatus with runtime, got RUNNING")
	}
}

func TestRunPodClient_FindPodByName_SkipsExited(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal([]map[string]interface{}{
			{"id": "pod-old", "name": "llm-launcher", "desiredStatus": "EXITED"},
			{"id": "pod-new", "name": "other-pod", "desiredStatus": "RUNNING"},
		})
		return out, nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	id, err := client.FindPodByName("llm-launcher")
	if err != nil {
		t.Fatalf("FindPodByName returned error: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty ID for EXITED pod, got %q", id)
	}
}

func TestRunPodClient_FindPodByName_EmptyList(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		return []byte("[]"), nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	id, err := client.FindPodByName("llm-launcher")
	if err != nil {
		t.Fatalf("expected nil error for empty list, got: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty ID for empty list, got %q", id)
	}
}

func TestRunPodClient_FindPodByName_ReturnsActive(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal([]map[string]interface{}{
			{"id": "pod-active", "name": "llm-launcher", "desiredStatus": "RUNNING"},
		})
		return out, nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	id, err := client.FindPodByName("llm-launcher")
	if err != nil {
		t.Fatalf("FindPodByName returned error: %v", err)
	}
	if id != "pod-active" {
		t.Errorf("expected pod ID 'pod-active', got %q", id)
	}
}

func TestRunPodClient_TerminatePod_CallsDelete(t *testing.T) {
	var capturedArgs []string
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte("{}"), nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	if err := client.TerminatePod("pod-terminate-me"); err != nil {
		t.Fatalf("TerminatePod returned error: %v", err)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "pod delete") {
		t.Errorf("expected 'pod delete' in args: %v", capturedArgs)
	}
	if !strings.Contains(joined, "pod-terminate-me") {
		t.Errorf("expected pod ID in args: %v", capturedArgs)
	}
}

func TestRunPodClient_CLIError_SurfacesAsError(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		return nil, errors.New("exit status 1")
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	_, err := client.GetPodStatus("pod-x")
	if err == nil {
		t.Fatal("expected error from CLI failure, got nil")
	}
}

func TestRunPodClient_GetGPUTypes_ReturnsGPUs(t *testing.T) {
	withMockCLI(t, func(apiKey string, args ...string) ([]byte, error) {
		out, _ := json.Marshal([]map[string]interface{}{
			{
				"id":                 "AMPERE_16",
				"displayName":        "RTX A4000",
				"memoryInGb":         16,
				"securePrice":        0.44,
				"communityPrice":     0.22,
				"secureSpotPrice":    0.22,
				"communitySpotPrice": 0.11,
				"secureCloud":        true,
				"communityCloud":     true,
			},
			{
				"id":             "ADA_LOVELACE_24",
				"displayName":    "RTX 5880 Ada",
				"memoryInGb":     24,
				"securePrice":    0.98,
				"communityPrice": 0.49,
				"secureCloud":    false,
				"communityCloud": true,
			},
		})
		return out, nil
	})

	client := &RunPodClient{apiKey: "test-api-key"}
	gpus, err := client.GetGPUTypes()
	if err != nil {
		t.Fatalf("GetGPUTypes returned error: %v", err)
	}

	if len(gpus) != 2 {
		t.Errorf("expected 2 GPUs, got %d", len(gpus))
	}

	if gpus[0].ID != "AMPERE_16" {
		t.Errorf("expected first GPU ID 'AMPERE_16', got %q", gpus[0].ID)
	}
	if gpus[0].DisplayName != "RTX A4000" {
		t.Errorf("expected display name 'RTX A4000', got %q", gpus[0].DisplayName)
	}
	if gpus[0].MemoryInGb != 16 {
		t.Errorf("expected 16GB memory, got %d", gpus[0].MemoryInGb)
	}
	if gpus[0].SecurePrice != 0.44 {
		t.Errorf("expected secure price 0.44, got %f", gpus[0].SecurePrice)
	}
	if !gpus[0].SecureCloud {
		t.Error("expected first GPU SecureCloud=true")
	}
	if !gpus[0].CommunityCloud {
		t.Error("expected first GPU CommunityCloud=true")
	}

	if gpus[1].ID != "ADA_LOVELACE_24" {
		t.Errorf("expected second GPU ID 'ADA_LOVELACE_24', got %q", gpus[1].ID)
	}
	if gpus[1].SecureCloud {
		t.Error("expected second GPU SecureCloud=false")
	}
	if !gpus[1].CommunityCloud {
		t.Error("expected second GPU CommunityCloud=true")
	}
}
