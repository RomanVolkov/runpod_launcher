package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ModelsDev represents a model from models.dev API
type ModelsDev struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Family      string `json:"family"`
	OpenWeights bool   `json:"open_weights"`
	Limit       struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	ReleaseDate string `json:"release_date"`
	Knowledge   string `json:"knowledge"`
}

// ModelsDevCache caches the models.dev API response
type ModelsDevCache struct {
	data      map[string]ModelsDev
	timestamp time.Time
	ttl       time.Duration
	mu        sync.RWMutex
}

// NewModelsDevCache creates a new cache with specified TTL
func NewModelsDevCache(ttl time.Duration) *ModelsDevCache {
	return &ModelsDevCache{
		data: make(map[string]ModelsDev),
		ttl:  ttl,
	}
}

// Get retrieves a model from cache if valid
func (c *ModelsDevCache) Get(modelID string) (ModelsDev, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Since(c.timestamp) > c.ttl {
		return ModelsDev{}, false
	}

	model, ok := c.data[modelID]
	return model, ok
}

// Set stores the model data in cache
func (c *ModelsDevCache) Set(data map[string]ModelsDev) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = data
	c.timestamp = time.Now()
}

// IsValid checks if cache is still valid
func (c *ModelsDevCache) IsValid() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return time.Since(c.timestamp) <= c.ttl && len(c.data) > 0
}

// ModelsDevClient fetches model metadata from models.dev API
type ModelsDevClient struct {
	httpClient *http.Client
	cache      *ModelsDevCache
	baseURL    string
}

// NewModelsDevClient creates a new models.dev client with caching
func NewModelsDevClient(cacheTTL time.Duration) *ModelsDevClient {
	return &ModelsDevClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cache:      NewModelsDevCache(cacheTTL),
		baseURL:    "https://models.dev",
	}
}

// FetchAllModels retrieves all models from models.dev API
func (c *ModelsDevClient) FetchAllModels() (map[string]ModelsDev, error) {
	// Check cache first
	if c.cache.IsValid() {
		c.cache.mu.RLock()
		defer c.cache.mu.RUnlock()
		return c.cache.data, nil
	}

	url := c.baseURL + "/models.json"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models.dev: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("models.dev API error: status %d: %s", resp.StatusCode, string(body))
	}

	var models map[string]ModelsDev
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("failed to parse models.dev response: %w", err)
	}

	// Store in cache
	c.cache.Set(models)

	return models, nil
}

// GetModel retrieves a single model by ID
func (c *ModelsDevClient) GetModel(modelID string) (ModelsDev, error) {
	// Try cache first
	if model, ok := c.cache.Get(modelID); ok {
		return model, nil
	}

	// Fetch all models if cache miss
	models, err := c.FetchAllModels()
	if err != nil {
		return ModelsDev{}, err
	}

	if model, ok := models[modelID]; ok {
		return model, nil
	}

	return ModelsDev{}, fmt.Errorf("model not found in models.dev: %s", modelID)
}

// EstimateVRAM estimates VRAM requirements based on model specs
// This is a heuristic - actual requirements depend on quantization and implementation
func EstimateVRAM(modelID string, spec ModelsDev) int {
	// Extract parameter count from model name if possible
	// Examples: "70b" in "llama-3.1-70b", "27b" in "qwen-27b"
	paramCount := extractParameterCount(modelID)
	if paramCount == 0 {
		paramCount = extractParameterCount(spec.Name)
	}

	if paramCount == 0 {
		// Fallback: assume medium model (13B parameters)
		paramCount = 13
	}

	// VRAM estimation formula (in GB):
	// Full precision (float32): params * 4 bytes
	// bfloat16/float16: params * 2 bytes
	// Quantized (int8): params * 1 byte
	// KV cache overhead: context_size * param_count (rough estimate)
	// Working space: 10-20GB

	// Conservative estimate for bfloat16
	// paramCount is in billions, so multiply by 2 directly for GB
	modelWeights := paramCount * 2
	kvCacheOverhead := spec.Limit.Context * paramCount / 1000000 // GB (rough)
	workingSpace := 15 // GB

	estimatedVRAM := modelWeights + kvCacheOverhead + workingSpace

	// Min 20GB, max 300GB
	if estimatedVRAM < 20 {
		estimatedVRAM = 20
	}
	if estimatedVRAM > 300 {
		estimatedVRAM = 300
	}

	return estimatedVRAM
}

// extractParameterCount tries to extract parameter count from model name
// Examples: "70b" -> 70, "3.5b" -> 3, "405b" -> 405
func extractParameterCount(name string) int {
	// Simple heuristic: look for patterns like "70b", "27b", "3.5b"
	// This is a simplified version - a full parser would use regex
	for _, substr := range []string{"405b", "200b", "120b", "70b", "34b", "27b", "13b", "7b", "3.5b", "3b", "2b", "1b"} {
		if contains(name, substr) {
			// Extract just the number part
			if substr == "405b" {
				return 405
			} else if substr == "200b" {
				return 200
			} else if substr == "120b" {
				return 120
			} else if substr == "70b" {
				return 70
			} else if substr == "34b" {
				return 34
			} else if substr == "27b" {
				return 27
			} else if substr == "13b" {
				return 13
			} else if substr == "7b" {
				return 7
			} else if substr == "3.5b" {
				return 3
			} else if substr == "3b" {
				return 3
			} else if substr == "2b" {
				return 2
			} else if substr == "1b" {
				return 1
			}
		}
	}
	return 0
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
