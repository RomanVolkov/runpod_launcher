package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/pod"
)

// executeDownDirect calls runDown directly, setting package-level flags before the call.
// It restores all mutated package-level state via t.Cleanup.
func executeDownDirect(t *testing.T, configPath string, jsonFlag bool) (string, error) {
	t.Helper()

	origCfgFile := cfgFile
	origDownJSON := downJSON
	t.Cleanup(func() {
		cfgFile = origCfgFile
		downJSON = origDownJSON
	})

	cfgFile = configPath
	downJSON = jsonFlag

	var stdout bytes.Buffer
	downCmd.SetOut(&stdout)
	downCmd.SetErr(bytes.NewBuffer(nil)) // discard stderr for tests

	err := runDown(downCmd, nil)
	return stdout.String(), err
}

// TestDown_JSONOutput_Success tests that `down --json` writes the correct JSON to stdout.
func TestDown_JSONOutput_Success(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	terminateCalled := false
	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-xyz", nil },
		terminateFn: func(podID string) error {
			terminateCalled = true
			return nil
		},
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeDownDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !terminateCalled {
		t.Error("expected TerminatePod to be called")
	}
	if !strings.Contains(stdout, `"status":"terminated"`) {
		t.Errorf("expected status:terminated in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"pod_id":"pod-xyz"`) {
		t.Errorf("expected pod_id in output, got: %s", stdout)
	}
}

// TestDown_PodNotFound tests that `down` returns a non-zero exit when pod is not found.
func TestDown_PodNotFound(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "", nil },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeDownDirect(t, configPath, true)
	if err == nil {
		t.Fatal("expected an error when pod not found, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got: %v", err)
	}
}

// TestDown_PlainText_Success tests that `down` (without --json) prints the plain-text
// "Pod terminated: <id>" message to stdout.
func TestDown_PlainText_Success(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-plain", nil },
		terminateFn:  func(podID string) error { return nil },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	stdout, err := executeDownDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "Pod terminated:") {
		t.Errorf("expected 'Pod terminated:' in plain text output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "pod-plain") {
		t.Errorf("expected pod ID in plain text output, got: %s", stdout)
	}
}

// TestDown_TerminatePodError tests that runDown propagates an error from TerminatePod.
func TestDown_TerminatePodError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockClient{
		findByNameFn: func(name string) (string, error) { return "pod-xyz", nil },
		terminateFn:  func(podID string) error { return errors.New("termination rejected by API") },
	}
	orig := newPodClient
	t.Cleanup(func() { newPodClient = orig })
	newPodClient = func(apiKey string) pod.PodClient { return mock }

	_, err := executeDownDirect(t, configPath, true)
	if err == nil {
		t.Fatal("expected an error from TerminatePod, got nil")
	}
	if !strings.Contains(err.Error(), "termination rejected by API") {
		t.Errorf("expected 'termination rejected by API' in error, got: %v", err)
	}
}
