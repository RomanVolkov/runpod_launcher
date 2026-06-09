# models.dev Integration Proposal

## Overview

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

### Proposed (models.dev-enhanced):

```
┌─ User requests model (config or CLI)
├─ Check config overrides
├─ Query models.dev API (cached, 10s TTL)
│  ├─ Get context window from "limit.context"
│  ├─ Get model capabilities (reasoning, tool_call)
│  ├─ Get benchmarks and release info
│  └─ Cache response locally
├─ Fall back to Ollama API if available
├─ Fall back to hardcoded defaults
└─ Estimate VRAM (formula based on parameters)
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

## Implementation Strategy

### Phase 1: Research (Current)
- ✅ Identified models.dev API endpoints
- ✅ Analyzed data structure and available fields
- ✅ Confirmed rich model metadata available

### Phase 2: Integration (Optional)
Would add to `internal/models/`:
- `modelsdev.go` — HTTP client + caching (5-10 min TTL)
- Model lookup: `GetModelSpecsFromModelsDev(name string)`
- Priority chain: config overrides → models.dev → Ollama → hardcoded defaults

### Phase 3: VRAM Estimation
Models.dev doesn't provide model size (parameters) directly, but we could:
1. Extract from model name patterns (e.g., "70b" → 70 billion parameters)
2. Use benchmarks as a proxy
3. Maintain a parameter size mapping table

**VRAM Formula:**
```
VRAM ≈ (parameters × dtype_bytes) + KV_cache_overhead + working_space

Example (Llama 3.1 70B, bfloat16):
- Weights: 70B × 2 bytes = 140 GB
- KV cache (131K context): ~70 GB  
- Working space: ~10 GB
- Total: ~220 GB (fits on H100 80GB with quantization)
```

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

## Example Integration

```go
// Future API
spec, err := models.GetModelSpecsEnhanced(ctx, "meta-llama/llama-3.1-70b", cfg)
// Returns context: 131072, estimated VRAM: 220GB (with fallback to hardcoded)

// GPU filtering now knows exact context requirements
suitability := models.CalculateSuitability(gpu, spec)
// Green if VRAM ≥ estimated + 10GB headroom AND context supported
```

## Recommendation

**For now:** Keep Ollama-based approach (proven, working)
**Future:** Add models.dev as optional enhancement layer via flag or config setting

```toml
# config.toml (future)
[model_specs]
primary_source = "ollama"  # or "models-dev" or "hybrid"
models_dev_cache_ttl = 300  # 5 minutes
```

## References

- models.dev API: https://models.dev/models.json
- Provider catalog: https://models.dev/catalog.json
- All endpoints: https://models.dev/api.json
