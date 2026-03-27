package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const (
	appName    = "peppol-cli"
	configFile = "config.yaml"
)

// Workspace represents a named API workspace.
type Workspace struct {
	Name string `yaml:"name"`
}

// Config holds the CLI configuration.
type Config struct {
	ActiveWorkspace string               `yaml:"active_workspace"`
	Workspaces      map[string]Workspace `yaml:"workspaces,omitempty"`
}

// ConfigDir returns the configuration directory path.
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config/peppol-cli.
func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".config", appName), nil
}

// Load reads the config file. Returns a default config if the file doesn't exist.
func Load() (*Config, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				Workspaces: make(map[string]Workspace),
			}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Workspaces == nil {
		cfg.Workspaces = make(map[string]Workspace)
	}
	return &cfg, nil
}

// Save writes the config file, creating the directory if needed.
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, configFile)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// LoadFrom reads config from a specific directory (for testing).
func LoadFrom(dir string) (*Config, error) {
	path := filepath.Join(dir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Workspaces: make(map[string]Workspace)}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Workspaces == nil {
		cfg.Workspaces = make(map[string]Workspace)
	}
	return &cfg, nil
}

// SaveTo writes config to a specific directory (for testing).
func SaveTo(dir string, cfg *Config) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, configFile), data, 0600)
}

// AddWorkspace adds a named workspace. Returns an error if it already exists.
func (c *Config) AddWorkspace(name string, ws Workspace) error {
	if _, exists := c.Workspaces[name]; exists {
		return fmt.Errorf("workspace %q already exists", name)
	}
	c.Workspaces[name] = ws
	if c.ActiveWorkspace == "" {
		c.ActiveWorkspace = name
	}
	return nil
}

// RemoveWorkspace removes a named workspace. Prevents removing the active
// workspace unless it is the last one remaining.
func (c *Config) RemoveWorkspace(name string) error {
	if _, exists := c.Workspaces[name]; !exists {
		return fmt.Errorf("workspace %q not found", name)
	}
	if c.ActiveWorkspace == name && len(c.Workspaces) > 1 {
		return fmt.Errorf("cannot remove active workspace %q — switch to another workspace first", name)
	}
	delete(c.Workspaces, name)
	if c.ActiveWorkspace == name {
		c.ActiveWorkspace = ""
	}
	return nil
}

// SetActiveWorkspace switches the active workspace.
func (c *Config) SetActiveWorkspace(name string) error {
	if _, exists := c.Workspaces[name]; !exists {
		return fmt.Errorf("workspace %q not found", name)
	}
	c.ActiveWorkspace = name
	return nil
}

// WorkspaceNames returns a sorted list of workspace names.
func (c *Config) WorkspaceNames() []string {
	names := make([]string, 0, len(c.Workspaces))
	for name := range c.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
