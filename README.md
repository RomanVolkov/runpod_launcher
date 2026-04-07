# RunPod Launcher

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://golang.org/)
[![Cobra CLI](https://img.shields.io/badge/Cobra-CLI-FF6B6B?logo=github&logoColor=white)](https://cobra.dev/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://www.docker.com/)
[![Ollama](https://img.shields.io/badge/Ollama-000000?logo=ollama&logoColor=white)](https://ollama.ai/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
## Overview

**RunPod Launcher** is a command-line tool that simplifies spinning up GPU pods on RunPod to serve large language models (LLMs) using Ollama. No more manual pod creation, model downloads, or complex configurations—just a single command to deploy a fully functional inference server.

Built in Go with the Cobra CLI framework, this tool automates the entire workflow: pod creation, model loading, API key generation, and OpenCode integration. It provides a simple, reliable way to deploy models like Gemma 4, Qwen, Mistral, and others on RunPod's distributed GPU infrastructure.

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

# Start a pod with defined in config model
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

Create and start a new pod, pull the model, and wait for it to be ready.

```bash
# Basic usage
runpod-launcher up

# Override region
runpod-launcher up --region "US-EAST"

# Output as JSON
runpod-launcher up --json
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

