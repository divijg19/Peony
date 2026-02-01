package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultSettleDuration is the default rest duration before a thought becomes eligible.
const DefaultSettleDuration = 18 * time.Hour

// Config holds user-configurable settings for Peony.
type Config struct {
	Editor         string `json:"editor,omitempty"`
	SettleDuration string `json:"settleDuration,omitempty"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		SettleDuration: DefaultSettleDuration.String(),
	}
}

// ConfigPath returns the location of the config file.
func ConfigPath() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "peony", "config.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config path: %w", err)
	}
	return filepath.Join(home, ".config", "peony", "config.json"), nil
}

// Load reads configuration from disk. If the config file does not exist, defaults are returned.
func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Default(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Default(), fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parse config: %w", err)
	}

	cfg = Normalize(cfg)
	return cfg, nil
}

// Save writes configuration to disk.
func Save(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	cfg = Normalize(cfg)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Normalize ensures defaults are set and invalid values are sanitized.
func Normalize(cfg Config) Config {
	cfg.Editor = strings.TrimSpace(cfg.Editor)
	cfg.SettleDuration = strings.TrimSpace(cfg.SettleDuration)
	if cfg.SettleDuration == "" {
		cfg.SettleDuration = DefaultSettleDuration.String()
		return cfg
	}
	if _, err := time.ParseDuration(cfg.SettleDuration); err != nil {
		cfg.SettleDuration = DefaultSettleDuration.String()
	}
	return cfg
}

// SettleDuration returns a parsed duration, falling back to DefaultSettleDuration.
func SettleDuration(cfg Config) time.Duration {
	cfg = Normalize(cfg)
	d, err := time.ParseDuration(cfg.SettleDuration)
	if err != nil {
		return DefaultSettleDuration
	}
	return d
}
