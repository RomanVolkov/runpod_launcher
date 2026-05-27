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
		saveEndpointFn: func(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
			return endpointID, nil
		},
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
		saveEndpointFn: func(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
			return endpointID, nil
		},
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

func TestServerlessDown_SaveEndpointCalled_WithZeroWorkers(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	var capturedWorkersMin, capturedWorkersMax int
	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-123", Name: name, WorkersMin: 2, WorkersMax: 5}, nil
		},
		saveEndpointFn: func(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
			capturedWorkersMin = workersMin
			capturedWorkersMax = workersMax
			return endpointID, nil
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessDownDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedWorkersMin != 0 || capturedWorkersMax != 0 {
		t.Errorf("SaveEndpoint should be called with workersMin=0, workersMax=0, got %d/%d", capturedWorkersMin, capturedWorkersMax)
	}
}

func TestServerlessDown_SaveEndpointError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-123", Name: name}, nil
		},
		saveEndpointFn: func(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
			return "", errors.New("scale operation failed")
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessDownDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error from SaveEndpoint")
	}
	if !strings.Contains(err.Error(), "scale") {
		t.Errorf("error should mention scale: %v", err)
	}
}
