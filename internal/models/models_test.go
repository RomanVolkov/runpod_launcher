package models

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCalculateSuitability(t *testing.T) {
	model := ModelSpec{Name: "test", MinVramGb: 50, ContextWindow: 8192}

	tests := []struct {
		name         string
		gpu          GPUInfo
		wantLevel    string
		wantScoreMin float64
		wantScoreMax float64
	}{
		{
			"green_high_headroom",
			GPUInfo{ID: "A100", Name: "A100-80GB", MemoryGb: 80},
			"green",
			0.99,
			1.0,
		},
		{
			"yellow_marginal",
			GPUInfo{ID: "A40", Name: "A40-48GB", MemoryGb: 55},
			"yellow",
			0.6,
			1.0,
		},
		{
			"red_insufficient",
			GPUInfo{ID: "T4", Name: "T4-16GB", MemoryGb: 16},
			"red",
			0.0,
			0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suitability := CalculateSuitability(tt.gpu, model)

			if suitability.Level != tt.wantLevel {
				t.Errorf("got level %q, want %q", suitability.Level, tt.wantLevel)
			}

			if suitability.SuitabilityScore < tt.wantScoreMin || suitability.SuitabilityScore > tt.wantScoreMax {
				t.Errorf("score %f not in range [%f, %f]", suitability.SuitabilityScore, tt.wantScoreMin, tt.wantScoreMax)
			}
		})
	}
}

func TestFilterGPUsByModel(t *testing.T) {
	model := ModelSpec{Name: "test", MinVramGb: 50, ContextWindow: 8192}
	gpus := []GPUInfo{
		{ID: "A100", Name: "A100", MemoryGb: 80},
		{ID: "A40", Name: "A40", MemoryGb: 48},
		{ID: "L40", Name: "L40", MemoryGb: 48},
		{ID: "RTX90", Name: "RTX90", MemoryGb: 24},
	}

	filtered := FilterGPUsByModel(gpus, model)

	if len(filtered) != 1 {
		t.Fatalf("expected 1 suitable GPU, got %d", len(filtered))
	}

	if filtered[0].ID != "A100" {
		t.Errorf("expected A100, got %s", filtered[0].ID)
	}
}

func TestRecommendGPUs(t *testing.T) {
	model := ModelSpec{Name: "test", MinVramGb: 50, ContextWindow: 8192}
	gpus := []GPUInfo{
		{ID: "A40", Name: "A40", MemoryGb: 48},
		{ID: "A100", Name: "A100", MemoryGb: 80},
		{ID: "H100", Name: "H100", MemoryGb: 80},
	}

	recommendations := RecommendGPUs(gpus, model)

	if len(recommendations) != 3 {
		t.Fatalf("expected 3 recommendations, got %d", len(recommendations))
	}

	if recommendations[0].Level != "green" {
		t.Errorf("first recommendation should be green, got %s", recommendations[0].Level)
	}

	if recommendations[len(recommendations)-1].Level != "red" {
		t.Errorf("last recommendation should be red, got %s", recommendations[len(recommendations)-1].Level)
	}

	for i := 0; i < len(recommendations)-1; i++ {
		if recommendations[i].SuitabilityScore < recommendations[i+1].SuitabilityScore {
			t.Errorf("scores not sorted descending at position %d", i)
		}
	}
}

func TestGetModelSpec_Default(t *testing.T) {
	spec, err := GetModelSpec("gemma4:31b", nil)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if spec.MinVramGb != 65 {
		t.Errorf("expected 65GB for gemma4:31b, got %d", spec.MinVramGb)
	}
}

func TestGetModelSpec_Override(t *testing.T) {
	overrides := map[string]ModelSpec{
		"qwen3.6:27b": {
			Name:          "qwen3.6:27b",
			MinVramGb:     80,
			ContextWindow: 64000,
		},
	}

	spec, err := GetModelSpec("qwen3.6:27b", overrides)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if spec.MinVramGb != 80 {
		t.Errorf("expected override value 80GB, got %d", spec.MinVramGb)
	}
}

func TestGetModelSpec_NotFound(t *testing.T) {
	_, err := GetModelSpec("nonexistent-model", nil)

	if err == nil {
		t.Fatal("expected error for nonexistent model")
	}
}

func TestModelsDevToSpec(t *testing.T) {
	dev := ModelsDev{
		ID:          "meta-llama/llama-3.1-70b",
		Name:        "Llama 3.1 70B",
		Family:      "llama-3.1",
		OpenWeights: true,
	}
	dev.Limit.Context = 131072
	dev.Limit.Output = 4096

	spec := modelsDevToSpec("meta-llama/llama-3.1-70b", dev)

	if spec.Name != "Llama 3.1 70B" {
		t.Errorf("expected name 'Llama 3.1 70B', got %q", spec.Name)
	}

	if spec.ContextWindow != 131072 {
		t.Errorf("expected context 131072, got %d", spec.ContextWindow)
	}

	if spec.MinVramGb < 50 || spec.MinVramGb > 200 {
		t.Errorf("expected VRAM in range [50, 200], got %d", spec.MinVramGb)
	}

	if spec.Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestListAvailableModels(t *testing.T) {
	overrides := map[string]ModelSpec{
		"custom:model": {Name: "custom:model"},
	}

	models := ListAvailableModels(overrides)

	if len(models) < len(DefaultModels) {
		t.Errorf("expected at least %d models, got %d", len(DefaultModels), len(models))
	}

	found := false
	for _, m := range models {
		if m == "custom:model" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custom model not in list")
	}
}

func TestOllamaClient_FetchModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"models": [
				{"name": "qwen3.5:27b", "modified_at": "2024-01-01T00:00:00Z", "size": 51000000000, "digest": "abc123"},
				{"name": "mistral:latest", "modified_at": "2024-01-02T00:00:00Z", "size": 26000000000, "digest": "def456"}
			]
		}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)
	models, err := client.FetchModels()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	if models[0].Name != "qwen3.5:27b" {
		t.Errorf("expected qwen3.5:27b, got %s", models[0].Name)
	}
}

func TestOllamaClient_FetchModels_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)
	_, err := client.FetchModels()

	if err == nil {
		t.Fatal("expected error from failed API call")
	}
}

func TestOllamaClient_GetModelNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"models": [
				{"name": "model1", "modified_at": "2024-01-01T00:00:00Z", "size": 1000, "digest": "abc"},
				{"name": "model2", "modified_at": "2024-01-01T00:00:00Z", "size": 2000, "digest": "def"}
			]
		}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)
	names, err := client.GetModelNames()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 model names, got %d", len(names))
	}

	if names[0] != "model1" || names[1] != "model2" {
		t.Errorf("unexpected model names: %v", names)
	}
}
