package models

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestModelsDevCache(t *testing.T) {
	cache := NewModelsDevCache(100 * time.Millisecond)

	// Initially empty
	_, ok := cache.Get("test-model")
	if ok {
		t.Error("expected cache miss on empty cache")
	}

	// Add data
	data := map[string]ModelsDev{
		"test-model": {ID: "test-model", Name: "Test Model"},
	}
	cache.Set(data)

	// Should be valid
	if !cache.IsValid() {
		t.Error("expected cache to be valid after Set")
	}

	// Should retrieve
	model, ok := cache.Get("test-model")
	if !ok {
		t.Error("expected cache hit")
	}
	if model.ID != "test-model" {
		t.Errorf("got %q, want test-model", model.ID)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)
	if cache.IsValid() {
		t.Error("expected cache to expire")
	}
}

func TestFetchAllModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"meta-llama/llama-3.1-70b": {
				"id": "meta-llama/llama-3.1-70b",
				"name": "Llama 3.1 70B",
				"family": "llama-3.1",
				"open_weights": true,
				"limit": {
					"context": 131072,
					"output": 4096
				}
			},
			"mistral/mistral-7b": {
				"id": "mistral/mistral-7b",
				"name": "Mistral 7B",
				"family": "mistral",
				"open_weights": true,
				"limit": {
					"context": 32000,
					"output": 4096
				}
			}
		}`))
	}))
	defer server.Close()

	// Create custom client with mock server
	client := &ModelsDevClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      NewModelsDevCache(1 * time.Minute),
		baseURL:    server.URL,
	}

	models, err := client.FetchAllModels()
	if err != nil {
		t.Fatalf("FetchAllModels error: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	if _, ok := models["meta-llama/llama-3.1-70b"]; !ok {
		t.Error("expected llama model in response")
	}

	if _, ok := models["mistral/mistral-7b"]; !ok {
		t.Error("expected mistral model in response")
	}
}

func TestExtractParameterCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"70b model", "meta-llama/llama-3.1-70b", 70},
		{"7b model", "mistral-7b", 7},
		{"27b model", "qwen-27b", 27},
		{"405b model", "llama-3.1-405b", 405},
		{"3.5b model", "phi-3.5b", 3},
		{"no parameters", "gpt-4", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractParameterCount(tt.input)
			if got != tt.expected {
				t.Errorf("extractParameterCount(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEstimateVRAM(t *testing.T) {
	tests := []struct {
		name     string
		modelID  string
		spec     ModelsDev
		minVRAM  int
		maxVRAM  int
	}{}

	// Test 70b model
	spec70b := ModelsDev{Name: "Llama 3.1 70B"}
	spec70b.Limit.Context = 131072
	spec70b.Limit.Output = 4096
	tests = append(tests, struct {
		name    string
		modelID string
		spec    ModelsDev
		minVRAM int
		maxVRAM int
	}{"70b model", "meta-llama/llama-3.1-70b", spec70b, 50, 200})

	// Test 7b model
	spec7b := ModelsDev{Name: "Mistral 7B"}
	spec7b.Limit.Context = 32000
	spec7b.Limit.Output = 4096
	tests = append(tests, struct {
		name    string
		modelID string
		spec    ModelsDev
		minVRAM int
		maxVRAM int
	}{"7b model", "mistral-7b", spec7b, 15, 50})

	// Test no params fallback
	specUnknown := ModelsDev{Name: "Unknown Model"}
	specUnknown.Limit.Context = 8192
	specUnknown.Limit.Output = 4096
	tests = append(tests, struct {
		name    string
		modelID string
		spec    ModelsDev
		minVRAM int
		maxVRAM int
	}{"no params fallback", "unknown-model", specUnknown, 20, 50})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vram := EstimateVRAM(tt.modelID, tt.spec)
			if vram < tt.minVRAM || vram > tt.maxVRAM {
				t.Errorf("EstimateVRAM = %d, want range [%d, %d]", vram, tt.minVRAM, tt.maxVRAM)
			}
		})
	}
}
