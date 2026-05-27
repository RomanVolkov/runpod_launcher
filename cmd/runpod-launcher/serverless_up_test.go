package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/serverless"
)

// mockServerlessClient implements serverless.Client for testing.
type mockServerlessClient struct {
	findEndpointFn   func(string) (*serverless.Endpoint, error)
	saveTemplateFn   func(string, string, string, string, int) (string, error)
	saveEndpointFn   func(string, string, string, string, int, int, int, int, string) (string, error)
}

func (m *mockServerlessClient) FindEndpointByName(name string) (*serverless.Endpoint, error) {
	if m.findEndpointFn != nil {
		return m.findEndpointFn(name)
	}
	return nil, errors.New("findEndpointFn not set")
}

func (m *mockServerlessClient) SaveTemplate(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) {
	if m.saveTemplateFn != nil {
		return m.saveTemplateFn(name, imageName, modelName, apiKey, containerDiskGB)
	}
	return "", errors.New("saveTemplateFn not set")
}

func (m *mockServerlessClient) SaveEndpoint(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
	if m.saveEndpointFn != nil {
		return m.saveEndpointFn(endpointID, name, gpuIDs, templateID, workersMin, workersMax, idleTimeout, scalerValue, scalerType)
	}
	return "", errors.New("saveEndpointFn not set")
}

// executeServerlessUpDirect calls runServerlessUp directly, setting package-level flags before the call.
func executeServerlessUpDirect(t *testing.T, configPath string, jsonFlag bool) (string, error) {
	t.Helper()

	origCfgFile := cfgFile
	origServerlessUpJSON := serverlessUpJSON
	origServerlessUpOpenCodeConfig := serverlessUpOpenCodeConfig
	origNewServerlessClient := newServerlessClient
	origUpdateServerlessOpenCodeConfig := updateServerlessOpenCodeConfig

	t.Cleanup(func() {
		cfgFile = origCfgFile
		serverlessUpJSON = origServerlessUpJSON
		serverlessUpOpenCodeConfig = origServerlessUpOpenCodeConfig
		newServerlessClient = origNewServerlessClient
		updateServerlessOpenCodeConfig = origUpdateServerlessOpenCodeConfig
	})

	cfgFile = configPath
	serverlessUpJSON = jsonFlag
	serverlessUpOpenCodeConfig = ""

	var stdout bytes.Buffer
	serverlessUpCmd.SetOut(&stdout)
	serverlessUpCmd.SetErr(bytes.NewBuffer(nil))

	err := runServerlessUp(serverlessUpCmd, nil)
	return stdout.String(), err
}

func TestServerlessUp_JSONOutput_NewEndpoint(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) { return nil, nil },
		saveTemplateFn: func(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) { return "tpl-123", nil },
		saveEndpointFn: func(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
			return "ep-abc123", nil
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	stdout, err := executeServerlessUpDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"status":"active"`) {
		t.Errorf("expected status:active in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"endpoint_id":"ep-abc123"`) {
		t.Errorf("expected endpoint_id in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"https://api.runpod.ai/v2/ep-abc123/openai/v1"`) {
		t.Errorf("expected serverless URL in output, got: %s", stdout)
	}
}

func TestServerlessUp_JSONOutput_AlreadyExists(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-existing", Name: name, WorkersMin: 0, WorkersMax: 3}, nil
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	stdout, err := executeServerlessUpDirect(t, configPath, true)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, `"endpoint_id":"ep-existing"`) {
		t.Errorf("expected existing endpoint_id in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"status":"active"`) {
		t.Errorf("expected status:active in output, got: %s", stdout)
	}
}

func TestServerlessUp_PlainText_NewEndpoint(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) { return nil, nil },
		saveTemplateFn: func(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) { return "tpl-123", nil },
		saveEndpointFn: func(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
			return "ep-xyz", nil
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	stdout, err := executeServerlessUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "Serverless endpoint created") {
		t.Errorf("expected 'Serverless endpoint created' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "ep-xyz") {
		t.Errorf("expected endpoint ID in output, got: %s", stdout)
	}
}

func TestServerlessUp_PlainText_AlreadyExists(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) {
			return &serverless.Endpoint{ID: "ep-existing", Name: name}, nil
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	stdout, err := executeServerlessUpDirect(t, configPath, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(stdout, "Serverless endpoint already exists") {
		t.Errorf("expected 'already exists' in output, got: %s", stdout)
	}
}

func TestServerlessUp_SaveTemplateError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) { return nil, nil },
		saveTemplateFn: func(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) {
			return "", errors.New("template creation failed")
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessUpDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error from SaveTemplate")
	}
	if !strings.Contains(err.Error(), "template") {
		t.Errorf("error should mention template: %v", err)
	}
}

func TestServerlessUp_SaveEndpointError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) { return nil, nil },
		saveTemplateFn: func(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) { return "tpl-123", nil },
		saveEndpointFn: func(endpointID, name, gpuIDs, templateID string, workersMin, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
			return "", errors.New("endpoint creation failed")
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessUpDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error from SaveEndpoint")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Errorf("error should mention endpoint: %v", err)
	}
}

func TestServerlessBaseURL_Format(t *testing.T) {
	url := serverlessBaseURL("ep-test123")
	expected := "https://api.runpod.ai/v2/ep-test123/openai/v1"
	if url != expected {
		t.Errorf("serverlessBaseURL: got %q, expected %q", url, expected)
	}
}
