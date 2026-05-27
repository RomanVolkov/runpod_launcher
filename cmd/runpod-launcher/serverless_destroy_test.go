package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/serverless"
)

func executeServerlessDestroyDirect(t *testing.T, configPath string, jsonFlag bool) (string, error) {
	t.Helper()

	origCfgFile := cfgFile
	origServerlessDestroyJSON := serverlessDestroyJSON
	origNewServerlessClient := newServerlessClient

	t.Cleanup(func() {
		cfgFile = origCfgFile
		serverlessDestroyJSON = origServerlessDestroyJSON
		newServerlessClient = origNewServerlessClient
	})

	cfgFile = configPath
	serverlessDestroyJSON = jsonFlag

	var stdout bytes.Buffer
	serverlessDestroyCmd.SetOut(&stdout)
	serverlessDestroyCmd.SetErr(bytes.NewBuffer(nil))

	err := runServerlessDestroy(serverlessDestroyCmd, nil)
	return stdout.String(), err
}

func TestServerlessDestroy_JSONOutput_Success(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-abc123", Name: name}, nil
		},
		deleteEndpointFn: func(endpointID string) error { return nil },
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	stdout, err := executeServerlessDestroyDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"status":"deleted"`) {
		t.Errorf("expected status:deleted in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"endpoint_id":"ep-abc123"`) {
		t.Errorf("expected endpoint_id in output, got: %s", stdout)
	}
}

func TestServerlessDestroy_PlainText_Success(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-xyz", Name: name}, nil
		},
		deleteEndpointFn: func(endpointID string) error { return nil },
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	stdout, err := executeServerlessDestroyDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "Serverless endpoint deleted") {
		t.Errorf("expected 'Serverless endpoint deleted' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "ep-xyz") {
		t.Errorf("expected endpoint ID in output, got: %s", stdout)
	}
}

func TestServerlessDestroy_NotFound(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) { return nil, nil },
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessDestroyDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error when endpoint not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

func TestServerlessDestroy_DeleteError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-123", Name: name}, nil
		},
		deleteEndpointFn: func(endpointID string) error {
			return errors.New("delete operation failed")
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessDestroyDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error from DeleteEndpoint")
	}
	if !strings.Contains(err.Error(), "delete") {
		t.Errorf("error should mention delete: %v", err)
	}
}
