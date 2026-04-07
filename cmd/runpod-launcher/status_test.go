package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

// executeStatusDirect calls runStatus directly, setting package-level flags before the call.
// It restores all mutated package-level state via t.Cleanup.
func executeStatusDirect(t *testing.T, configPath string, jsonFlag bool) (string, error) {
	t.Helper()

	origCfgFile := cfgFile
	origStatusJSON := statusJSON
	t.Cleanup(func() {
		cfgFile = origCfgFile
		statusJSON = origStatusJSON
	})

	cfgFile = configPath
	statusJSON = jsonFlag

	var stdout bytes.Buffer
	statusCmd.SetOut(&stdout)
	statusCmd.SetErr(bytes.NewBuffer(nil))

	err := runStatus(statusCmd, nil)
	return stdout.String(), err
}

// TestStatus_Running tests that `status --json` reports the actual desiredStatus when pod is found.
func TestStatus_Running(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-abc123", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeStatusDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"status":"RUNNING"`) {
		t.Errorf("expected status:RUNNING in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"pod_id":"pod-abc123"`) {
		t.Errorf("expected pod_id in output, got: %s", stdout)
	}
}

// TestStatus_Starting tests that `status --json` reports STARTING desiredStatus
// when the pod is found but has not yet become RUNNING.
func TestStatus_Starting(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-starting", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "STARTING", DesiredStatus: "STARTING"}, nil
		},
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeStatusDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"status":"STARTING"`) {
		t.Errorf("expected status:STARTING in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"pod_id":"pod-starting"`) {
		t.Errorf("expected pod_id in output, got: %s", stdout)
	}
}

// TestStatus_GetPodStatusError tests that runStatus propagates an error from GetPodStatus
// when FindPodByName returns a valid pod ID but GetPodStatus fails.
func TestStatus_GetPodStatusError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-abc123", nil },
		getStatusFn:  func(podID string) (*pod.PodStatus, error) { return nil, errors.New("API timeout") },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeStatusDirect(t, configPath, true)
	if err == nil {
		t.Fatal("expected an error from GetPodStatus, got nil")
	}
	if !strings.Contains(err.Error(), "API timeout") {
		t.Errorf("expected 'API timeout' in error, got: %v", err)
	}
}

// TestStatus_FindPodByNameError tests that runStatus propagates an error from FindPodByName.
func TestStatus_FindPodByNameError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", errors.New("network timeout") },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeStatusDirect(t, configPath, true)
	if err == nil {
		t.Fatal("expected an error from FindPodByName, got nil")
	}
	if !strings.Contains(err.Error(), "network timeout") {
		t.Errorf("expected 'network timeout' in error, got: %v", err)
	}
}

// TestStatus_NotFound tests that `status --json` reports not_found when no pod exists.
func TestStatus_NotFound(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeStatusDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"status":"not_found"`) {
		t.Errorf("expected status:not_found in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"pod_id":null`) {
		t.Errorf("expected pod_id:null in output, got: %s", stdout)
	}
}

// TestStatus_PlainText_Running tests that `status` (without --json) prints
// "Pod status: <desiredStatus> (<id>)" reflecting the actual RunPod desiredStatus.
func TestStatus_PlainText_Running(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-plain-run", nil },
		getStatusFn: func(podID string) (*pod.PodStatus, error) {
			return &pod.PodStatus{ID: podID, Status: "RUNNING", DesiredStatus: "RUNNING"}, nil
		},
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeStatusDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "Pod status:") {
		t.Errorf("expected 'Pod status:' in plain text output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "RUNNING") {
		t.Errorf("expected desiredStatus 'RUNNING' in plain text output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "pod-plain-run") {
		t.Errorf("expected pod ID in plain text output, got: %s", stdout)
	}
}

// TestStatus_PlainText_NotFound tests that `status` (without --json) prints
// "Pod not found" when no pod exists.
func TestStatus_PlainText_NotFound(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeStatusDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "Pod not found") {
		t.Errorf("expected 'Pod not found' in plain text output, got: %s", stdout)
	}
}
