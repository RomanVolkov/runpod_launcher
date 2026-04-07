// Package config provides configuration loading for runpod-launcher.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

//go:embed config.template.toml
var TemplateContent string

// Config holds all runpod-launcher settings loaded from a TOML file.
type Config struct {
	RunpodAPIKey string `toml:"runpod_api_key"`

	GPUTypeID        string `toml:"gpu_type_id"`
	CudaVersion      string `toml:"cuda_version"`
	Region           string `toml:"region"`
	ImageName        string `toml:"image_name"`
	ContainerDiskGB  int    `toml:"container_disk_gb"`
	VolumeMountPath  string `toml:"volume_mount_path"`
	ModelName        string `toml:"model_name"`
	PodName          string `toml:"pod_name"`
	MaxModelLen      int    `toml:"max_model_len"`
	ToolCallParser   string `toml:"tool_call_parser"`
	LastLLMAPIKey    string `toml:"last_llm_api_key"`

	OpenCodeConfigPath string            `toml:"opencode_config_path"`
	EnvVars            map[string]string `toml:"env_vars"`

	// configPath is stored but not serialized - used by SaveAPIKey
	configPath string
}

// DefaultPath returns the default config file path:
// ~/.config/runpod-launcher/config.toml
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "runpod-launcher", "config.toml")
}

// Load reads and validates the TOML config at path.
// If path is empty, DefaultPath() is used.
// It warns to stderr if the file permissions are not 0600 (skipped on Windows).
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("cannot stat config file %s: %w", path, err)
	}

	// Warn about insecure permissions on non-Windows systems.
	// Check whether group or other bits are set (mask 0177); modes like 0400 are fine.
	if runtime.GOOS != "windows" {
		mode := info.Mode().Perm()
		if mode&0177 != 0 {
			fmt.Fprintf(os.Stderr, "warning: config file %s has permissions %04o; recommend chmod 600\n", path, mode)
		}
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	cfg.configPath = path
	return &cfg, nil
}

// validate checks that all required fields are present.
func validate(cfg *Config) error {
	if cfg.RunpodAPIKey == "" {
		return fmt.Errorf("config: required field %q is missing or empty", "runpod_api_key")
	}
	if cfg.GPUTypeID == "" {
		return fmt.Errorf("config: required field %q is missing or empty", "gpu_type_id")
	}
	if cfg.ModelName == "" {
		return fmt.Errorf("config: required field %q is missing or empty", "model_name")
	}
	return nil
}

// SaveAPIKey updates the config file with the new API key for the current pod.
func (c *Config) SaveAPIKey(apiKey string) error {
	if c.configPath == "" {
		return fmt.Errorf("config path not set; cannot save API key")
	}

	c.LastLLMAPIKey = apiKey

	// Marshal the updated config back to TOML
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write back to file
	if err := os.WriteFile(c.configPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}
