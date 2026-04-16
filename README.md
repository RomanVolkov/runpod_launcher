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
gpu_type_id = "NVIDIA A100-SXM4-80GB"
cuda_version = "13.0"  # Leave empty for any CUDA version
region = ""  # Leave empty for any region

# Container setup
image_name = "ollama/ollama:latest"
container_disk_gb = 50
volume_mount_path = "/workspace"

# Model configuration
model_name = "gemma4"
pod_name = "llm-launcher"

# OpenCode integration (optional)
opencode_config_path = "~/.config/opencode/opencode.jsonc"
```

### Configuration Options

| Option | Description | Example |
|--------|-------------|---------|
| `runpod_api_key` | Your RunPod API key | `abc123def456` |
| `gpu_type_id` | GPU type to request | `NVIDIA A100-SXM4-80GB` |
| `cuda_version` | Minimum CUDA version (empty = any) | `13.0` |
| `region` | Preferred region (empty = any) | `EUR-NO-1` |
| `image_name` | Docker image to use | `ollama/ollama:latest` |
| `model_name` | Ollama model to run | `gemma4`, `mistral`, `qwen` |
| `container_disk_gb` | Container disk space (GB) | `50` |
| `opencode_config_path` | OpenCode config file path | `~/.config/opencode/opencode.jsonc` |

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

List deployable GPU types filtered by your config constraints (region, CUDA version, secure cloud). Shows only GPUs you can actually deploy with `up`.

```bash
# Show available GPUs based on config (secure cloud, your region, your CUDA version)
runpod-launcher availability

# Override region for this query
runpod-launcher availability --region "US-WEST"

# Override CUDA version for this query
runpod-launcher availability --cuda-version "12.1"

# Override both
runpod-launcher availability --region "EU" --cuda-version "13.0"

# Output as JSON
runpod-launcher availability --json
```

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
  -d '{"model": "gemma4:latest", "messages": [{"role": "user", "content": "Hello!"}]}'

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

✅ **GPU Availability Checking** — Query RunPod's current GPU inventory with pricing and specifications

✅ **Auto-Generated API Keys** — Secure, random keys created automatically

✅ **Ollama Integration** — Support for 100+ models (Gemma, Qwen, Mistral, Llama, etc.). Or use a different inference like llama, vLLM, etc.

✅ **OpenAI-Compatible API** — Drop-in replacement for OpenAI client

✅ **Flexible Configuration** — TOML-based config with CLI flag overrides

✅ **OpenCode Integration** — Auto-update OpenCode config

✅ **Status Monitoring** — Check pod and model status from CLI

## Supported Models

E.g., any model from [Ollama Library](https://ollama.com/library):

- **Gemma**: `gemma`, `gemma:4`
- **Mistral**: `mistral`, `mistral-openorca`
- **Qwen**: `qwen`, `qwen3.5-9b`
- **Llama**: `llama2`, `llama2-uncensored`
- **Neural Chat**: `neural-chat`
- **Dolphin**: `dolphin-mixtral`
- And 100+ more...

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

