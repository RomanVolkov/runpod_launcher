# RunPod Launcher

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://golang.org/)
[![Cobra CLI](https://img.shields.io/badge/Cobra-CLI-FF6B6B?logo=github&logoColor=white)](https://cobra.dev/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://www.docker.com/)
[![Ollama](https://img.shields.io/badge/Ollama-000000?logo=ollama&logoColor=white)](https://ollama.ai/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
## Overview

**RunPod Launcher** is a command-line tool that simplifies spinning up GPU pods on RunPod to serve large language models (LLMs) using Ollama. No more manual pod creation, model downloads, or complex configurations—just a single command to deploy a fully functional inference server.

Built in Go with the Cobra CLI framework, this tool automates the entire workflow: pod creation, interactive GPU selection with real-time availability checking, model loading, API key generation, and OpenCode integration. It provides a simple, reliable way to deploy models like Gemma 4, Qwen, Mistral, and others on RunPod's distributed GPU infrastructure with an intuitive terminal UI for GPU browsing and selection.

**Perfect for:**
- ML engineers who want quick LLM serving without DevOps complexity
- Researchers testing different models on different hardware
- Development teams needing temporary inference endpoints
- Anyone who values privacy over SaaS AI agents

## Quick Start

### Prerequisites

- Go 1.21 or later
- RunPod account with API key
- A RunPod API key ([get yours here](https://console.runpod.io/user/settings))

### Installation

```bash
# Clone the repository
git clone https://github.com/romanvolkov/runpod-launcher.git
cd runpod_orchestrator

# Build the binary
go build -o runpod-launcher ./cmd/runpod-launcher/

# Install globally (optional)
go install ./cmd/runpod-launcher/
```

### Basic Usage

```bash
# Initialize configuration
runpod-launcher init

# Check available GPUs and pricing
runpod-launcher availability

# Start a pod with GPU selection (or use config default)
runpod-launcher up

# Check model status
runpod-launcher model-status

# Stop the pod
runpod-launcher down

# Query the running model (e.g., using gemma4)
curl https://<pod-id>-8000.proxy.runpod.net/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemma4:latest",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Configuration

Create or edit `~/.config/runpod-launcher/config.toml`:

```toml
# RunPod API key (required)
runpod_api_key = "your-api-key-here"

# GPU selection
gpu_type_id = "NVIDIA A100-SXM4-80GB"    # 80GB VRAM for large models
cuda_version = ""                         # Leave empty for any CUDA version
region = ""                               # Leave empty for any region

# Container setup
image_name = "ollama/ollama:latest"      # Latest Ollama version
container_disk_gb = 150                  # Storage for model weights
volume_mount_path = "/workspace"

# Model configuration
model_name = "qwen3.6:27b"               # Qwen 3.6 27B model
pod_name = "llm-launcher"
ollama_context_len = 131072              # 128K token context window
max_model_len = 0                        # Let Ollama auto-detect

# OpenCode integration (optional)
opencode_config_path = "~/.config/opencode/opencode.jsonc"
env_file_path = "~/.env"
```

### Configuration Options

| Option | Description | Example |
|--------|-------------|---------|
| `runpod_api_key` | Your RunPod API key | `abc123def456` |
| `gpu_type_id` | GPU type to request | `NVIDIA A100-SXM4-80GB` |
| `cuda_version` | Minimum CUDA version (empty = any) | `13.0` |
| `region` | Preferred region (empty = any) | `EUR-NO-1` |
| `image_name` | Docker image to use | `ollama/ollama:latest` |
| `model_name` | Ollama model to run | `qwen3.6:27b`, `mistral`, `gemma4` |
| `container_disk_gb` | Container disk space (GB) | `150` |
| `ollama_context_len` | Context window in tokens | `131072` (128K) |
| `opencode_config_path` | OpenCode config file path | `~/.config/opencode/opencode.jsonc` |
| `env_file_path` | Environment file for credentials | `~/.env` |

## Advanced Features

### Interactive Model Selection TUI

When you run `runpod-launcher up`, you'll be presented with an interactive model selector showing all available models with their VRAM requirements and context window sizes:

```
┌─ Select Model
│  ↑/↓ or k/j: navigate | Enter: select | /: filter | q: quit
│
│  → codellama:7b         15GB VRAM   16K context
│    codellama:13b        20GB VRAM   16K context
│    codellama:34b        50GB VRAM   16K context
│    gemma4:12b           18GB VRAM  128K context
│    gemma4:26b           40GB VRAM  256K context
│    gemma4:31b           50GB VRAM  256K context
│    llama3.1:8b          15GB VRAM  128K context
│    mistral:7b           15GB VRAM   32K context
│    mistral-small:12b    18GB VRAM   32K context
│    mistral-small:22b    30GB VRAM   32K context
│    qwen3.6:7b           15GB VRAM  256K context
│    qwen3.6:27b          50GB VRAM  256K context
│    qwen3.6:32b          60GB VRAM  256K context
```

**Navigation:**
- `↑` / `↓` or `j` / `k` — Navigate through models
- `/` — Enter filter mode to search by model name
- `Enter` — Select highlighted model
- `q` or `ctrl+c` — Cancel

### Model Selection & GPU Filtering

RunPod Launcher provides intelligent GPU recommendations based on your selected model:

```bash
# Interactive model selection during `up`
$ runpod-launcher up
# TUI displays all models with specs

# Or specify a model in config.toml:
model_name = "qwen3.6:27b"
```

**Model Specs Resolution Chain:**

When you specify a model, specs are resolved in this order:
1. **Config overrides** — Custom specs in `[model_specs_override]`
2. **Hardcoded defaults** — Latest Ollama models (≤40B parameters only)
3. **models.dev API** — 100K+ models with real metadata (cached 5 minutes)

This means you can use any model from [models.dev](https://models.dev) catalog!

**Latest Generation Builtin Models (verified from Ollama Library):**

| Model | Size | VRAM | Context | Use Case |
|-------|------|------|---------|----------|
| `gemma4:12b` | 12B | 18GB | 128K | Fast inference, moderate accuracy |
| `gemma4:26b` | 26B | 40GB | 256K | Balanced performance & accuracy |
| `gemma4:31b` | 31B | 50GB | 256K | High accuracy for A100 |
| `llama3.1:8b` | 8B | 15GB | 128K | Lightweight, versatile |
| `qwen3.6:7b` | 7B | 15GB | 256K | Fast with long context |
| `qwen3.6:27b` | 27B | 50GB | 256K | Premium quality (Recommended) |
| `qwen3.6:32b` | 32B | 60GB | 256K | Maximum quality for large GPUs |
| `mistral:7b` | 7B | 15GB | 32K | Fast, instruction-following |
| `mistral-small:12b` | 12B | 18GB | 32K | Balanced Mistral variant |
| `mistral-small:22b` | 22B | 30GB | 32K | Larger Mistral option |
| `codellama:7b` | 7B | 15GB | 16K | Code generation |
| `codellama:13b` | 13B | 20GB | 16K | Better code understanding |
| `codellama:34b` | 34B | 50GB | 16K | Advanced code tasks |

**Custom Model Specifications:**

Override specs for any model (including those from models.dev):

```toml
[model_specs_override.meta-llama/llama-3.1-70b]
min_vram_gb = 60
context_window = 32768
description = "Tuned settings for my GPU"
```

### GPU Suitability Scoring

When you run `runpod-launcher availability --model "qwen3.6:27b"`, GPUs are scored:
- **Green** — Sufficient VRAM + 10GB headroom (ideal)
- **Yellow** — Meets minimum VRAM but tight on memory (risky)
- **Red** — Insufficient VRAM (won't work)

### GraphQL API Integration

RunPod Launcher now uses RunPod's GraphQL API directly for:
- **Real-time GPU availability** — Eliminates stale inventory data
- **Accurate pricing** — Direct from RunPod, updated frequently
- **Pod creation** — Faster, more reliable than CLI wrapper

This provides better reliability and eliminates the runpodctl dependency for core operations.

## CLI Commands

### `runpod-launcher init`

Initialize configuration with a template.

```bash
runpod-launcher init
```

### `runpod-launcher up`

Create and start a new pod, pull the model, and wait for it to be ready. Optionally select a GPU interactively.

```bash
# Basic usage (uses GPU from config)
runpod-launcher up

# Interactive GPU selection with beautiful TUI
# Follow the prompts to browse and select a GPU
runpod-launcher up

# Override region
runpod-launcher up --region "US-EAST"

# Output as JSON
runpod-launcher up --json
```

#### Interactive GPU Selection Example

When you run `runpod-launcher up`, if your configured GPU is unavailable or you use `--select-gpu`, you'll get an interactive terminal UI with your deployment filters displayed:

**Without region/CUDA constraints:**
```
Fetching available GPUs...

┌─ Select GPU (Secure Cloud, Region=(any), CUDA=(any))
│  ↑/↓ or k/j: navigate | Enter: select | /: filter | q: quit
│
│  ▶ NVIDIA RTX 6000 Ada              48GB  High (12)       $0.4400/hr
│    NVIDIA A100-40GB-PCIE            40GB  Limited (5)     $0.6200/hr
│    NVIDIA RTX 5880 Ada              48GB  High (8)        $0.4800/hr
│    NVIDIA L40S                      48GB  Limited (3)     $0.7200/hr
└─  H100-SXM-80GB                    80GB  High (2)        $1.3900/hr
```

**With region and CUDA version constraints:**
```
Fetching available GPUs...

┌─ Select GPU (Secure Cloud, Region=US-WEST, CUDA=12.1)
│  ↑/↓ or k/j: navigate | Enter: select | /: filter | q: quit
│
│  ▶ NVIDIA RTX 6000 Ada              48GB  High (12)       $0.4400/hr
│    NVIDIA A100-40GB-PCIE            40GB  Limited (5)     $0.6200/hr
└─  NVIDIA L40S                      48GB  Limited (3)     $0.7200/hr
```

**Navigation:**
- `↑` / `↓` or `j` / `k` — Move selection up/down
- `/` — Enter filter mode to search by GPU name or ID
- `Enter` — Select highlighted GPU and deploy
- `ctrl+c` or `q` — Cancel selection

**Example filtering (press `/` then type):**
```
┌─ Select GPU (Secure Cloud, Region=(any), CUDA=(any))
│  ↑/↓ or k/j: navigate | Enter: select | /: filter | q: quit
│  Search: a100
│
│  ▶ NVIDIA A100-40GB-PCIE            40GB  Limited (5)     $0.6200/hr
│  NVIDIA A100-80GB                   80GB  High (2)        $1.2400/hr
└
```

After selection, the pod will be created with your chosen GPU and you'll see:
```
Creating pod...
........
Pod is ready: pod-abc123def456
URL: https://pod-abc123def456-8000.proxy.runpod.net
API Key: your-generated-api-key
```

### `runpod-launcher availability`

List deployable GPU types with real-time pricing and optional model suitability filtering.

```bash
# Show available GPUs (secure cloud only, sorted by price)
runpod-launcher availability

# Filter and highlight GPUs suitable for a specific model
runpod-launcher availability --model "qwen3.6:27b"

# Combine with other options
runpod-launcher availability --model "mistral:latest"

# Output as JSON
runpod-launcher availability --json
```

**GPU Suitability Column (when using --model):**
- Shows which GPUs meet the model's minimum VRAM requirement
- Green ✓ = Suitable (meets requirement + 10GB headroom)
- Yellow ⚠ = Marginal (meets requirement but tight)
- Red ✗ = Not suitable (insufficient VRAM)

**Applied Filters (from config, unless overridden by flags):**
- **Cloud:** Always Secure Cloud (matches `up` behavior)
- **Region:** From config or `--region` flag (empty = any region)
- **CUDA Version:** From config or `--cuda-version` flag (empty = any CUDA version)

Output includes:
- GPU name and specifications
- Hourly pricing ($/hr)
- Current availability (High, Limited, Unavailable)
- Memory details

**All GPUs listed here are immediately deployable with your constraints.** Just run `runpod-launcher up` to deploy one, or use `up --select-gpu` to interactively choose a different GPU.

#### Typical Workflow

```bash
# 1. Check what GPUs are available with your constraints
$ runpod-launcher availability
Filters: Cloud=Secure, Region=(any), CudaVersion=(any)

Available GPUs (Secure Cloud, Region=(any), CUDA=(any)):

GPU TYPE ID         NAME                      MEMORY  AVAILABILITY    SECURE PRICE
NVIDIA_RTX_6000_ADA NVIDIA RTX 6000 Ada       48GB    High (12)       $0.4400/hr
NVIDIA_A100_40GB    NVIDIA A100-40GB-PCIE     40GB    Limited (5)     $0.6200/hr
NVIDIA_L40S         NVIDIA L40S               48GB    Limited (3)     $0.7200/hr

# 2. Deploy with interactive GPU selection
$ runpod-launcher up --select-gpu
# → TUI appears, you select "NVIDIA RTX 6000 Ada"
# → Pod launches and becomes ready

# 3. Check pod status anytime
$ runpod-launcher status
pod-abc123: RUNNING

# 4. Verify model is ready
$ runpod-launcher model-status
Model gemma4:latest is loaded and ready

# 5. Query your model
$ curl https://pod-abc123-8000.proxy.runpod.net/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "qwen3.6:27b", "messages": [{"role": "user", "content": "Hello!"}]}'

# 6. Tear down when done
$ runpod-launcher down
Pod terminated: pod-abc123
```

### `runpod-launcher down`

Terminate the running pod.

```bash
runpod-launcher down
```

### `runpod-launcher status`

Check the current pod status.

```bash
runpod-launcher status
runpod-launcher status --json
```

### `runpod-launcher model-status`

Check if the model is loaded and ready.

```bash
# Use saved API key from config
runpod-launcher model-status

# Specify API key explicitly
runpod-launcher model-status --api-key "your-key"

# Check specific model
runpod-launcher model-status gemma4:latest
```

## API Usage

Once your pod is running, use the OpenAI-compatible API:

### List Available Models

```bash
curl https://<pod-id>-8000.proxy.runpod.net/v1/models
```

### Chat Completion

```bash
curl -X POST https://<pod-id>-8000.proxy.runpod.net/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemma4:latest",
    "messages": [{"role": "user", "content": "What is 2+2?"}],
    "temperature": 0.7
  }'
```


## Project Structure

```
runpod-launcher/
├── cmd/runpod-launcher/           # CLI application
│   ├── main.go, init.go, up.go, down.go, status.go, model_status.go
├── internal/
│   ├── config/                    # Configuration management
│   ├── pod/                       # RunPod API client
│   ├── startup/                   # Container startup scripts
│   ├── opencode/                  # OpenCode integration
│   └── util/                      # Helper functions
└── README.md
```

## Key Features

✅ **One-Command Deployment** — Single command to spin up fully functional LLM server

✅ **Interactive GPU Selection** — Beautiful TUI with real-time availability, filtering, and price sorting

✅ **GPU Availability Validation** — Pre-flight checks ensure selected GPU is actually available before deployment

✅ **GPU Inventory Checking** — Query RunPod's current GPU inventory with real-time availability status

✅ **Auto-Generated API Keys** — Secure, random keys created automatically

✅ **Ollama Integration** — Support for 100+ models (Qwen, Gemma, Mistral, Llama, etc.)

✅ **OpenAI-Compatible API** — Drop-in replacement for OpenAI client

✅ **Flexible Configuration** — TOML-based config with CLI flag overrides

✅ **OpenCode Integration** — Auto-update OpenCode config

✅ **Status Monitoring** — Check pod and model status from CLI

## Supported Models

RunPod Launcher includes 13 latest-generation models (≤40B parameters) from the [Ollama Library](https://ollama.com/library):

**Recommended Models:**
- `qwen3.6:27b` — Premium quality with 256K context (default recommendation)
- `gemma4:26b` — Google's latest with 256K context, balanced for A100
- `llama3.1:8b` — Meta's lightweight option with 128K context

**All Built-in Models:**
- **Gemma 4**: `gemma4:12b` (128K context), `gemma4:26b` (256K), `gemma4:31b` (256K)
- **Qwen 3.6**: `qwen3.6:7b` (256K context), `qwen3.6:27b` (256K), `qwen3.6:32b` (256K)
- **Llama 3.1**: `llama3.1:8b` (128K context)
- **Mistral**: `mistral:7b` (32K context), `mistral-small:12b` (32K), `mistral-small:22b` (32K)
- **CodeLlama**: `codellama:7b` (16K context), `codellama:13b` (16K), `codellama:34b` (16K)

**Models from models.dev:**
You can also use any model ID from [models.dev](https://models.dev) — specs will be auto-fetched:
- `meta-llama/llama-3.1-70b` — Auto-fetched from models.dev
- `mistral-ai/Mistral-7B-Instruct-v0.3` — Auto-fetched from models.dev
- `allenai/OLMo-7B-hf` — Auto-fetched from models.dev

**Memory Guidance:**

| GPU | Recommended Model | Notes |
|-----|-------------------|-------|
| **H100 80GB+** | `qwen3.6:32b` | Maximum quality inference |
| **A100 80GB** | `qwen3.6:27b` (50GB weights) | 256K context, perfectly fitted |
| **RTX 6000 Ada 48GB** | `gemma4:26b` or `qwen3.6:27b` | With careful context window tuning |
| **L40S 48GB** | `gemma4:26b` | Good balance of quality and VRAM |
| **RTX 4090 24GB** | `mistral:7b`, `codellama:7b` | Fast inference for coding/chat |
| **Smaller GPUs <20GB** | `gemma4:12b`, `llama3.1:8b` | Lightweight options

## Testing

```bash
go test ./...
go vet ./...
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Built With

- [Cobra](https://cobra.dev/) — CLI framework
- [Ollama](https://ollama.ai/) — Model serving
- [RunPod](https://www.runpod.io/) — GPU infrastructure

## Contact

**Roman Volkov** — GitHub: [@romanvolkov](https://github.com/romanvolkov)

Questions? Open an [issue](https://github.com/romanvolkov/runpod-launcher/issues)!

