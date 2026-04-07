# RunPod LLM Launcher

## Overview

A Go CLI tool to spin up and tear down a RunPod GPU pod running an LLM (vLLM),
with a stable Cloudflare Tunnel URL so OpenCode's config never needs updating.

**Core workflow:**
1. `runpod-launcher up` — creates a RunPod pod, waits until it's ready, cloudflared tunnel
   connects the pod to a fixed domain (e.g. `llm.yourdomain.com`)
2. OpenCode connects to `https://llm.yourdomain.com` (configured once, never changes)
3. `runpod-launcher down` — terminates the pod and stops billing

**Future:** Alfred workflow that calls `up`/`down` as background scripts.

**Why Go:**
- Compiles to a single binary — no runtime to manage, ideal for Alfred scripts
- Fast startup, easy `go install` distribution
- Standard `net/http` for RunPod's GraphQL API (no Python SDK dependency)

## Context (from discovery)

- Files/components: greenfield project, nothing exists yet
- Related patterns: RunPod GraphQL API (no official Go SDK — use `net/http`), `cobra` CLI, Cloudflare Tunnel
- Dependencies: `github.com/spf13/cobra`, `github.com/BurntSushi/toml`
- Auth: vLLM `--api-key` (layer 1) + Cloudflare Access service token (layer 2)

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Make small, focused changes
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**
- Run tests after each change

## Testing Strategy

- **Unit tests**: RunPod API calls mocked via Go interfaces — no real HTTP in tests
- **Integration tests**: optional, run with `-tags integration`, skipped by default
- Test command: `go test ./...`

## Progress Tracking

- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with ➕ prefix
- Document issues/blockers with ⚠️ prefix

## What Goes Where

- **Implementation Steps** (`[ ]` checkboxes): code changes achievable in this repo
- **Post-Completion**: manual steps requiring external systems (Cloudflare dashboard, RunPod UI)

## Implementation Steps

### Task 1: Project scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/runpod-launcher/main.go`
- Create: `internal/config/config.go`
- Create: `internal/pod/pod.go`
- Create: `internal/tunnel/tunnel.go`
- Create: `.gitignore`
- Create: `README.md` (stub)

- [x] run `go mod init github.com/romanvolkov/runpod-launcher` to create `go.mod`
- [x] add dependencies: `go get github.com/spf13/cobra github.com/BurntSushi/toml`
- [x] create `cmd/runpod-launcher/main.go` with a minimal `cobra` root command and `--help`
- [x] create stub `internal/config/config.go`, `internal/pod/pod.go`,
      `internal/tunnel/tunnel.go` with empty package declarations
- [x] create minimal stub `README.md` (project name + one-line description, marked WIP)
- [x] create `.gitignore` (Go standard: `*.exe`, `*.test`, `*.out`, `/dist/`, `.env`)
- [x] verify `go build ./...` succeeds and `./runpod-launcher --help` works
- [x] write `internal/config/config_test.go` with a smoke test that the package compiles
- [x] run `go test ./...` — must pass before task 2

### Task 2: Config management

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/config/config.template.toml`

- [x] define `Config` struct with TOML tags for all fields:
      - `RunpodAPIKey string` (`runpod_api_key`)
      - `GPUTypeID string` (`gpu_type_id`, e.g. `"AMPERE_16"` — verify from RunPod docs)
      - `ImageName string` (`image_name`, default `"vllm/vllm-openai:latest"`)
      - `ContainerDiskGB int` (`container_disk_gb`, default 50)
      - `VolumeMountPath string` (`volume_mount_path`, default `"/workspace"`)
      - `CloudflareTunnelToken string` (`cloudflare_tunnel_token`)
      - `CloudflareAccessClientID string` (`cloudflare_access_client_id`)
      - `CloudflareAccessClientSecret string` (`cloudflare_access_client_secret`)
      - `LLMAPIKey string` (`llm_api_key`)
      - `ModelName string` (`model_name`, required)
      - `TunnelHostname string` (`tunnel_hostname`, e.g. `"llm.yourdomain.com"`)
      - `PodName string` (`pod_name`, default `"llm-launcher"`)
      - `EnvVars map[string]string` (`env_vars`, optional extra pod env vars)
- [x] implement `Load(path string) (*Config, error)` that reads TOML from the given path
      (default `~/.config/runpod-launcher/config.toml`), returns descriptive error for
      missing file or missing required fields
- [x] implement `DefaultPath() string` returning the default config file path
- [x] in `Load`, check file permissions with `os.Stat` — warn to stderr if mode is not `0600`
      (skip check if `runtime.GOOS == "windows"`)
- [x] create `internal/config/config.template.toml` with all fields and inline comments
- [x] embed the template in `config.go` using `//go:embed config.template.toml` and expose
      it as `var TemplateContent string` — keeps the binary self-contained for `init` command
- [x] write tests in `internal/config/config_test.go`:
      - valid full config parses correctly with all fields populated
      - missing file returns error containing the path
      - empty `runpod_api_key`, `llm_api_key`, `model_name`, CF Access fields return errors
      - insecure permissions (0644) triggers warning written to stderr
- [x] run `go test ./internal/config/...` — must pass before task 3

### Task 3: Startup script builder

**Files:**
- Modify: `internal/tunnel/tunnel.go`
- Create: `internal/tunnel/tunnel_test.go`

*This task comes before pod management so `CreatePod` can call `BuildStartupScript`.*

- [x] implement `BuildStartupScript(modelName string, servicePort int) (string, error)`
      returning a bash script string injected as the pod's startup command;
      secrets come from pod env vars at runtime (never hardcoded):
      - downloads and installs `cloudflared` deb for linux/amd64
      - runs `cloudflared tunnel --no-autoupdate run --token "$CLOUDFLARE_TUNNEL_TOKEN"` in background
      - runs `python -m vllm.entrypoints.openai.api_server` with `--model <modelName>`,
        `--api-key "$LLM_API_KEY"`, `--host 0.0.0.0`, `--port <servicePort>`
- [x] validate `modelName` contains no shell metacharacters before interpolating;
      return an error if invalid
- [x] write tests in `internal/tunnel/tunnel_test.go`:
      - output contains `cloudflared tunnel` invocation
      - output contains `--api-key "$LLM_API_KEY"` (env var reference, not a literal value)
      - output contains `"$CLOUDFLARE_TUNNEL_TOKEN"` (env var reference)
      - `modelName` appears in the vLLM command
      - model name with shell metacharacters (`; rm -rf /`) returns an error
- [x] run `go test ./internal/tunnel/...` — must pass before task 4

### Task 4: RunPod pod management

**Files:**
- Modify: `internal/pod/pod.go`
- Create: `internal/pod/pod_test.go`

- [x] verify RunPod GraphQL API before writing code: inspect the API at
      `https://api.runpod.io/graphql` — find the mutations/queries for pod creation
      (`podFindAndDeployOnDemand` or `podRentInterruptable`), `pod` query, `podTerminate`
      mutation — document the exact GraphQL operation names and required variables in a
      comment block at the top of `pod.go`
- [x] define `PodClient` interface with methods:
      `CreatePod(cfg *config.Config) (string, error)`,
      `GetPodStatus(podID string) (*PodStatus, error)`,
      `TerminatePod(podID string) error`,
      `FindPodByName(name string) (string, error)`
      — enables test mocking without external HTTP
- [x] define `PodStatus` struct with fields: `ID`, `Status`, `DesiredStatus`
- [x] implement `RunPodClient` (satisfies `PodClient`) that calls RunPod's GraphQL API
      over `net/http` using API key from `cfg.RunpodAPIKey` as `Authorization` header;
      `CreatePod` passes `LLM_API_KEY`, `CLOUDFLARE_TUNNEL_TOKEN`, `MODEL_NAME` as pod
      env vars and uses `tunnel.BuildStartupScript(cfg.ModelName, 8000)` as the startup command
- [x] implement `WaitForReady(client PodClient, podID string, timeout time.Duration) error`
      that polls `GetPodStatus` every 5 seconds, prints a dot to stderr each poll for feedback
      (stderr keeps stdout clean for `--json` mode), returns nil when `Status == "RUNNING"`,
      error on timeout
- [x] write tests in `internal/pod/pod_test.go` using a `mockPodClient` struct:
      - `WaitForReady` succeeds when mock returns RUNNING on second call
      - `WaitForReady` returns timeout error when mock never returns RUNNING
      - `FindPodByName` found and not-found cases via mock
- [x] run `go test ./internal/pod/...` — must pass before task 5

### Task 5: CLI — up and down commands

**Files:**
- Modify: `cmd/runpod-launcher/main.go`
- Create: `cmd/runpod-launcher/up.go`
- Create: `cmd/runpod-launcher/down.go`
- Create: `cmd/runpod-launcher/up_test.go`
- Create: `cmd/runpod-launcher/down_test.go`

- [x] implement `up` cobra command:
      - load config (from `--config` flag or default path)
      - call `FindPodByName`; if already running, print existing pod info and exit 0
      - call `CreatePod(cfg)`
      - call `WaitForReady(client, podID, 5*time.Minute)`
      - print success with `https://<cfg.TunnelHostname>`
      - `--json` flag: write `{"status":"running","pod_id":"...","url":"https://..."}` to stdout
- [x] implement `down` cobra command:
      - load config
      - call `FindPodByName`; exit with error message if not found
      - call `TerminatePod(podID)`
      - `--json` flag: write `{"status":"terminated","pod_id":"..."}` to stdout
- [x] add persistent `--config string` flag on root command
- [x] define package-level `var newPodClient func(apiKey string) pod.PodClient` factory
      (default: `pod.NewRunPodClient`); tests override it with a mock constructor
- [x] write tests for `up` and `down` by overriding `newPodClient` with a mock constructor:
      - `up` success: JSON output matches schema, URL comes from config
      - `up` already running: exits 0, outputs existing pod info
      - `down` success: correct JSON
      - `down` pod not found: non-zero exit, clear error to stderr
- [x] run `go test ./cmd/...` — must pass before task 6

### Task 6: CLI — status and init commands

**Files:**
- Modify: `cmd/runpod-launcher/main.go`
- Create: `cmd/runpod-launcher/status.go`
- Create: `cmd/runpod-launcher/init.go`
- Create: `cmd/runpod-launcher/status_test.go`
- Create: `cmd/runpod-launcher/init_test.go`

- [x] implement `status` cobra command:
      - call `FindPodByName`
      - `--json` flag: write `{"status":"running"|"not_found","pod_id":"..."|null}`
- [x] implement `init` cobra command:
      - write `config.TemplateContent` (embedded string) to `~/.config/runpod-launcher/config.toml`
        using `os.WriteFile` — no file path dependency at runtime
      - call `os.Chmod(path, 0600)` immediately after writing
      - print path + next-step instructions
      - return error if file exists and `--force` not set
- [x] write tests:
      - `status` running: correct JSON
      - `status` not found: correct JSON
      - `init` creates file, verifies permissions are `0600`
      - `init` errors without `--force` when file exists
      - `init --force` overwrites
- [x] run `go test ./cmd/...` — must pass before task 7

### Task 7: Verify acceptance criteria

- [x] `go build -o runpod-launcher ./cmd/runpod-launcher/` produces a working binary
- [x] `./runpod-launcher --help` shows all subcommands (up, down, status, init)
- [x] `./runpod-launcher up --json` outputs valid JSON with `url` field (verified via TestUp_JSONOutput_Success)
- [x] `./runpod-launcher down --json` outputs valid JSON (verified via TestDown_JSONOutput_Success)
- [x] `./runpod-launcher init` creates config at correct path with `0600` permissions (verified via TestInit_CreatesFile)
- [x] config errors are clear (missing required field names mentioned explicitly — verified via TestLoad_MissingRequiredFields)
- [x] run full test suite: `go test ./...`
- [x] `go vet ./...` reports no issues

### Task 8: [Final] Documentation and cleanup

**Files:**
- Modify: `README.md`
- Create: `CLAUDE.md`

- [x] write README with: install (`go install`), quickstart (3 steps), all CLI commands,
      Cloudflare Tunnel + Access setup guide, OpenCode config snippet
- [x] add `CLAUDE.md` documenting package layout, test command, RunPod GraphQL API note
- [x] move this plan to `docs/plans/completed/`

## Technical Details

### Package layout

```
.
├── cmd/runpod-launcher/   # cobra commands (main, up, down, status, init)
├── internal/
│   ├── config/            # Config struct + Load() + config.template.toml (go:embed)
│   ├── pod/               # PodClient interface + RunPodClient + WaitForReady
│   └── tunnel/            # BuildStartupScript
├── go.mod
└── go.sum
```

### Startup script injected into RunPod pod

The pod uses `vllm/vllm-openai:latest` (vLLM pre-installed). The startup script adds cloudflared:

```bash
#!/bin/bash
# Install cloudflared
curl -L --output /tmp/cloudflared.deb \
  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
dpkg -i /tmp/cloudflared.deb

# Start tunnel in background (token from pod env var)
cloudflared tunnel --no-autoupdate run --token "$CLOUDFLARE_TUNNEL_TOKEN" &

# Start vLLM with API key auth
python -m vllm.entrypoints.openai.api_server \
  --model "$MODEL_NAME" \
  --host 0.0.0.0 \
  --port 8000 \
  --api-key "$LLM_API_KEY"
```

`CLOUDFLARE_TUNNEL_TOKEN`, `LLM_API_KEY`, and `MODEL_NAME` are RunPod pod env vars sourced
from `config.toml`. Never hardcoded.

### RunPod GraphQL API

No official Go SDK exists. Use `net/http` with JSON body:

```
POST https://api.runpod.io/graphql?api_key=<RunpodAPIKey>
Content-Type: application/json

{"query": "mutation { podFindAndDeployOnDemand(...) { id } }"}
```

Verify exact mutation names and arguments from RunPod's API explorer before implementing.

### Mocking strategy

Tests inject a `PodClient` interface rather than hitting the real API:

```go
type mockPodClient struct {
    createFn     func(*config.Config) (string, error)
    getStatusFn  func(string) (*PodStatus, error)
    terminateFn  func(string) error
    findByNameFn func(string) (string, error)
}
```

No external mock libraries needed — standard Go interfaces suffice.

### Cloudflare Tunnel + Access setup (one-time, manual)

**Tunnel (stable HTTPS transport):**
1. Cloudflare Zero Trust → Tunnels → Create a tunnel
2. Name it (e.g. `runpod-llm`)
3. Add public hostname: `llm.yourdomain.com` → `http://localhost:8000`
4. Copy tunnel token → `~/.config/runpod-launcher/config.toml`

**Cloudflare Access (blocks unauthenticated requests before reaching vLLM):**
5. Zero Trust → Access → Applications → Add application (Self-hosted)
6. Domain: `llm.yourdomain.com`, policy: Service Auth only
7. Access → Service Auth → Create a service token
8. Copy `CF-Access-Client-Id` and `CF-Access-Client-Secret` → `config.toml`
9. Add headers to OpenCode config (see snippet below)

### OpenCode config snippet (set once)

```json
{
  "providers": {
    "openai": {
      "base_url": "https://llm.yourdomain.com/v1",
      "api_key": "<llm_api_key from config.toml>",
      "headers": {
        "CF-Access-Client-Id": "<cloudflare_access_client_id>",
        "CF-Access-Client-Secret": "<cloudflare_access_client_secret>"
      }
    }
  }
}
```

Two auth layers:
- CF Access headers → Cloudflare rejects requests at the edge without them
- `Authorization: Bearer <llm_api_key>` → vLLM rejects requests with wrong/missing key

### Alfred workflow integration

Each Alfred workflow action calls the binary as a background script:
```bash
/usr/local/bin/runpod-launcher up --json
/usr/local/bin/runpod-launcher down --json
/usr/local/bin/runpod-launcher status --json
```

Alfred parses the JSON output to show notifications. Use absolute path since Alfred may not
have the user's `$PATH`.

## Post-Completion

**Cloudflare Tunnel + Access one-time setup:**
- Create a Cloudflare Zero Trust account (free tier)
- Create a named tunnel, add public hostname → `localhost:8000`
- Copy tunnel token into `~/.config/runpod-launcher/config.toml`
- Create CF Access application (Self-hosted) for `llm.yourdomain.com`
- Create a service token, copy client ID + secret into `config.toml`
- Add CF Access headers + API key to OpenCode config

**RunPod setup:**
- Generate a RunPod API key at `runpod.io/console/user/settings`
- Choose GPU type ID from RunPod's available GPU list
- Default image is `vllm/vllm-openai:latest`; override in config if needed

**Alfred workflow:**
- Create Alfred workflow with keyword triggers (`llm up`, `llm down`)
- Each trigger runs `runpod-launcher up --json` / `runpod-launcher down --json` (absolute path)
- Parse JSON output for notification text

**OpenCode:**
- Set `base_url` in OpenCode config to the Cloudflare tunnel hostname (once, never changes)

**Binary distribution:**
- `go install github.com/romanvolkov/runpod-launcher@latest` for local install
- Or `go build -o runpod-launcher ./cmd/runpod-launcher/` and copy to `/usr/local/bin/`

---

## Appendix: What You Need to Set Up (Before First Use)

Everything you need to gather before running `runpod-launcher init` and filling in your config.

### A. RunPod

**What you need:** API key, GPU type ID

1. **Create account**: go to `runpod.io` and sign up
2. **Add payment method**: Settings → Billing → add a credit card (pods are billed per second)
3. **Generate API key**:
   - Settings → API Keys → `+ API Key`
   - Name it (e.g. `runpod-launcher`)
   - Copy the key → this is your `runpod_api_key` in `config.toml`
   - ⚠️ It's only shown once — save it immediately
4. **Choose a GPU type ID**:
   - Go to `runpod.io/console/gpu-cloud` and browse available GPUs
   - The `gpu_type_id` is the machine-readable ID (e.g. `AMPERE_16`, `ADA_LOVELACE_24`)
   - Check the RunPod GraphQL API or console URL for the exact string when implementing Task 4
5. **No project/workspace setup needed** — RunPod is just API + pay-per-use

---

### B. Cloudflare

**What you need:** domain on Cloudflare, tunnel token, CF Access service token (client ID + secret)

#### B1. Domain setup (skip if you already have a domain on Cloudflare)

1. If you don't have a domain: buy one at `cloudflare.com/products/registrar` (cheapest option)
   or use an existing domain from another registrar and point its nameservers to Cloudflare
2. Your domain must be active in a Cloudflare account before you can create tunnels against it

#### B2. Create a Cloudflare Tunnel

3. Go to `one.dash.cloudflare.com` → select your account → **Zero Trust**
   (first time: you'll be asked to pick a team name, e.g. `romanvolkov` — this is free)
4. In Zero Trust: **Networks → Tunnels → Create a tunnel**
5. Choose **Cloudflared** as the connector type
6. Name the tunnel (e.g. `runpod-llm`) → click Save tunnel
7. Skip the connector install step (the pod installs cloudflared itself)
8. **Add a public hostname**:
   - Subdomain: `llm` (or whatever you prefer)
   - Domain: your domain (e.g. `yourdomain.com`) → result: `llm.yourdomain.com`
   - Service: `HTTP` → `localhost:8000`
   - Save hostname
9. Copy the **tunnel token** from the install instructions page
   → this is your `cloudflare_tunnel_token` in `config.toml`
   → it looks like a long JWT string
10. Note the full hostname you chose (e.g. `llm.yourdomain.com`)
    → this is your `tunnel_hostname` in `config.toml`

#### B3. Create a Cloudflare Access application + service token

11. In Zero Trust: **Access → Applications → Add an application**
12. Choose **Self-hosted**
13. Fill in:
    - Application name: `RunPod LLM` (or anything)
    - Subdomain: `llm`, Domain: your domain (must match tunnel hostname exactly)
14. Click Next → **Add a policy**:
    - Policy name: `Service token only`
    - Action: `Service Auth`
    - Include rule: leave empty (service tokens are matched at auth level, not rule level)
    - Save policy
15. Click Next → Save application
16. Now create the service token: **Access → Service Auth → Service Tokens → Create Service Token**
17. Name it (e.g. `opencode-client`) → click **Generate token**
18. Copy both values immediately (shown only once):
    - `CF-Access-Client-Id` → this is your `cloudflare_access_client_id` in `config.toml`
    - `CF-Access-Client-Secret` → this is your `cloudflare_access_client_secret` in `config.toml`
19. Back in your Access Application, go to the policy → add an Include rule:
    - Selector: **Service Token** → choose the token you just created
    - Save — now only that service token can reach `llm.yourdomain.com`

---

### C. Config summary

Once done, your `~/.config/runpod-launcher/config.toml` should look like:

```toml
runpod_api_key              = "rp_xxxx..."            # from RunPod Settings → API Keys
gpu_type_id                 = "AMPERE_16"              # verify from RunPod GPU list
image_name                  = "vllm/vllm-openai:latest"
model_name                  = "mistralai/Mistral-7B-Instruct-v0.2"
container_disk_gb           = 50
volume_mount_path           = "/workspace"

cloudflare_tunnel_token     = "eyJ..."                 # from Cloudflare tunnel setup (step 9)
tunnel_hostname             = "llm.yourdomain.com"     # your chosen subdomain (step 10)
cloudflare_access_client_id     = "abc123.access"      # from CF service token (step 18)
cloudflare_access_client_secret = "xyz789..."          # from CF service token (step 18)

llm_api_key                 = "choose-any-secret"      # pick any strong random string
pod_name                    = "llm-launcher"
```

### D. OpenCode config (set once, never changes)

Add to your OpenCode config file:

```json
{
  "providers": {
    "openai": {
      "base_url": "https://llm.yourdomain.com/v1",
      "api_key": "<same value as llm_api_key above>",
      "headers": {
        "CF-Access-Client-Id": "<cloudflare_access_client_id>",
        "CF-Access-Client-Secret": "<cloudflare_access_client_secret>"
      }
    }
  }
}
```
