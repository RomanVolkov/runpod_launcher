package serverless_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/serverless"
)

func newTestServerlessClient(serverURL string) *serverless.RunPodServerlessClient {
	client := serverless.NewRunPodServerlessClient("test-api-key").(*serverless.RunPodServerlessClient)
	client.BaseURL = serverURL
	return client
}

func TestRunPodServerlessClient_FindEndpointByName_ReturnsMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"data": map[string]interface{}{
				"myself": map[string]interface{}{
					"endpoints": []map[string]interface{}{
						{
							"id":         "ep-123",
							"name":       "test-endpoint",
							"workersMin": float64(0),
							"workersMax": float64(3),
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	ep, err := client.FindEndpointByName("test-endpoint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep == nil {
		t.Fatalf("expected endpoint, got nil")
	}
	if ep.ID != "ep-123" || ep.Name != "test-endpoint" {
		t.Errorf("endpoint mismatch: got %+v", ep)
	}
}

func TestRunPodServerlessClient_FindEndpointByName_ReturnsNilWhenNotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"data": map[string]interface{}{
				"myself": map[string]interface{}{
					"endpoints": []map[string]interface{}{
						{
							"id":   "ep-999",
							"name": "other-endpoint",
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	ep, err := client.FindEndpointByName("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}

func TestRunPodServerlessClient_FindEndpointByName_NullEndpoints(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"data": map[string]interface{}{
				"myself": map[string]interface{}{
					"endpoints": nil,
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	ep, err := client.FindEndpointByName("any")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep != nil {
		t.Errorf("expected nil, got %+v", ep)
	}
}

func TestRunPodServerlessClient_SaveTemplate_ReturnsID(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"data": map[string]interface{}{
				"saveTemplate": map[string]interface{}{
					"id": "tpl-abc123",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	id, err := client.SaveTemplate("test-template", "runpod/worker-vllm:latest", "qwen/qwen3.6:27b", "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "tpl-abc123" {
		t.Errorf("expected 'tpl-abc123', got %q", id)
	}
}

func TestRunPodServerlessClient_SaveTemplate_SendsIsServerless(t *testing.T) {
	var capturedBody string
	handler := func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		capturedBody = string(buf[:n])

		result := map[string]interface{}{
			"data": map[string]interface{}{
				"saveTemplate": map[string]interface{}{
					"id": "tpl-123",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	_, _ = client.SaveTemplate("test", "image", "model", "key")

	if !strings.Contains(capturedBody, "isServerless") {
		t.Errorf("request body should contain 'isServerless': %s", capturedBody)
	}
}

func TestRunPodServerlessClient_SaveEndpoint_ReturnsID(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"data": map[string]interface{}{
				"saveEndpoint": map[string]interface{}{
					"id":   "ep-xyz789",
					"name": "test-endpoint",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	id, err := client.SaveEndpoint("", "test-endpoint", "AMPERE_16", "tpl-123", 0, 3, 5, 4, "QUEUE_DELAY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "ep-xyz789" {
		t.Errorf("expected 'ep-xyz789', got %q", id)
	}
}

func TestRunPodServerlessClient_SaveEndpoint_SendsFlashboot(t *testing.T) {
	var capturedBody string
	handler := func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		capturedBody = string(buf[:n])

		result := map[string]interface{}{
			"data": map[string]interface{}{
				"saveEndpoint": map[string]interface{}{
					"id": "ep-123",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	_, _ = client.SaveEndpoint("", "test", "GPU_ID", "tpl-123", 0, 3, 5, 4, "QUEUE_DELAY")

	if !strings.Contains(capturedBody, "flashboot") {
		t.Errorf("request body should contain 'flashboot': %s", capturedBody)
	}
}

func TestRunPodServerlessClient_SaveEndpoint_IncludesIdWhenNonEmpty(t *testing.T) {
	var capturedBody string
	handler := func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		capturedBody = string(buf[:n])

		result := map[string]interface{}{
			"data": map[string]interface{}{
				"saveEndpoint": map[string]interface{}{
					"id": "ep-456",
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	_, _ = client.SaveEndpoint("ep-existing", "test", "GPU_ID", "tpl-123", 0, 3, 5, 4, "QUEUE_DELAY")

	if !strings.Contains(capturedBody, "ep-existing") {
		t.Errorf("request body should contain endpoint ID when provided: %s", capturedBody)
	}
}

func TestRunPodServerlessClient_GraphQLError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"errors": []map[string]string{
				{"message": "unauthorized"},
			},
		}
		json.NewEncoder(w).Encode(result)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	_, err := client.SaveTemplate("test", "image", "model", "key")
	if err == nil {
		t.Fatal("expected error for GraphQL error response")
	}
	if !strings.Contains(err.Error(), "graphql error") {
		t.Errorf("error should mention 'graphql error': %v", err)
	}
}

func TestRunPodServerlessClient_HTTPError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client := newTestServerlessClient(server.URL)
	_, err := client.SaveTemplate("test", "image", "model", "key")
	if err == nil {
		t.Fatal("expected error for HTTP error response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code: %v", err)
	}
}
