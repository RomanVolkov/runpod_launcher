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
	createTemplateFn func(string, string, string, string, int) (string, error)
	createEndpointFn func(string, string, string, int, int, int, string) (string, error)
	scaleToZeroFn    func(string) error
	deleteEndpointFn func(string) error
}

func (m *mockServerlessClient) FindEndpointByName(name string) (*serverless.Endpoint, error) {
	if m.findEndpointFn != nil {
		return m.findEndpointFn(name)
	}
	return nil, errors.New("findEndpointFn not set")
}

func (m *mockServerlessClient) CreateTemplate(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) {
	if m.createTemplateFn != nil {
		return m.createTemplateFn(name, imageName, modelName, apiKey, containerDiskGB)
	}
	return "", errors.New("createTemplateFn not set")
}

func (m *mockServerlessClient) CreateEndpoint(name, gpuID, templateID string, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
	if m.createEndpointFn != nil {
		return m.createEndpointFn(name, gpuID, templateID, workersMax, idleTimeout, scalerValue, scalerType)
	}
	return "", errors.New("createEndpointFn not set")
}

func (m *mockServerlessClient) ScaleToZero(endpointID string) error {
	if m.scaleToZeroFn != nil {
		return m.scaleToZeroFn(endpointID)
	}
	return errors.New("scaleToZeroFn not set")
}

func (m *mockServerlessClient) DeleteEndpoint(endpointID string) error {
	if m.deleteEndpointFn != nil {
		return m.deleteEndpointFn(endpointID)
	}
	return errors.New("deleteEndpointFn not set")
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
		findEndpointFn:   func(name string) (*serverless.Endpoint, error) { return nil, nil },
		createTemplateFn: func(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) { return "tpl-123", nil },
		createEndpointFn: func(name, gpuID, templateID string, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
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
		findEndpointFn:   func(name string) (*serverless.Endpoint, error) { return nil, nil },
		createTemplateFn: func(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) { return "tpl-123", nil },
		createEndpointFn: func(name, gpuID, templateID string, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
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

func TestServerlessUp_CreateTemplateError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn: func(name string) (*serverless.Endpoint, error) { return nil, nil },
		createTemplateFn: func(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) {
			return "", errors.New("template creation failed")
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessUpDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error from CreateTemplate")
	}
	if !strings.Contains(err.Error(), "template") {
		t.Errorf("error should mention template: %v", err)
	}
}

func TestServerlessUp_CreateEndpointError(t *testing.T) {
	configPath := writeTestConfig(t, testConfig)

	mock := &mockServerlessClient{
		findEndpointFn:   func(name string) (*serverless.Endpoint, error) { return nil, nil },
		createTemplateFn: func(name, imageName, modelName, apiKey string, containerDiskGB int) (string, error) { return "tpl-123", nil },
		createEndpointFn: func(name, gpuID, templateID string, workersMax, idleTimeout, scalerValue int, scalerType string) (string, error) {
			return "", errors.New("endpoint creation failed")
		},
	}

	orig := newServerlessClient
	t.Cleanup(func() { newServerlessClient = orig })
	newServerlessClient = func(apiKey string) serverless.Client { return mock }

	_, err := executeServerlessUpDirect(t, configPath, false)
	if err == nil {
		t.Fatal("expected error from CreateEndpoint")
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
