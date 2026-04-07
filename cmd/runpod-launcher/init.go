package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/romanvolkov/runpod-launcher/internal/config"
)

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file at ~/.config/runpod-launcher/config.toml",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing config file")
}

func runInit(cmd *cobra.Command, args []string) error {
	return runInitWithPath(cmd, config.DefaultPath(), initForce)
}

// runInitWithPath writes the template config to the given path.
// It is separated from runInit so tests can supply an arbitrary path without
// depending on os.UserHomeDir.
func runInitWithPath(cmd *cobra.Command, path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists: %s (use --force to overwrite)", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot stat config path %s: %w", path, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(config.TemplateContent), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Config written to: %s\n\nNext steps:\n  1. Edit the file and fill in your RunPod API key, GPU type, and LLM settings.\n  2. Run `runpod-launcher up` to start your pod.\n", path)
	return nil
}
