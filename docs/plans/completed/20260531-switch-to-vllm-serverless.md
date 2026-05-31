# Plan: Switch Serverless Endpoints to Official vLLM

## Overview

Replace the non-working Ollama serverless implementation with RunPod's official vLLM worker. This uses RunPod's pre-cached, optimized vLLM Docker image (`runpod/worker-v1-vllm`) with `Qwen/Qwen3.5-27B` model.

**Benefits:**
- ✅ Official RunPod recommendation with full support
- ✅ Pre-cached on all RunPod machines (instant startup)
- ✅ OpenAI-compatible API (works with OpenCode unchanged)
- ✅ Handler-based model (proper serverless architecture)
- ✅ Automatic scaling from 0→N workers
- ✅ No port mapping / socat / startup script complexity

**How it works:**
- Instead of exposing port 8000 with reverse proxies, use RunPod's handler infrastructure
- vLLM worker receives requests in RunPod format, processes them via inference engine, returns responses
- Endpoint URL: `https://api.runpod.ai/v2/{endpoint_id}/openai/v1` (same OpenAI format)

## Context

**Files involved:**
- Modify: `internal/serverless/serverless.go` (remove Ollama-specific code, simplify template creation)
- Modify: `cmd/runpod-launcher/serverless_up.go` (change image + environment variables)
- Modify: `internal/config/config.go` (vLLM-specific config fields)
- Modify: `internal/config/config.template.toml` (document vLLM settings)
- Keep unchanged: All pod-related code (Ollama for pods continues to work)

**What we're removing:**
- `isOllamaImage()` function
- Socat reverse proxy logic
- Docker entrypoint/startup command hacks
- OLLAMA_HOST, OLLAMA_MODEL environment variables

**What we're adding:**
- Direct reference to vLLM worker image
- vLLM environment variables: `MAX_MODEL_LEN`, `DTYPE`, `GPU_MEMORY_UTILIZATION`
- Qwen3.5-27B specific configuration

## Development Approach

- **testing approach**: No tests (per user request)
- Complete each task sequentially
- Commit after each task completion
- Update CLAUDE.md if new patterns discovered
- Move plan to `docs/plans/completed/` when finished

## Implementation Steps

### Task 1: Refactor serverless template creation for vLLM

**Files:**
- Modify: `internal/serverless/serverless.go`

- [x] Update `DefaultImageName` constant to `runpod/worker-v1-vllm:stable-cuda12.1.0`
- [x] Remove `isOllamaImage()` function entirely
- [x] Refactor `CreateTemplate()` to accept options struct with vLLM parameters: `MAX_MODEL_LEN`, `DTYPE`, `GPU_MEMORY_UTILIZATION`
  - Old signature: `CreateTemplate(name, imageName, modelName, apiKey string, containerDiskGB int)`
  - New signature: `CreateTemplate(name, imageName, modelName string, containerDiskGB int, opts *VLLMTemplateOptions)`
  - Struct: `type VLLMTemplateOptions struct { MaxModelLen int; Dtype string; GpuMemoryUtil float64 }`
- [x] Set only `MODEL_NAME` environment variable (remove Ollama-specific and incorrect HF_TOKEN mapping)
- [x] Remove all docker-entrypoint hacks and socat logic
- [x] Update `Client` interface to match new signature
- [x] Update all test mocks in `serverless_test.go` to use new interface (no new tests required, but existing must compile/pass)
- [x] Verify build: `go build -o runpod-launcher ./cmd/runpod-launcher/` (config field errors expected, will resolve in Task 2)

### Task 2: Update config to support vLLM serverless settings

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config.template.toml`

- [x] Add field to Config struct: `ServerlessMaxModelLen int` (optional, default 0)
- [x] Add field to Config struct: `ServerlessDtype string` (optional, default "bfloat16")
- [x] Add field to Config struct: `ServerlessGpuMemoryUtilization float64` (optional, default 0.85)
- [x] Update `config.template.toml` with documentation and examples for vLLM settings
- [x] Document recommended values: `max_model_len = 32000` for Qwen3.5-27B on H100
- [x] Verify parsing: `go vet ./internal/config && go build ./internal/config` ✓

### Task 3: Wire serverless_up.go to use vLLM config

**Files:**
- Modify: `cmd/runpod-launcher/serverless_up.go`
- Modify: `cmd/runpod-launcher/serverless_down_test.go`, `cmd/runpod-launcher/serverless_destroy_test.go` (update mocks)

- [x] Change default image to use new `DefaultImageName` constant (already updated in Task 1)
- [x] Update model name handling: use HuggingFace format `Qwen/Qwen3.5-27B` (not Ollama `modelname:tag`)
- [x] Wire vLLM config fields to template creation call (pass new vLLMTemplateOptions struct)
- [x] Remove OLLAMA_* and incorrect HF_TOKEN references from environment setup
- [x] Update output messages to reference vLLM and OpenAI compatibility
- [x] Update test mocks in `serverless_up_test.go` to use new CreateTemplate interface
- [x] Verify build: `go build -o runpod-launcher ./cmd/runpod-launcher/` ✓

### Task 4: Update documentation

**Files:**
- Modify: `CLAUDE.md`

- [x] Update package layout to include serverless package
- [x] Add "Serverless (vLLM)" section documenting:
  - RunPod's official vLLM worker architecture (handler-based, not HTTP port)
  - Deployment flow and automatic scaling
  - Environment variables: `MAX_MODEL_LEN`, `DTYPE`, `GPU_MEMORY_UTILIZATION` with recommended values
  - OpenAI API compatibility (same endpoint format as pod but different implementation)
  - Supported models: HuggingFace format only (e.g., `Qwen/Qwen3.5-27B`, `meta-llama/Llama-2-70b`)
- [x] Document pod vs serverless differences clearly
- [x] Update comments in `internal/serverless/serverless.go` explaining vLLM model format

### Task 5: Final verification and cleanup

- [ ] Run full build: `go build -o runpod-launcher ./cmd/runpod-launcher/`
- [ ] Run all tests: `go test ./...` (existing mocks should all compile/pass with new interface)
- [ ] Manually deploy serverless endpoint and verify:
  - Endpoint creates successfully
  - Model loads (Qwen/Qwen3.5-27B)
  - Can make curl request to endpoint URL
  - OpenCode connects and recognizes endpoint
- [ ] Move plan to `docs/plans/completed/`

## Configuration Example

After implementation, users will configure serverless in `~/.config/runpod-launcher/config.toml`:

```toml
# Serverless endpoint settings
serverless_image_name = "runpod/worker-v1-vllm:stable-cuda12.1.0"
serverless_endpoint_name = "llm-launcher-serverless"
serverless_workers_max = 3
serverless_gpu_type_id = "NVIDIA H100 80GB HBM3"
serverless_model_name = "Qwen/Qwen3.5-27B"

# vLLM specific configuration (all optional)
serverless_max_model_len = 32000      # Limits KV cache; Qwen3.5-27B default is 128K, 32K recommended for H100
serverless_dtype = "bfloat16"          # Model weight precision; bfloat16 = quality/speed balance
serverless_gpu_memory_utilization = 0.85  # Use 85% of available VRAM for model, reserve 15% for batch processing
```

Command usage:
```bash
./runpod-launcher serverless up
# Creates endpoint using vLLM worker, Qwen3.5-27B model
# Updates OpenCode config to point to endpoint
# Writes API key to ~/.env
```

OpenCode will automatically use the vLLM endpoint (OpenAI-compatible):
```
Provider: runpod-serverless
Base URL: https://api.runpod.ai/v2/{endpoint_id}/openai/v1
API Key: {env:RUNPOD_API_KEY}
Model: Qwen/Qwen3.5-27B
```

## Technical Notes

**Why vLLM over Ollama for serverless:**
- vLLM is handler-based (matches RunPod's serverless model)
- Ollama is HTTP-server-based (requires socat/port-mapping hacks)
- RunPod officially recommends vLLM for serverless LLM inference
- vLLM worker is pre-cached on RunPod machines

**Model choice: Qwen/Qwen3.5-27B:**
- Official HuggingFace repo: `Qwen/Qwen3.5-27B` ([https://huggingface.co/Qwen/Qwen3.5-27B](https://huggingface.co/Qwen/Qwen3.5-27B))
- Stable, production-ready model (no preview/experimental risks)
- 27B parameters fits well on H100 80GB GPU with bfloat16 precision
- Supports long context (default 128K, safe to limit to 32K on H100)
- Strong quality for cost, multilingual support
- User's current pod uses `qwen3.6:27b` (Ollama format) — serverless will use HuggingFace format

**Environment variables:**
- `MAX_MODEL_LEN`: Limits KV cache to prevent OOM on smaller models. Qwen3.5-27B default is 128K, but 32K recommended for H100
- `DTYPE`: bfloat16 balances quality and memory (float16 faster, float32 uses more VRAM)
- `GPU_MEMORY_UTILIZATION`: 0.85 allows room for batch processing while using most VRAM

## Post-Completion

**Manual testing (recommended):**
1. Deploy endpoint with `./runpod-launcher serverless up`
2. Wait for model to load (5-10 minutes)
3. Test with curl using vLLM endpoint URL
4. Verify OpenCode can connect to serverless endpoint
5. Test inference with a prompt

**Consuming projects:**
- None (this is internal to runpod-launcher)

**Documentation updates:**
- Update README.md with vLLM serverless section (explain difference from pod)
- Add example OpenCode configuration for serverless
