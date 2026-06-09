# Model Selection & GPU Intelligence

## Overview

Improve RunPod Launcher reliability and user experience by:
1. **Switching to GraphQL API** for direct GPU/pricing queries (replacing unreliable runpodctl JSON parsing)
2. **Adding model selection** during pod creation with specs from Ollama API
3. **Smart GPU filtering** - highlight/recommend GPUs based on selected model's requirements

This solves the core problem: GPU selection often fails because availability data is stale or incorrect. By using RunPod's GraphQL API directly, we get real-time accurate data. Model selection from Ollama API eliminates manual GPU hunting—the app automatically recommends suitable GPUs for the chosen model.

## Context

**Files/Components Involved:**
- `internal/pod/pod.go` - PodClient interface + RunPodClient implementation
- `cmd/runpod-launcher/up.go` - Pod creation flow
- `cmd/runpod-launcher/availability.go` - GPU listing
- `internal/config/config.go` + `config.template.toml` - Configuration
- **New:** `internal/graphql/graphql.go` - GraphQL client
- **New:** `internal/models/models.go` - Model specs and filtering logic

**Related Patterns Found:**
- runpodctl uses GraphQL for queries (https://github.com/runpod/runpodctl/blob/main/internal/api/graphql.go)
- GraphQL queries available: `gpuTypes`, `lowestPrice`, `podFindAndDeployOnDemand`
- Current code parses runpodctl CLI output (error-prone, stale data)

**Dependencies Identified:**
- RunPod GraphQL API: `https://api.runpod.io/graphql`
- Model specs source: **Ollama API** (fetch from local/remote Ollama instance)
- Fallback: Hardcoded defaults for common models (qwen, gemma, mistral)
- Config file: Optional overrides for custom models
- TTL requires pod deletion/termination logic

## Development Approach

- **testing approach**: Minimal (code-first, basic unit tests)
- Complete each task fully before moving to next
- Make small, focused changes
- Run tests after each change (when relevant)
- Maintain backward compatibility with existing config

## Progress Tracking

- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with ➕ prefix
- Document issues/blockers with ⚠️ prefix
- Update plan if scope changes

## What Goes Where

- **Implementation Steps** (`[ ]` checkboxes): Code changes, tests, documentation
- **Post-Completion** (no checkboxes): Manual testing, verification items

## Implementation Steps

### Task 1: Create GraphQL client for direct RunPod API access

**Files:**
- Create: `internal/graphql/graphql.go`
- Create: `internal/graphql/types.go`

- [x] Create GraphQL client struct with Query() method (bearer token auth)
- [x] Implement GPU types query: retrieve available GPUs with pricing
- [x] Implement lowest price query: query GPU pricing with filters (region, CUDA, etc.)
- [x] Add error handling for GraphQL errors and network failures
- [x] Write basic tests for GraphQL client queries (mock HTTP responses)
- [x] run tests - must pass before next task

### Task 2: Add model specs and GPU filtering logic with Ollama API support

**Files:**
- Create: `internal/models/models.go`
- Create: `internal/models/models_test.go`
- Create: `internal/models/ollama.go` (Ollama API client)
- Modify: `internal/config/config.go`
- Modify: `internal/config/config.template.toml`

- [x] Define ModelSpec struct: name, minVramGb, contextWindow, description
- [x] Create Ollama API client: fetch model list from local/remote Ollama instance
- [x] Implement GetModelSpecs() function: query Ollama API for model metadata + fallback to hardcoded defaults
- [x] Create FilterGPUsByModel() function: returns GPUs suitable for model + context
- [x] Create RecommendGPUs() function: rank GPUs by suitability score
- [x] Add hardcoded model specs for common models (qwen, gemma, mistral, llama)
- [x] Add optional model_specs_override section to config (allow custom VRAM/context)
- [x] Write basic tests for model filtering, GPU recommendation, and Ollama API fallback
- [x] run tests - must pass before next task

### Task 3: Integrate GraphQL into GPU availability command

**Files:**
- Modify: `cmd/runpod-launcher/availability.go`
- Modify: `internal/pod/pod.go` - extend PodClient interface

- [x] Replace runpodctl CLI calls with GraphQL queries in availability command
- [x] Update GPU display: show pricing from GraphQL (no more $0.0000)
- [x] Add model filter option to availability command (--model flag)
- [x] When model specified, highlight suitable GPUs
- [x] Update availability output format to show model compatibility
- [x] run tests - must pass before next task

### Task 4: Enhance `up` command with model selection and GPU recommendations

**Files:**
- Modify: `cmd/runpod-launcher/up.go`
- Modify: `cmd/runpod-launcher/gpu_selector.go`
- Modify: `cmd/runpod-launcher/gpu_selector_tui.go`

- [x] Add model selection to `up` flow (before GPU selection)
- [x] If no model specified, prompt user to select from config models
- [x] Use GraphQL to fetch real-time GPU list (instead of runpodctl)
- [x] Pre-filter GPUs based on selected model's requirements
- [ ] Show GPU suitability score in TUI (green=good fit, yellow=marginal, red=not suitable)
- [ ] Update TUI to display "Recommended for [model]" label on suitable GPUs
- [x] run tests - must pass before next task

### Task 5: Replace runpodctl with GraphQL in pod creation flow

**Files:**
- Modify: `internal/pod/pod.go` - CreatePod implementation
- Modify: Go module dependencies if needed

- [x] Add `podFindAndDeployOnDemand` GraphQL mutation to GraphQL client
- [x] Update CreatePod() to use GraphQL instead of runpodctl CLI
- [x] Maintain compatibility: still accept environment variables
- [x] Test pod creation with actual RunPod API
- [x] run tests - must pass before next task

### Task 6: Improve GPU validation with real-time availability

**Files:**
- Modify: `cmd/runpod-launcher/up.go` - validateGPUAvailable function
- Modify: `internal/graphql/graphql.go` - add real-time capacity query

- [ ] Replace MaxGpuCount checks with GraphQL "available" field
- [ ] Implement stricter validation: check capacity immediately before pod creation
- [ ] Add retry logic: if GPU unavailable, suggest alternatives
- [ ] Display alternative GPU recommendations if selected GPU is unavailable
- [ ] run tests - must pass before next task

### Task 7: Update documentation and configuration

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/20260608-graphql-gpu-models.md` (this file)

- [ ] Document model selection in README with examples
- [ ] Document model specs fetching from Ollama API
- [ ] Document GPU filtering logic and suitability scores
- [ ] Update CLAUDE.md if new patterns discovered
- [ ] Move this plan to `docs/plans/completed/`

### Task 8: Verify acceptance criteria

- [ ] Verify GPU availability is accurate (test with 5+ different GPUs)
- [ ] Verify model selection works during `up` command (test with qwen, gemma)
- [ ] Verify GPU filtering correctly recommends suitable GPUs for each model
- [ ] Verify Ollama API integration (fetch model specs, fallback to hardcoded)
- [ ] Verify GraphQL queries return accurate pricing data
- [ ] Run full test suite: `go test ./...`
- [ ] Test with actual RunPod deployment (small pod, verify end-to-end)

## Technical Details

### Model Specs Strategy

**Primary**: Query Ollama API (if running locally or accessible)
```bash
# Query running Ollama instance for models
curl http://localhost:11434/api/tags
# Returns: {"models": [{"name": "qwen3.6:27b", ...}]}
```

**Fallback**: Hardcoded defaults if Ollama unreachable
- Keeps basic functionality without external dependency
- User can override with config file

**Config Override**: Optional `model_specs_override` section in config.toml

### GraphQL Queries to Implement

1. **GPU Types Query** (for availability command):
```graphql
query {
  gpuTypes {
    id
    displayName
    memoryInGb
    secureCloud
    communityCloud
  }
}
```

2. **Lowest Price Query** (for pricing):
```graphql
query LowestPrice($input: GpuLowestPriceInput!) {
  gpuTypes {
    lowestPrice(input: $input) {
      gpuTypeId
      gpuName
      minimumBidPrice
      uninterruptablePrice
      minMemory
    }
  }
}
```

3. **Pod Deploy Mutation** (for pod creation):
```graphql
mutation PodFindAndDeployOnDemand($input: PodFindAndDeployOnDemandInput!) {
  podFindAndDeployOnDemand(input: $input) {
    id
    name
    desiredStatus
  }
}
```

### Model Specs Format (config.toml)

**Optional Overrides Only** (primary source is Ollama API):

```toml
# Optional: Override specs for specific models pulled from Ollama
[model_specs_override.qwen3_6_27b]
min_vram_gb = 75
context_window = 131072

[model_specs_override.gemma4]
min_vram_gb = 40
context_window = 262144
```

**Hardcoded Defaults** (when Ollama unavailable):
```
qwen3.6:27b → 75 GB, 128K context
gemma:4     → 40 GB, 256K context
mistral     → 20 GB, 32K context
llama3:70b  → 80 GB, 8K context
```

### GPU Suitability Scoring

- **Green (Suitable)**: GPU VRAM ≥ minVramGb + 10GB headroom
- **Yellow (Marginal)**: GPU VRAM ≥ minVramGb but < 10GB headroom
- **Red (Not Suitable)**: GPU VRAM < minVramGb

## Post-Completion

**Manual verification:**
- Test GPU selection with 10+ different GPU types
- Verify pricing data matches RunPod console
- Test TTL auto-shutdown with actual pods
- Verify model selection works with custom models in config
- Test with both Secure and Community cloud GPUs
- Verify fallback behavior when selected GPU becomes unavailable

**External system updates:**
- None required - uses existing RunPod GraphQL API
- Optional: Update runpod-launcher docs with new features
