package config_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/romanvolkov/runpod-launcher/internal/config"
)

// writeConfig writes content to a temp file with the given permissions and
// returns the file path plus a cleanup function.
func writeConfig(t *testing.T, content string, perm os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return path
}

const validConfig = `
runpod_api_key    = "rp_test_key"
gpu_type_id       = "AMPERE_16"
image_name        = "vllm/vllm-openai:latest"
model_name        = "mistralai/Mistral-7B-Instruct-v0.2"
container_disk_gb = 50
volume_mount_path = "/workspace"
llm_api_key       = "my-llm-key"
pod_name          = "llm-launcher"

[env_vars]
EXTRA_VAR = "hello"
`

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, validConfig, 0600)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	checks := []struct {
		got  string
		want string
		name string
	}{
		{cfg.RunpodAPIKey, "rp_test_key", "RunpodAPIKey"},
		{cfg.GPUTypeID, "AMPERE_16", "GPUTypeID"},
		{cfg.ImageName, "vllm/vllm-openai:latest", "ImageName"},
		{cfg.ModelName, "mistralai/Mistral-7B-Instruct-v0.2", "ModelName"},
		{cfg.VolumeMountPath, "/workspace", "VolumeMountPath"},
		{cfg.PodName, "llm-launcher", "PodName"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
	if cfg.ContainerDiskGB != 50 {
		t.Errorf("ContainerDiskGB: got %d, want 50", cfg.ContainerDiskGB)
	}
	if cfg.EnvVars["EXTRA_VAR"] != "hello" {
		t.Errorf("EnvVars[EXTRA_VAR]: got %q, want %q", cfg.EnvVars["EXTRA_VAR"], "hello")
	}
	if cfg.OpenCodeConfigPath != "" {
		t.Errorf("OpenCodeConfigPath: got %q, want empty string (not in validConfig)", cfg.OpenCodeConfigPath)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.toml")
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error should contain path %q, got: %v", path, err)
	}
}

func requiredFieldConfig(omit string) string {
	fields := map[string]string{
		"runpod_api_key": `runpod_api_key = "rp_key"`,
		"gpu_type_id":    `gpu_type_id = "AMPERE_16"`,
		"model_name":     `model_name = "some/model"`,
	}
	var lines []string
	for k, v := range fields {
		if k != omit {
			lines = append(lines, v)
		}
	}
	return strings.Join(lines, "\n")
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	required := []string{
		"runpod_api_key",
		"gpu_type_id",
		"model_name",
	}
	for _, field := range required {
		t.Run(fmt.Sprintf("missing_%s", field), func(t *testing.T) {
			content := requiredFieldConfig(field)
			path := writeConfig(t, content, 0600)
			_, err := config.Load(path)
			if err == nil {
				t.Fatalf("expected error for missing %q, got nil", field)
			}
			if !strings.Contains(err.Error(), field) {
				t.Errorf("error should mention field %q, got: %v", field, err)
			}
		})
	}
}

// TestLoad_NoCFFields verifies that config loads successfully with only the three
// required fields — no Cloudflare fields needed.
func TestLoad_NoCFFields(t *testing.T) {
	content := `
runpod_api_key = "rp_key"
gpu_type_id    = "AMPERE_16"
model_name     = "some/model"
`
	path := writeConfig(t, content, 0600)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error loading minimal config, got: %v", err)
	}
	if cfg.RunpodAPIKey != "rp_key" {
		t.Errorf("RunpodAPIKey: got %q, want %q", cfg.RunpodAPIKey, "rp_key")
	}
}

func TestLoad_InsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check not applicable on Windows")
	}

	path := writeConfig(t, validConfig, 0644)

	// Capture stderr output by redirecting os.Stderr temporarily.
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	_, loadErr := config.Load(path)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	if loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}

	stderr := buf.String()
	if !strings.Contains(stderr, "warning") {
		t.Errorf("expected a warning on stderr for 0644 permissions, got: %q", stderr)
	}
}

func TestLoad_SecurePermissionsNoWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check not applicable on Windows")
	}

	path := writeConfig(t, validConfig, 0600)

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	_, loadErr := config.Load(path)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	if loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}

	stderr := buf.String()
	if strings.Contains(stderr, "warning") {
		t.Errorf("expected no warning on stderr for 0600 permissions, got: %q", stderr)
	}
}

// TestLoad_RestrictivePermissionsNoWarning verifies that 0400 (owner read-only) does NOT
// trigger a permission warning — only group/other readable bits should warn.
func TestLoad_RestrictivePermissionsNoWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check not applicable on Windows")
	}

	path := writeConfig(t, validConfig, 0400)

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	_, loadErr := config.Load(path)

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	if loadErr != nil {
		t.Fatalf("unexpected load error: %v", loadErr)
	}

	stderr := buf.String()
	if strings.Contains(stderr, "warning") {
		t.Errorf("expected no warning on stderr for 0400 permissions, got: %q", stderr)
	}
}

func TestDefaultPath(t *testing.T) {
	p := config.DefaultPath()
	if !strings.HasSuffix(p, filepath.Join(".config", "runpod-launcher", "config.toml")) {
		t.Errorf("DefaultPath() = %q, expected suffix .config/runpod-launcher/config.toml", p)
	}
}

func TestLoad_OpenCodeConfigPathField(t *testing.T) {
	content := `
runpod_api_key    = "rp_key"
gpu_type_id       = "AMPERE_16"
model_name        = "some/model"
opencode_config_path = "~/.config/opencode/config.json"
`
	path := writeConfig(t, content, 0600)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error loading config with opencode_config_path, got: %v", err)
	}
	if cfg.OpenCodeConfigPath != "~/.config/opencode/config.json" {
		t.Errorf("OpenCodeConfigPath: got %q, want %q", cfg.OpenCodeConfigPath, "~/.config/opencode/config.json")
	}
}

func TestLoad_OpenCodeConfigPathOptional(t *testing.T) {
	// Verify that empty opencode_config_path is valid (field is optional)
	content := `
runpod_api_key = "rp_key"
gpu_type_id    = "AMPERE_16"
model_name     = "some/model"
`
	path := writeConfig(t, content, 0600)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error loading config without opencode_config_path, got: %v", err)
	}
	if cfg.OpenCodeConfigPath != "" {
		t.Errorf("OpenCodeConfigPath should be empty string when not specified, got %q", cfg.OpenCodeConfigPath)
	}
}

func TestTemplateContent(t *testing.T) {
	if config.TemplateContent == "" {
		t.Error("TemplateContent should not be empty")
	}
	if !strings.Contains(config.TemplateContent, "runpod_api_key") {
		t.Error("TemplateContent should contain 'runpod_api_key'")
	}
}
