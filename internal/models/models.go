package models

import (
	"fmt"
	"sort"
	"time"
)

var modelsDevClient = NewModelsDevClient(5 * time.Minute)

// ModelSpec represents specifications for an LLM model
type ModelSpec struct {
	Name          string `toml:"name"`
	MinVramGb     int    `toml:"min_vram_gb"`
	ContextWindow int    `toml:"context_window"`
	Description   string `toml:"description"`
}

// GPUInfo represents GPU information with memory
type GPUInfo struct {
	ID        string
	Name      string
	MemoryGb  int
	PricePerHr float64
}

// GPUSuitability represents GPU suitability for a model
type GPUSuitability struct {
	GPU           GPUInfo
	SuitabilityScore float64 // 0.0 to 1.0
	Level         string    // "green", "yellow", "red"
}

// DefaultModels contains hardcoded specs for common models
var DefaultModels = map[string]ModelSpec{
	"qwen3.6:27b": {
		Name:          "qwen3.6:27b",
		MinVramGb:     70,
		ContextWindow: 131072,
		Description:   "Qwen3.6 27B - 70GB VRAM, 128K context",
	},
	"qwen3.5:27b": {
		Name:          "qwen3.5:27b",
		MinVramGb:     55,
		ContextWindow: 32768,
		Description:   "Qwen3.5 27B - 55GB VRAM, 32K context",
	},
	"gemma:4": {
		Name:          "gemma:4",
		MinVramGb:     40,
		ContextWindow: 262144,
		Description:   "Gemma 4 - 40GB VRAM, 256K context",
	},
	"gemma2:27b": {
		Name:          "gemma2:27b",
		MinVramGb:     50,
		ContextWindow: 8192,
		Description:   "Gemma2 27B - 50GB VRAM, 8K context",
	},
	"mistral:latest": {
		Name:          "mistral:latest",
		MinVramGb:     20,
		ContextWindow: 32768,
		Description:   "Mistral - 20GB VRAM, 32K context",
	},
	"llama3:70b": {
		Name:          "llama3:70b",
		MinVramGb:     75,
		ContextWindow: 8192,
		Description:   "Llama3 70B - 75GB VRAM, 8K context",
	},
}

// CalculateSuitability determines GPU suitability for a model
func CalculateSuitability(gpu GPUInfo, model ModelSpec) GPUSuitability {
	headroom := gpu.MemoryGb - model.MinVramGb
	var score float64
	var level string

	if headroom >= 10 {
		score = 1.0
		level = "green"
	} else if headroom >= 0 {
		score = 0.6 + (float64(headroom) / 10.0 * 0.4)
		level = "yellow"
	} else {
		score = 0.0
		level = "red"
	}

	return GPUSuitability{
		GPU:                gpu,
		SuitabilityScore: score,
		Level:            level,
	}
}

// FilterGPUsByModel returns GPUs suitable for a model (score > 0)
func FilterGPUsByModel(gpus []GPUInfo, model ModelSpec) []GPUInfo {
	var suitable []GPUInfo
	for _, gpu := range gpus {
		if gpu.MemoryGb >= model.MinVramGb {
			suitable = append(suitable, gpu)
		}
	}
	return suitable
}

// RecommendGPUs ranks GPUs by suitability score (highest first)
func RecommendGPUs(gpus []GPUInfo, model ModelSpec) []GPUSuitability {
	var recommendations []GPUSuitability
	for _, gpu := range gpus {
		rec := CalculateSuitability(gpu, model)
		recommendations = append(recommendations, rec)
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].SuitabilityScore > recommendations[j].SuitabilityScore
	})

	return recommendations
}

// ValidateModel checks if a model name is known (either hardcoded or in override specs)
func ValidateModel(modelName string, overrides map[string]ModelSpec) error {
	if _, ok := DefaultModels[modelName]; ok {
		return nil
	}
	if _, ok := overrides[modelName]; ok {
		return nil
	}
	return fmt.Errorf("unknown model: %s", modelName)
}

// GetModelSpec retrieves model specs with fallback chain:
// 1. Config overrides
// 2. models.dev API (with caching)
// 3. Hardcoded defaults
func GetModelSpec(modelName string, overrides map[string]ModelSpec) (ModelSpec, error) {
	if spec, ok := overrides[modelName]; ok {
		return spec, nil
	}
	if spec, ok := DefaultModels[modelName]; ok {
		return spec, nil
	}

	// Try models.dev API (non-blocking fallback)
	modelsDev, err := modelsDevClient.GetModel(modelName)
	if err == nil {
		return modelsDevToSpec(modelName, modelsDev), nil
	}

	return ModelSpec{}, fmt.Errorf("model not found: %s", modelName)
}

// modelsDevToSpec converts a ModelsDev response to ModelSpec format
func modelsDevToSpec(modelID string, dev ModelsDev) ModelSpec {
	vramEst := EstimateVRAM(modelID, dev)
	return ModelSpec{
		Name:          dev.Name,
		MinVramGb:     vramEst,
		ContextWindow: dev.Limit.Context,
		Description:   fmt.Sprintf("%s (context: %d, open weights: %v)", dev.Name, dev.Limit.Context, dev.OpenWeights),
	}
}

// ListAvailableModels returns all available model names
func ListAvailableModels(overrides map[string]ModelSpec) []string {
	seen := make(map[string]bool)
	var models []string

	for name := range DefaultModels {
		models = append(models, name)
		seen[name] = true
	}

	for name := range overrides {
		if !seen[name] {
			models = append(models, name)
		}
	}

	sort.Strings(models)
	return models
}
