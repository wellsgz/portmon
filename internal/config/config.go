// Package config handles configuration loading from YAML files.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PortConfig holds port configuration with optional description.
type PortConfig struct {
	Port        int    `yaml:"port"`
	Description string `yaml:"description"`
}

// Config holds daemon configuration.
type Config struct {
	Ports         []PortConfig `yaml:"-"` // Handled by custom unmarshaler
	RawPorts      interface{}  `yaml:"ports"`
	DataDir       string       `yaml:"data_dir"`
	RetentionDays int          `yaml:"retention_days"`
	Socket        string       `yaml:"socket"`
	LogLevel      string       `yaml:"log_level"`
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

	// Parse ports - support both old (int list) and new (object list) formats
	cfg.Ports = parsePorts(cfg.RawPorts)

	return &cfg, nil
}

// parsePorts handles both formats:
// ports: [5000, 8080]  OR  ports: [{port: 5000, description: "API"}]
func parsePorts(raw interface{}) []PortConfig {
	if raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []interface{}:
		ports := make([]PortConfig, 0, len(v))
		for _, item := range v {
			switch p := item.(type) {
			case int:
				ports = append(ports, PortConfig{Port: p})
			case float64:
				ports = append(ports, PortConfig{Port: int(p)})
			case map[string]interface{}:
				pc := PortConfig{}
				if port, ok := p["port"]; ok {
					switch pv := port.(type) {
					case int:
						pc.Port = pv
					case float64:
						pc.Port = int(pv)
					}
				}
				if desc, ok := p["description"].(string); ok {
					pc.Description = desc
				}
				if pc.Port > 0 {
					ports = append(ports, pc)
				}
			}
		}
		return ports
	}
	return nil
}

// GetPortNumbers returns just the port numbers for backward compatibility.
func (c *Config) GetPortNumbers() []int {
	ports := make([]int, len(c.Ports))
	for i, p := range c.Ports {
		ports[i] = p.Port
	}
	return ports
}

// Defaults returns a config with default values.
func Defaults() *Config {
	return &Config{
		Ports:         []PortConfig{},
		DataDir:       "/var/lib/portmon",
		RetentionDays: 180,
		Socket:        "/run/portmon/portmon.sock",
		LogLevel:      "info",
	}
}
