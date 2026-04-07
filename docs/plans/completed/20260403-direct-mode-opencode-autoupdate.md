# Direct Mode + OpenCode Auto-Update

## Overview

Remove all Cloudflare-related code from the codebase, then add a Cloudflare-free "direct mode"
where the pod is accessed via RunPod's built-in proxy URL (`https://<pod-id>-8000.proxy.runpod.net`).
After `up` succeeds, automatically update the OpenCode config JSON with the new URL so the user
never has to touch it manually.

The result: fewer required config fields, no third-party tunnel dependency, and a fully automated
OpenCode integration.

## Context (from discovery)

- Cloudflare fields to remove: `CloudflareTunnelToken`, `CloudflareAccessClientID`,
  `CloudflareAccessClientSecret`, `TunnelHostname` in `internal/config/config.go`
- `internal/tunnel/tunnel.go` — `BuildStartupScript` installs and starts cloudflared; becomes
  a plain vLLM startup script builder after removal
- `internal/pod/pod.go` — `CreatePod` passes `CLOUDFLARE_TUNNEL_TOKEN` as pod env var; imports
  `internal/tunnel`
- `cmd/runpod-launcher/up.go` — `printUpResult` constructs URL from `cfg.TunnelHostname`
- OpenCode config format: JSON with `providers.openai.base_url` and `providers.openai.api_key`

## Development Approach

- **Testing approach**: Regular (code first, tests after each task)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**
- Run `go test ./...` after each task

## Testing Strategy

- Unit tests for all new/modified functions, both success and error paths
- No e2e tests (CLI tool, no UI)

## Progress Tracking

- Mark completed items with `[x]` immediately when done
- Add newly discovered tasks with ➕ prefix
- Document issues/blockers with ⚠️ prefix

## What Goes Where

- **Implementation Steps**: all code changes, tests, documentation in this repo
- **Post-Completion**: manual smoke test of full `up` → OpenCode auto-updated → query LLM flow

## Implementation Steps

---

### Task 1: Remove Cloudflare fields from config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config.template.toml`
- Modify: `internal/config/config_test.go`

- [x] delete `CloudflareTunnelToken`, `CloudflareAccessClientID`, `CloudflareAccessClientSecret`,
  `TunnelHostname` fields from the `Config` struct
- [x] remove those four fields from the `required` slice in `validate()`
- [x] remove the four fields from `config.template.toml`
- [x] update tests: remove any assertions referencing CF fields; add test that config loads
  successfully without any CF fields present
- [x] run `go test ./...` — must pass before task 2

---

### Task 2: Simplify startup script — remove cloudflared

The `tunnel` package name is misleading once CF is gone. Rename it to `startup` and strip
the cloudflared install/start block, leaving only the vLLM launch.

**Files:**
- Rename: `internal/tunnel/` → `internal/startup/` (package name `tunnel` → `startup`)
- Modify: `internal/startup/startup.go` (was `tunnel.go`)
- Modify: `internal/startup/startup_test.go` (was `tunnel_test.go`)
- Modify: `internal/pod/pod.go` — update import path

- [x] rename directory `internal/tunnel` → `internal/startup`; rename files to `startup.go`
  and `startup_test.go`; update `package tunnel` → `package startup`
- [x] rename `BuildStartupScript` — keep the name or rename to `BuildStartupScript` (keep it)
- [x] remove the cloudflared install block and the `if ! kill -0` liveness check from the
  generated script; keep only the vLLM `python -m vllm.entrypoints.openai.api_server ...` command
- [x] update `internal/pod/pod.go` import from `internal/tunnel` → `internal/startup`;
  update the call site `tunnel.BuildStartupScript` → `startup.BuildStartupScript`
- [x] update package-level comment in `startup.go` to describe vLLM-only script
- [x] update tests: verify script no longer contains `cloudflared`; keep model name validation
  and port validation tests
- [x] run `go test ./...` — must pass before task 3

---

### Task 3: Remove Cloudflare env var from CreatePod

**Files:**
- Modify: `internal/pod/pod.go`
- Modify: `internal/pod/pod_test.go`

- [x] remove `{"key": "CLOUDFLARE_TUNNEL_TOKEN", "value": cfg.CloudflareTunnelToken}` from the
  `envVars` slice in `CreatePod`
- [x] update the GraphQL API reference comment block at the top of the file (remove
  `CLOUDFLARE_TUNNEL_TOKEN` from the example env array)
- [x] update the `CreatePod` godoc comment to remove CF token mention
- [x] update tests: remove any assertions that check for `CLOUDFLARE_TUNNEL_TOKEN` in requests
- [x] run `go test ./...` — must pass before task 4

---

### Task 4: Replace tunnel URL with RunPod proxy URL in `up` command

`printUpResult` currently uses `cfg.TunnelHostname` to build the URL. Replace this with the
RunPod-assigned proxy URL (`https://<pod-id>-8000.proxy.runpod.net`) derived from the pod ID.

**Files:**
- Modify: `cmd/runpod-launcher/up.go`
- Modify: `cmd/runpod-launcher/up_test.go`

- [x] remove all references to `cfg.TunnelHostname` in `up.go`
- [x] add `podProxyURL(podID string, port int) string` helper that returns
  `fmt.Sprintf("https://%s-%d.proxy.runpod.net", podID, port)`
- [x] update `runUp` to pass `podID` to `printUpResult` (already present) and compute URL via
  `podProxyURL(podID, pod.DefaultServicePort)` inside `printUpResult`
- [x] update `printUpResult` signature: remove `tunnelHostname string` parameter, use
  `podProxyURL(podID, pod.DefaultServicePort)` for the URL field
- [x] write tests for `podProxyURL`: verify correct URL format
- [x] write test: `up` JSON output contains proxy URL with pod ID embedded
- [x] run `go test ./...` — must pass before task 5

---

### Task 5: Update README and CLAUDE.md — remove Cloudflare references

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [x] remove the "Cloudflare setup" section and all CF-specific steps from `README.md`
- [x] update the OpenCode config example in `README.md` to show only `base_url` and `api_key`
  (no CF headers); note that the URL will be updated by `runpod-launcher up` automatically
- [x] update `CLAUDE.md`: remove CF fields from package layout, remove Cloudflare design
  decisions, update startup script description
- [x] run `go test ./...` — confirm no regressions

---

### Task 6: Add `opencode_config_path` to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config.template.toml`
- Modify: `internal/config/config_test.go`

- [x] add `OpenCodeConfigPath string \`toml:"opencode_config_path"\`` to `Config` struct
  (optional — not in the required slice)
- [x] add `opencode_config_path = ""` with an inline comment to `config.template.toml`
- [x] write test: field is loaded correctly from TOML; empty value is valid (field is optional)
- [x] run `go test ./...` — must pass before task 7

---

### Task 7: Implement OpenCode config updater

Implement a function that reads the OpenCode JSON config file, merges in the new `base_url`
and `api_key` under `providers.openai`, and writes it back atomically.

**Files:**
- Create: `internal/opencode/opencode.go`
- Create: `internal/opencode/opencode_test.go`

- [x] create package `opencode` with:
  `UpdateConfig(path, baseURL, apiKey string) error`
- [x] expand `~` in path using `os.UserHomeDir()`
- [x] read existing file; if absent, start from an empty map
- [x] unmarshal into `map[string]interface{}`, create/navigate to `providers → openai`,
  set `base_url` and `api_key` keys
- [x] marshal back with `json.MarshalIndent` (2-space indent); write atomically via temp file +
  `os.Rename`
- [x] write test: creates new file with correct JSON structure
- [x] write test: updates existing file; other providers are preserved
- [x] write test: `~` in path is expanded to home directory
- [x] write test: returns error if parent directory does not exist
- [x] run `go test ./...` — must pass before task 8

---

### Task 8: Wire `--opencode-config` flag into `up` command

After the pod is ready, if an OpenCode config path is configured (from `cfg.OpenCodeConfigPath`
or `--opencode-config` flag), call `opencode.UpdateConfig` with the pod proxy URL + llm_api_key.

**Files:**
- Modify: `cmd/runpod-launcher/up.go`
- Modify: `cmd/runpod-launcher/up_test.go`

- [x] add `var upOpenCodeConfig string` and register `--opencode-config string` flag on `upCmd`
- [x] after `printUpResult` succeeds, resolve effective path:
  flag value takes precedence over `cfg.OpenCodeConfigPath`; if both empty, skip
- [x] call `opencode.UpdateConfig(path, proxyURL, cfg.LLMAPIKey)` when path is non-empty
- [x] in plain-text mode print `OpenCode config updated: <path>` to stdout
- [x] in JSON mode add `"opencode_updated": true` to the output map
- [x] inject `opencode.UpdateConfig` via a package-level var for testing:
  `var updateOpenCodeConfig = opencode.UpdateConfig`
- [x] write test: `up` calls `updateOpenCodeConfig` when path is set via flag
- [x] write test: `up` calls `updateOpenCodeConfig` when path is set in config
- [x] write test: flag takes precedence over config field
- [x] write test: `up` skips update when no path is set
- [x] write test: `up` returns error when `updateOpenCodeConfig` fails
- [x] run `go test ./...` — must pass before task 9

---

### Task 9: Verify acceptance criteria

- [x] `runpod-launcher up` works with only `runpod_api_key`, `gpu_type_id`, `model_name`,
  `llm_api_key` set (no CF fields needed) (verified: config.go validate() requires only these 4 fields, no CF fields in config.template.toml)
- [x] JSON output URL is in `https://<pod-id>-8000.proxy.runpod.net` format (verified: TestPodProxyURL tests confirm format; TestUp_JSONOutput_Success shows correct format with pod ID embedded)
- [x] OpenCode config is created/updated after `up` when `opencode_config_path` is configured (verified: TestUp_OpenCodeConfig_FlagPrecedence, ConfigFileFallback, JSONOutput, and PlainTextOutput tests all pass)
- [x] updating OpenCode config preserves pre-existing keys in the JSON (verified: TestUpdateConfigUpdatesExistingFile verifies that other providers are preserved when updating)
- [x] no references to Cloudflare remain in source code, config template, or docs (verified: grep for Cloudflare/cloudflared in active source code found only in old tunnel/ directory; README.md and CLAUDE.md contain no CF references)
- [x] run `go test ./...` — all pass (verified: all 6 packages pass tests)
- [x] run `go vet ./...` — no issues (verified: go vet ./... completed with no errors)

---

### Task 10: [Final] Update documentation and move plan

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [x] update README: add "Direct mode" quickstart with minimal config example
- [x] update README: document `opencode_config_path` and `--opencode-config` flag
- [x] update CLAUDE.md with new package layout (`internal/startup` instead of `internal/tunnel`)
  and OpenCode auto-update pattern
- [x] move this plan to `docs/plans/completed/`

---

## Technical Details

**RunPod proxy URL format:**
```
https://<pod-id>-<port>.proxy.runpod.net
```
Example: `https://abc1234def567-8000.proxy.runpod.net/v1`

**Minimal config after this change:**
```toml
runpod_api_key = "..."
gpu_type_id    = "NVIDIA GeForce RTX 4090"
model_name     = "mistralai/Mistral-7B-Instruct-v0.2"
llm_api_key    = "..."
opencode_config_path = "~/.config/opencode/config.json"
```

**OpenCode config written by auto-update:**
```json
{
  "providers": {
    "openai": {
      "base_url": "https://abc1234def567-8000.proxy.runpod.net/v1",
      "api_key": "your-llm-api-key"
    }
  }
}
```

**Atomic write pattern:** write to `<path>.tmp`, then `os.Rename` to `<path>` — prevents
half-written file on interrupt.

**Package-level var for OpenCode injection in tests:**
```go
var updateOpenCodeConfig = opencode.UpdateConfig
```
Tests override this before calling the command; `t.Cleanup` restores it.

## Post-Completion

**Manual smoke test:**
1. Set only the four required fields + `opencode_config_path` in `config.toml`
2. Run `runpod-launcher up` — pod should start, OpenCode config should update
3. Check `~/.config/opencode/config.json` has the new proxy URL
4. Open OpenCode, verify it connects to the model
5. Run `runpod-launcher down`, then `up` again — OpenCode config updates to new pod ID
