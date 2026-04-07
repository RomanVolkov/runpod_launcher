# CLAUDE.md — runpod-launcher

## Package Layout

```
.
├── cmd/runpod-launcher/   # cobra CLI commands: main, up, down, status, init
├── internal/
│   ├── config/            # Config struct, Load(), DefaultPath(), embedded template
│   ├── pod/               # PodClient interface, RunPodClient, WaitForReady
│   ├── startup/           # BuildStartupScript (vLLM launch script builder)
│   └── opencode/          # UpdateConfig (OpenCode config JSON updater)
├── docs/plans/
│   └── completed/         # Completed plan files
├── go.mod
├── go.sum
├── README.md
└── CLAUDE.md
```

## Commands

**Run all tests:**

```bash
go test ./...
```

**Build the binary:**

```bash
go build -o runpod-launcher ./cmd/runpod-launcher/
```

**Vet:**

```bash
go vet ./...
```

**Install locally:**

```bash
go install ./cmd/runpod-launcher/
```

## Key Design Decisions

### PodClient Interface

`internal/pod/pod.go` defines a `PodClient` interface with four methods:
`CreatePod`, `GetPodStatus`, `TerminatePod`, `FindPodByName`. The real implementation
(`RunPodClient`) calls RunPod's GraphQL API. Tests inject a `mockPodClient` struct with
function fields for each method — no external mock libraries needed.

### Factory DI Pattern

`cmd/runpod-launcher/` declares a package-level variable:

```go
var newPodClient func(apiKey string) pod.PodClient = pod.NewRunPodClient
```

Tests override this variable with a mock constructor before calling the command. This keeps
commands testable without hitting real HTTP endpoints.

### go:embed for Config Template

`internal/config/config.go` embeds `config.template.toml` at compile time using
`//go:embed config.template.toml`. The embedded content is exposed as `var TemplateContent string`.
The `init` CLI command writes this string to disk — no file path dependency at runtime,
keeping the binary fully self-contained.

### RunPod GraphQL API

There is no official Go SDK for RunPod. The implementation uses `net/http` with a JSON body:

```
POST https://api.runpod.io/graphql
Content-Type: application/json
Authorization: Bearer <RunpodAPIKey>

{"query": "mutation { podFindAndDeployOnDemand(...) { id } }"}
```

Key operations used:
- `podFindAndDeployOnDemand` — creates a pod on demand
- `pod(input: {podId: ...})` — queries pod status
- `podTerminate` — terminates a pod by ID

The API key is sent as an `Authorization: Bearer <key>` header, not as a query parameter.

### Startup Script Injection

`internal/startup/BuildStartupScript` generates a bash script that is injected as the pod's
startup command. It starts vLLM with the configured model and API key.
Secrets (`LLM_API_KEY`, `MODEL_NAME`) are passed as RunPod pod environment variables — never
hardcoded in the script. RunPod's built-in proxy automatically handles network exposure at
`https://<pod-id>-8000.proxy.runpod.net`.

### Status String Conventions

Status strings used in JSON output follow a two-tier convention:

- **`up` / `down` commands** emit synthetic CLI-friendly labels defined as constants in `internal/pod`:
  - `pod.StatusRunning` = `"running"` — emitted by `up` on success
  - `pod.StatusTerminated` = `"terminated"` — emitted by `down` on success
  These represent the *outcome* of the action, not a live RunPod API value.

- **`status` command** emits the raw RunPod `desiredStatus` value verbatim (e.g. `"RUNNING"`, `"STARTING"`, `"EXITED"`), or `pod.StatusNotFound` = `"not_found"` when no pod is found.
  This reflects the live API state for external consumers/scripts.

This two-tier design is intentional: `up`/`down` confirm action results simply; `status` reports real-time RunPod state. All string literals are package-level constants (`pod.StatusRunning`, `pod.StatusTerminated`, `pod.StatusNotFound`) — never inline magic strings.

### stderr for Progress Output

`WaitForReady` writes dots to `stderr` (not `stdout`) while polling. This keeps `stdout`
clean for `--json` mode, where only the final JSON object should appear on stdout.

### WaitForReady — fast tests via tickInterval

`WaitForReady` accepts a variadic `tickInterval ...time.Duration` parameter. If omitted (or
zero), the default polling interval is 5 seconds. Tests should always pass a short interval
(e.g. `10 * time.Millisecond`) to avoid slow test runs:

```go
pod.WaitForReady(mock, "pod-id", 30*time.Second, &stderr, 10*time.Millisecond)
```

Never call `WaitForReady` from a test without an explicit `tickInterval`.

### OpenCode Auto-Update Pattern

`internal/opencode/UpdateConfig` provides a single, focused function that reads an OpenCode
config JSON file (or creates it if missing), updates the `providers.openai.base_url` and
`providers.openai.api_key` fields, and writes it back atomically via temp file + `os.Rename`.

The `up` command uses a package-level var for injection:

```go
var updateOpenCodeConfig = opencode.UpdateConfig
```

Tests override this function before calling the command, allowing them to verify the correct
parameters are passed without hitting the filesystem. This keeps the config updater testable
and deterministic.
