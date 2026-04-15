package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/romanvolkov/runpod-launcher/internal/config"
	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

// testConfig is a valid TOML configuration used across command tests.
const testConfig = `
runpod_api_key    = "rp_test"
gpu_type_id       = "AMPERE_16"
image_name        = "vllm/vllm-openai:latest"
model_name        = "mistral/Mistral-7B"
container_disk_gb = 50
volume_mount_path = "/workspace"
llm_api_key       = "my-llm-key"
pod_name          = "llm-launcher"
`

// writeTestConfig writes the given TOML content to a temp file with 0600 permissions
// and returns the path.
func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeTestConfig: %v", err)
	}
	return path
}

// mockClient implements pod.PodClient for testing.
type mockClient struct {
	createFn      func(*config.Config, string) (string, error)
	getStatusFn   func(string) (*pod.PodStatus, error)
	terminateFn   func(string) error
	findByNameFn  func(string) (string, error)
	getGPUTypesFn func() ([]pod.GPUType, error)
}

func (m *mockClient) CreatePod(cfg *config.Config, llmAPIKey string) (string, error) {
	if m.createFn != nil {
		return m.createFn(cfg, llmAPIKey)
	}
	return "", errors.New("createFn not set")
}

func (m *mockClient) GetPodStatus(podID string) (*pod.PodStatus, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(podID)
	}
	return nil, errors.New("getStatusFn not set")
}

func (m *mockClient) TerminatePod(podID string) error {
	if m.terminateFn != nil {
		return m.terminateFn(podID)
	}
	return errors.New("terminateFn not set")
}

func (m *mockClient) FindPodByName(name string) (string, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(name)
	}
	return "", errors.New("findByNameFn not set")
}

func (m *mockClient) GetGPUTypes() ([]pod.GPUType, error) {
	if m.getGPUTypesFn != nil {
		return m.getGPUTypesFn()
	}
	return nil, errors.New("getGPUTypesFn not set")
}

// executeUpDirect calls runUp directly, setting package-level flags before the call.
// It restores all mutated package-level state via t.Cleanup.
// Note: upOpenCodeConfig is not reset here so tests can set it before calling this function.
func executeUpDirect(t *testing.T, configPath string, jsonFlag bool) (string, error) {
	t.Helper()

	origCfgFile := cfgFile
	origUpJSON := upJSON
	t.Cleanup(func() {
		cfgFile = origCfgFile
		upJSON = origUpJSON
	})

	cfgFile = configPath
	upJSON = jsonFlag

	var stdout bytes.Buffer
	upCmd.SetOut(&stdout)
	upCmd.SetErr(bytes.NewBuffer(nil)) // discard stderr for tests

	err := runUp(upCmd, nil)
	return stdout.String(), err
}

// TestUp_JSONOutput_Success tests that `up --json` writes the correct JSON schema to stdout
// when the pod is created and becomes RUNNING.
func TestUp_JSONOutput_Success(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeUpDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"status":"running"`) {
		t.Errorf("expected status:running in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"pod_id":"pod-abc123"`) {
		t.Errorf("expected pod_id in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"url":"https://pod-abc123-8000.proxy.runpod.net"`) {
		t.Errorf("expected proxy url in output, got: %s", stdout)
	}
}

// TestUp_AlreadyRunning tests that `up` exits 0 and outputs existing pod info
// when FindPodByName returns an existing pod ID.
func TestUp_AlreadyRunning(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-existing", nil },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeUpDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error for already-running pod, got: %v", err)
	}

	if !strings.Contains(stdout, `"pod_id":"pod-existing"`) {
		t.Errorf("expected existing pod_id in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"status":"running"`) {
		t.Errorf("expected status:running in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"url":"https://pod-existing-8000.proxy.runpod.net"`) {
		t.Errorf("expected proxy url in output, got: %s", stdout)
	}
}

// TestUp_AlreadyRunning_PlainText tests that `up` (without --json) outputs existing pod
// info in plain text.
func TestUp_AlreadyRunning_PlainText(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-existing", nil },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error for already-running pod, got: %v", err)
	}

	if !strings.Contains(stdout, "pod-existing") {
		t.Errorf("expected pod ID in plain text output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "proxy.runpod.net") {
		t.Errorf("expected proxy URL in plain text output, got: %s", stdout)
	}
}

// TestUp_NewPod_PlainText tests that `up` (without --json) prints "Pod is ready:" for a
// newly created pod, exercising the plain-text success path that was previously untested.
func TestUp_NewPod_PlainText(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-new-123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "Pod is ready:") {
		t.Errorf("expected 'Pod is ready:' in plain text output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "pod-new-123") {
		t.Errorf("expected pod ID in plain text output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "proxy.runpod.net") {
		t.Errorf("expected proxy URL in plain text output, got: %s", stdout)
	}
}

// TestUp_WaitForReady_Timeout tests that runUp returns a non-zero exit (non-nil error)
// when WaitForReady times out because the pod never reaches RUNNING status.
func TestUp_WaitForReady_Timeout(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-timeout-123", nil },
		// getStatusFn always returns STARTING — pod never becomes RUNNING.
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "STARTING", DesiredStatus: "STARTING"}, nil
		},
	}

	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	// Use a very short timeout and tick so the test finishes quickly.
	origTimeout := upWaitTimeout
	origTick := upWaitTick
	t.Cleanup(func() {
		upWaitTimeout = origTimeout
		upWaitTick = origTick
	})
	upWaitTimeout = 50 * time.Millisecond
	upWaitTick = 10 * time.Millisecond

	_, err := executeUpDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected non-nil error when WaitForReady times out, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error message, got: %v", err)
	}
}

// TestUp_CreatePodError tests that runUp propagates an error from CreatePod.
func TestUp_CreatePodError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "", errors.New("API quota exceeded") },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeUpDirect(t, configPath, true)
	if err == nil {
		t.Fatal("expected an error from CreatePod, got nil")
	}
	if !strings.Contains(err.Error(), "API quota exceeded") {
		t.Errorf("expected 'API quota exceeded' in error, got: %v", err)
	}
}

// TestPodProxyURL tests that podProxyURL returns the correct RunPod proxy URL format.
func TestPodProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		podID    string
		port     int
		expected string
	}{
		{
			name:     "standard pod with port 8000",
			podID:    "pod-abc123",
			port:     8000,
			expected: "https://pod-abc123-8000.proxy.runpod.net",
		},
		{
			name:     "pod with different port",
			podID:    "pod-xyz789",
			port:     5000,
			expected: "https://pod-xyz789-5000.proxy.runpod.net",
		},
		{
			name:     "pod ID with numbers only",
			podID:    "123456",
			port:     8000,
			expected: "https://123456-8000.proxy.runpod.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := podProxyURL(tt.podID, tt.port)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestUp_OpenCodeConfig_FlagPrecedence tests that --opencode-config flag takes precedence
// over the config file value.
func TestUp_OpenCodeConfig_FlagPrecedence(t *testing.T) {
	configContent := testConfig + `
opencode_config_path = "/tmp/config-from-file.json"
`
	configPath := writeTestConfig(t, configContent)

	// Track which path the updater was called with
	var updateCalled bool
	var updatePath string
	mockUpdateOpenCode := func(path, baseURL, apiKey, modelName string) error {
		updateCalled = true
		updatePath = path
		return nil
	}

	origUpdateOpenCodeConfig := updateOpenCodeConfig
	t.Cleanup(func() { updateOpenCodeConfig = origUpdateOpenCodeConfig })
	updateOpenCodeConfig = mockUpdateOpenCode

	// Override upOpenCodeConfig via flag
	origOpenCodeConfig := upOpenCodeConfig
	t.Cleanup(func() { upOpenCodeConfig = origOpenCodeConfig })
	upOpenCodeConfig = "/tmp/config-from-flag.json"

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !updateCalled {
		t.Fatal("expected updateOpenCodeConfig to be called")
	}
	if updatePath != "/tmp/config-from-flag.json" {
		t.Errorf("expected flag value /tmp/config-from-flag.json, got %q", updatePath)
	}
}

// TestUp_OpenCodeConfig_ConfigFileFallback tests that opencode_config_path from config
// file is used when --opencode-config flag is not set.
func TestUp_OpenCodeConfig_ConfigFileFallback(t *testing.T) {
	configContent := testConfig + `
opencode_config_path = "/tmp/config-from-file.json"
`
	configPath := writeTestConfig(t, configContent)

	// Track which path the updater was called with
	var updateCalled bool
	var updatePath string
	mockUpdateOpenCode := func(path, baseURL, apiKey, modelName string) error {
		updateCalled = true
		updatePath = path
		return nil
	}

	origUpdateOpenCodeConfig := updateOpenCodeConfig
	t.Cleanup(func() { updateOpenCodeConfig = origUpdateOpenCodeConfig })
	updateOpenCodeConfig = mockUpdateOpenCode

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !updateCalled {
		t.Fatal("expected updateOpenCodeConfig to be called")
	}
	if updatePath != "/tmp/config-from-file.json" {
		t.Errorf("expected config file value /tmp/config-from-file.json, got %q", updatePath)
	}
}

// TestUp_OpenCodeConfig_SkipWhenEmpty tests that OpenCode updater is not called
// when neither --opencode-config flag nor config file value is set.
func TestUp_OpenCodeConfig_SkipWhenEmpty(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	// Track calls to the updater
	var updateCalled bool
	mockUpdateOpenCode := func(path, baseURL, apiKey, modelName string) error {
		updateCalled = true
		return nil
	}

	origUpdateOpenCodeConfig := updateOpenCodeConfig
	t.Cleanup(func() { updateOpenCodeConfig = origUpdateOpenCodeConfig })
	updateOpenCodeConfig = mockUpdateOpenCode

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updateCalled {
		t.Fatal("expected updateOpenCodeConfig NOT to be called when path is empty")
	}
}

// TestUp_OpenCodeConfig_JSONOutput tests that JSON output includes opencode_updated
// field when config path is set.
func TestUp_OpenCodeConfig_JSONOutput(t *testing.T) {
	configContent := testConfig + `
opencode_config_path = "/tmp/config.json"
`
	configPath := writeTestConfig(t, configContent)

	mockUpdateOpenCode := func(path, baseURL, apiKey, modelName string) error {
		return nil
	}

	origUpdateOpenCodeConfig := updateOpenCodeConfig
	t.Cleanup(func() { updateOpenCodeConfig = origUpdateOpenCodeConfig })
	updateOpenCodeConfig = mockUpdateOpenCode

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeUpDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"opencode_updated":true`) {
		t.Errorf("expected opencode_updated:true in JSON output, got: %s", stdout)
	}
}

// TestUp_OpenCodeConfig_JSONOutput_NoUpdate tests that opencode_updated is false
// when no config path is set.
func TestUp_OpenCodeConfig_JSONOutput_NoUpdate(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeUpDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"opencode_updated":false`) {
		t.Errorf("expected opencode_updated:false in JSON output, got: %s", stdout)
	}
}

// TestUp_OpenCodeConfig_UpdateError tests that an error from updateOpenCodeConfig
// is propagated back to the caller.
func TestUp_OpenCodeConfig_UpdateError(t *testing.T) {
	configContent := testConfig + `
opencode_config_path = "/tmp/config.json"
`
	configPath := writeTestConfig(t, configContent)

	mockUpdateOpenCode := func(path, baseURL, apiKey, modelName string) error {
		return errors.New("parent directory does not exist")
	}

	origUpdateOpenCodeConfig := updateOpenCodeConfig
	t.Cleanup(func() { updateOpenCodeConfig = origUpdateOpenCodeConfig })
	updateOpenCodeConfig = mockUpdateOpenCode

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeUpDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error from updateOpenCodeConfig, got nil")
	}
	if !strings.Contains(err.Error(), "parent directory does not exist") {
		t.Errorf("expected error message to contain 'parent directory does not exist', got: %v", err)
	}
}

// TestUp_OpenCodeConfig_PlainTextOutput tests that plain text output includes
// OpenCode config path when update succeeds.
func TestUp_OpenCodeConfig_PlainTextOutput(t *testing.T) {
	configContent := testConfig + `
opencode_config_path = "/tmp/config.json"
`
	configPath := writeTestConfig(t, configContent)

	mockUpdateOpenCode := func(path, baseURL, apiKey, modelName string) error {
		return nil
	}

	origUpdateOpenCodeConfig := updateOpenCodeConfig
	t.Cleanup(func() { updateOpenCodeConfig = origUpdateOpenCodeConfig })
	updateOpenCodeConfig = mockUpdateOpenCode

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "OpenCode config updated: /tmp/config.json") {
		t.Errorf("expected 'OpenCode config updated: /tmp/config.json' in output, got: %s", stdout)
	}
}

// TestUp_OpenCodeConfig_CorrectURL tests that the correct proxy URL with /v1 suffix
// is passed to updateOpenCodeConfig.
func TestUp_OpenCodeConfig_CorrectURL(t *testing.T) {
	configContent := testConfig + `
opencode_config_path = "/tmp/config.json"
`
	configPath := writeTestConfig(t, configContent)

	var updateURL string
	mockUpdateOpenCode := func(path, baseURL, apiKey, modelName string) error {
		updateURL = baseURL
		return nil
	}

	origUpdateOpenCodeConfig := updateOpenCodeConfig
	t.Cleanup(func() { updateOpenCodeConfig = origUpdateOpenCodeConfig })
	updateOpenCodeConfig = mockUpdateOpenCode

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
		createFn:     func(cfg *config.Config, llmAPIKey string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	origClient := newPodClient
	t.Cleanup(func() { newPodClient = origClient })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := "https://pod-abc123-8000.proxy.runpod.net/v1"
	if updateURL != expected {
		t.Errorf("expected URL %q, got %q", expected, updateURL)
	}
}
