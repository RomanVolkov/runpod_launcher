# models.dev Integration

## Overview

✅ **COMPLETED** — models.dev is now integrated as a fallback model specs source.

models.dev provides a comprehensive, provider-agnostic catalog of AI models with rich metadata including:
- Context window limits (critical for GPU selection)
- Input/output modality information  
- Benchmark scores
- Open weights URLs
- Model families and capabilities
- Knowledge cutoff dates

## Current Architecture vs models.dev

### Current (Ollama-based):
1. **Ollama API** → fetches available models from local instance
2. **Hardcoded fallback** → qwen, gemma, mistral, llama with estimated VRAM
3. **Config overrides** → allow custom specs

**Strengths:**
- Works offline (after initial fetch)
- Simple, focused on model availability
- No external API dependency

**Limitations:**
- Only shows models installed on local Ollama
- Limited metadata (model name only from API)
- Manual spec maintenance for new models

### Implemented (models.dev-enhanced):

```
┌─ User requests model (config or CLI)
├─ Check config overrides (highest priority)
├─ Check hardcoded defaults (for common Ollama models)
├─ Query models.dev API (cached, 5-min TTL)
│  ├─ Get context window from "limit.context"
│  ├─ Extract parameter count from model name (70b, 7b, etc.)
│  ├─ Estimate VRAM using formula (params × dtype + KV cache + working space)
│  └─ Cache response locally (TTL-based validity)
└─ Return error if model not found
```

## Data Structure from models.dev

```json
{
  "id": "meta-llama/llama-3.1-70b",
  "name": "Llama 3.1 70B",
  "family": "llama-3.1",
  "limit": {
    "context": 131072,      // ← Use this for context window
    "output": 4096
  },
  "open_weights": true,      // ← Can run locally
  "modalities": {
    "input": ["text"],
    "output": ["text"]
  },
  "release_date": "2024-07-23",
  "knowledge": "2024-06"
}
```

## Implementation Details

### What Was Implemented

**Files Created:**
- `internal/models/modelsdev.go` — ModelsDevClient with HTTP and caching
- `internal/models/modelsdev_test.go` — Comprehensive test suite

**Core Components:**

1. **ModelsDevClient** — HTTP client with built-in caching
   - `FetchAllModels()` — fetches entire catalog from https://models.dev/models.json
   - `GetModel(modelID)` — retrieves single model (with cache-first strategy)
   - Configurable `baseURL` for testing with mock servers
   - Thread-safe operations via `sync.RWMutex`

2. **ModelsDevCache** — TTL-based cache with automatic expiration
   - Configurable TTL (default: 5 minutes in production)
   - Checks validity before returning cached data
   - Thread-safe reads and writes

3. **EstimateVRAM()** — VRAM estimation from model specs
   - Extracts parameter count from model name (patterns: 70b, 7b, 405b, etc.)
   - Formula: `params × 2 bytes (bfloat16) + KV cache overhead + 15GB working space`
   - Bounds: min 20GB, max 300GB
   - Fallback: assumes 13B if parameters can't be extracted

4. **Integration in GetModelSpec()** — Fallback chain
   - Priority 1: Config overrides
   - Priority 2: Hardcoded defaults (for Ollama models)
   - Priority 3: models.dev API (with caching)
   - Returns ModelSpec with estimated VRAM and context window

**VRAM Estimation Details:**

We extract parameter count from model name patterns (e.g., "70b" → 70 billion parameters):
```go
// Extraction patterns supported: 405b, 200b, 120b, 70b, 34b, 27b, 13b, 7b, 3.5b, 3b, 2b, 1b
paramCount := extractParameterCount(modelID)

// Estimation formula:
// model_weights = paramCount × 2 bytes (for bfloat16)
// kv_cache = context_size × param_count / 1000000 (rough heuristic)
// working_space = 15 GB
// total = max(20GB, min(300GB, model_weights + kv_cache + working_space))
```

**Example (Llama 3.1 70B, bfloat16, 131K context):**
- Model weights: 70 × 2 = 140 GB
- KV cache overhead: 131072 × 70 / 1000000 ≈ 9.2 GB
- Working space: 15 GB
- **Total: ~164 GB** (fits on H100 80GB with quantization)

## Benefits

1. **Real-time model metadata** — Always current, no manual updates
2. **Rich capabilities** — Know if model supports reasoning, tool calls, etc.
3. **Cross-platform** — Models.dev covers 100+ providers, not just Ollama
4. **Benchmarks** — Help users choose right model for their task
5. **Backward compatible** — Ollama + hardcoded fallbacks still work

## Tradeoffs

| Aspect | Benefit | Tradeoff |
|--------|---------|----------|
| Completeness | 100K+ models vs 10s in Ollama | Network call required |
| Accuracy | Real-time model info | Needs caching strategy |
| VRAM estimation | Data-driven | Still requires formula tuning |
| Offline operation | Works without network | Falls back to cached data |

## Usage in Code

The integration is transparent to callers — `GetModelSpec()` now automatically falls back to models.dev:

```go
import "github.com/romanvolkov/runpod-launcher/internal/models"

// This will try: overrides → hardcoded defaults → models.dev API
spec, err := models.GetModelSpec("meta-llama/llama-3.1-70b", overrides)
if err != nil {
    log.Fatal(err)  // Model not found in any source
}

fmt.Printf("Model: %s\n", spec.Name)
fmt.Printf("Min VRAM: %d GB\n", spec.MinVramGb)
fmt.Printf("Context: %d tokens\n", spec.ContextWindow)

// GPU filtering works with real context requirements
gpu := models.GPUInfo{Name: "NVIDIA H100", MemoryGb: 80}
suitability := models.CalculateSuitability(gpu, spec)
if suitability.Level == "green" {
    fmt.Println("GPU is suitable for this model")
}
```

## Configuration

Currently, models.dev integration is automatic with no configuration needed. The cache TTL is hardcoded to 5 minutes.

**Future enhancements (if needed):**
- Add `models_dev_cache_ttl` config option
- Add flag to disable models.dev queries
- Add environment variable override for base URL

## References

- models.dev API: https://models.dev/models.json
- Provider catalog: https://models.dev/catalog.json
- All endpoints: https://models.dev/api.json
