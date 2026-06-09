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

// DefaultModels contains hardcoded specs for common models (verified Ollama IDs)
var DefaultModels = map[string]ModelSpec{
	// Gemma models (verified from Ollama library)
	"gemma:latest": {
		Name:          "gemma:latest",
		MinVramGb:     20,
		ContextWindow: 8192,
		Description:   "Google Gemma - 20GB VRAM, 8K context",
	},
	"gemma2:latest": {
		Name:          "gemma2:latest",
		MinVramGb:     35,
		ContextWindow: 8192,
		Description:   "Google Gemma 2 - 35GB VRAM, 8K context",
	},
	"gemma3:27b": {
		Name:          "gemma3:27b",
		MinVramGb:     50,
		ContextWindow: 8192,
		Description:   "Google Gemma 3 27B - 50GB VRAM, 8K context",
	},
	"gemma4:31b": {
		Name:          "gemma4:31b",
		MinVramGb:     65,
		ContextWindow: 8192,
		Description:   "Google Gemma 4 31B - 65GB VRAM, 8K context",
	},

	// Qwen models (verified from Ollama library)
	"qwen:latest": {
		Name:          "qwen:latest",
		MinVramGb:     25,
		ContextWindow: 32768,
		Description:   "Alibaba Qwen - 25GB VRAM, 32K context",
	},
	"qwen2.5:latest": {
		Name:          "qwen2.5:latest",
		MinVramGb:     30,
		ContextWindow: 32768,
		Description:   "Alibaba Qwen 2.5 - 30GB VRAM, 32K context",
	},

	// Kimi models (verified from Ollama library - Chinese LLM)
	"kimi-k2.5": {
		Name:          "kimi-k2.5",
		MinVramGb:     50,
		ContextWindow: 32768,
		Description:   "Moonshot Kimi K2.5 - 50GB VRAM, 32K context",
	},
	"kimi-k2.6": {
		Name:          "kimi-k2.6",
		MinVramGb:     55,
		ContextWindow: 32768,
		Description:   "Moonshot Kimi K2.6 - 55GB VRAM, 32K context",
	},

	// Mistral models (verified from Ollama library)
	"mistral:latest": {
		Name:          "mistral:latest",
		MinVramGb:     20,
		ContextWindow: 32768,
		Description:   "Mistral AI - 20GB VRAM, 32K context",
	},

	// Llama models (verified from Ollama library)
	"llama2": {
		Name:          "llama2",
		MinVramGb:     35,
		ContextWindow: 4096,
		Description:   "Meta Llama 2 - 35GB VRAM, 4K context",
	},
	"llama3:latest": {
		Name:          "llama3:latest",
		MinVramGb:     40,
		ContextWindow: 8192,
		Description:   "Meta Llama 3 - 40GB VRAM, 8K context",
	},
	"llama3.1:latest": {
		Name:          "llama3.1:latest",
		MinVramGb:     45,
		ContextWindow: 131072,
		Description:   "Meta Llama 3.1 - 45GB VRAM, 128K context",
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
