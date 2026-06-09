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

// DefaultModels contains hardcoded specs for popular Ollama models
var DefaultModels = map[string]ModelSpec{
	// Deepseek (Reasoning & Latest Gen)
	"deepseek-r1:latest": {
		Name:          "deepseek-r1:latest",
		MinVramGb:     140,
		ContextWindow: 131072,
		Description:   "DeepSeek R1 (Reasoning) - 140GB VRAM, 128K context",
	},
	"deepseek-v3:latest": {
		Name:          "deepseek-v3:latest",
		MinVramGb:     200,
		ContextWindow: 131072,
		Description:   "DeepSeek V3 (Latest) - 200GB VRAM, 128K context",
	},
	"deepseek-coder-v2:latest": {
		Name:          "deepseek-coder-v2:latest",
		MinVramGb:     100,
		ContextWindow: 131072,
		Description:   "DeepSeek Coder V2 - 100GB VRAM, 128K context",
	},

	// Llama (Meta - Most Popular)
	"llama2": {
		Name:          "llama2",
		MinVramGb:     35,
		ContextWindow: 4096,
		Description:   "Meta Llama 2 70B - 35GB VRAM, 4K context",
	},
	"llama3:latest": {
		Name:          "llama3:latest",
		MinVramGb:     40,
		ContextWindow: 8192,
		Description:   "Meta Llama 3 70B - 40GB VRAM, 8K context",
	},
	"llama3.1:latest": {
		Name:          "llama3.1:latest",
		MinVramGb:     45,
		ContextWindow: 131072,
		Description:   "Meta Llama 3.1 405B - 80GB VRAM, 128K context",
	},
	"llama3.2:latest": {
		Name:          "llama3.2:latest",
		MinVramGb:     40,
		ContextWindow: 131072,
		Description:   "Meta Llama 3.2 - 40GB VRAM, 128K context",
	},
	"llama3.3:latest": {
		Name:          "llama3.3:latest",
		MinVramGb:     50,
		ContextWindow: 131072,
		Description:   "Meta Llama 3.3 (Latest) - 50GB VRAM, 128K context",
	},

	// Gemma (Google - Efficient)
	"gemma:latest": {
		Name:          "gemma:latest",
		MinVramGb:     15,
		ContextWindow: 8192,
		Description:   "Google Gemma 7B - 15GB VRAM, 8K context",
	},
	"gemma2:latest": {
		Name:          "gemma2:latest",
		MinVramGb:     35,
		ContextWindow: 8192,
		Description:   "Google Gemma 2 27B - 35GB VRAM, 8K context",
	},
	"gemma3:latest": {
		Name:          "gemma3:latest",
		MinVramGb:     40,
		ContextWindow: 8192,
		Description:   "Google Gemma 3 - 40GB VRAM, 8K context",
	},
	"gemma4:latest": {
		Name:          "gemma4:latest",
		MinVramGb:     65,
		ContextWindow: 8192,
		Description:   "Google Gemma 4 31B - 65GB VRAM, 8K context",
	},

	// Qwen (Alibaba - Multilingual)
	"qwen:latest": {
		Name:          "qwen:latest",
		MinVramGb:     25,
		ContextWindow: 32768,
		Description:   "Alibaba Qwen - 25GB VRAM, 32K context",
	},
	"qwen2:latest": {
		Name:          "qwen2:latest",
		MinVramGb:     30,
		ContextWindow: 32768,
		Description:   "Alibaba Qwen 2 - 30GB VRAM, 32K context",
	},
	"qwen2.5:latest": {
		Name:          "qwen2.5:latest",
		MinVramGb:     35,
		ContextWindow: 131072,
		Description:   "Alibaba Qwen 2.5 (Latest) - 35GB VRAM, 128K context",
	},
	"qwen3:latest": {
		Name:          "qwen3:latest",
		MinVramGb:     40,
		ContextWindow: 131072,
		Description:   "Alibaba Qwen 3 - 40GB VRAM, 128K context",
	},
	"qwen3.6:latest": {
		Name:          "qwen3.6:latest",
		MinVramGb:     70,
		ContextWindow: 131072,
		Description:   "Alibaba Qwen 3.6 27B - 70GB VRAM, 128K context",
	},

	// Mistral (European - Fast)
	"mistral:latest": {
		Name:          "mistral:latest",
		MinVramGb:     15,
		ContextWindow: 32768,
		Description:   "Mistral 7B - 15GB VRAM, 32K context",
	},
	"mistral-small:latest": {
		Name:          "mistral-small:latest",
		MinVramGb:     20,
		ContextWindow: 32768,
		Description:   "Mistral Small - 20GB VRAM, 32K context",
	},
	"mistral-medium-3.5:latest": {
		Name:          "mistral-medium-3.5:latest",
		MinVramGb:     40,
		ContextWindow: 131072,
		Description:   "Mistral Medium 3.5 - 40GB VRAM, 128K context",
	},
	"mistral-large-3:latest": {
		Name:          "mistral-large-3:latest",
		MinVramGb:     90,
		ContextWindow: 131072,
		Description:   "Mistral Large 3 123B - 90GB VRAM, 128K context",
	},

	// CodeLlama (Code-Focused)
	"codellama:latest": {
		Name:          "codellama:latest",
		MinVramGb:     35,
		ContextWindow: 16384,
		Description:   "CodeLlama 70B - 35GB VRAM, 16K context",
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
