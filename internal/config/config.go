// Package config handles configuration loading from YAML files.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds daemon configuration.
type Config struct {
	Ports         []int  `yaml:"ports"`
	DataDir       string `yaml:"data_dir"`
	RetentionDays int    `yaml:"retention_days"`
	Socket        string `yaml:"socket"`
	LogLevel      string `yaml:"log_level"`
}

// DefaultConfigPath is the default location for the config file.
const DefaultConfigPath = "/etc/portmon/portmon.yaml"

// Load reads configuration from a YAML file.
func Load(path string) (*Config, error) {
	// Expand ~ to home directory
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Defaults returns a config with default values.
func Defaults() *Config {
	return &Config{
		Ports:         []int{},
		DataDir:       "",
		RetentionDays: 180,
		Socket:        "",
		LogLevel:      "info",
	}
}
