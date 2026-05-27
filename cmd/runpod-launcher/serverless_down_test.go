package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/serverless"
)

// executeServerlessDownDirect calls runServerlessDown directly, setting package-level flags before the call.
func executeServerlessDownDirect(t *testing.T, configPath string, jsonFlag bool) (string, error) {
	t.Helper()

	origCfgFile := cfgFile
	origServerlessDownJSON := serverlessDownJSON
	origNewServerlessClient := newServerlessClient

	t.Cleanup(func() {
		cfgFile = origCfgFile
		serverlessDownJSON = origServerlessDownJSON
		newServerlessClient = origNewServerlessClient
	})

	cfgFile = configPath
	serverlessDownJSON = jsonFlag

	var stdout bytes.Buffer
	serverlessDownCmd.SetOut(&stdout)
	serverlessDownCmd.SetErr(bytes.NewBuffer(nil))

	err := runServerlessDown(serverlessDownCmd, nil)
	return stdout.String(), err
}

func TestServerlessDown_JSONOutput_Success(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-abc123", Name: name, WorkersMin: 0, WorkersMax: 3}, nil
		},
		scaleToZeroFn: func(endpointID string) error { return nil },
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	stdout, err := executeServerlessDownDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"status":"scaled_to_zero"`) {
		t.Errorf("expected status:scaled_to_zero in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"endpoint_id":"ep-abc123"`) {
		t.Errorf("expected endpoint_id in output, got: %s", stdout)
	}
}

func TestServerlessDown_PlainText_Success(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-xyz", Name: name}, nil
		},
		scaleToZeroFn: func(endpointID string) error { return nil },
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	stdout, err := executeServerlessDownDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "Serverless endpoint scaled to zero") {
		t.Errorf("expected 'scaled to zero' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "ep-xyz") {
		t.Errorf("expected endpoint ID in output, got: %s", stdout)
	}
}

func TestServerlessDown_NotFound(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) { return nil, nil },
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessDownDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error when endpoint not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

func TestServerlessDown_ScaleToZeroCalledWithEndpointID(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	var capturedID string
	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-123", Name: name, WorkersMin: 2, WorkersMax: 5}, nil
		},
		scaleToZeroFn: func(endpointID string) error {
			capturedID = endpointID
			return nil
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessDownDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedID != "ep-123" {
		t.Errorf("ScaleToZero called with %q, want 'ep-123'", capturedID)
	}
}

func TestServerlessDown_ScaleToZeroError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-123", Name: name}, nil
		},
		scaleToZeroFn: func(endpointID string) error {
			return errors.New("scale operation failed")
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessDownDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error from ScaleToZero")
	}
	if !strings.Contains(err.Error(), "scale") {
		t.Errorf("error should mention scale: %v", err)
	}
}
