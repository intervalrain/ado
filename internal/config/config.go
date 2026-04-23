package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the ado CLI tool.
// Loaded from ~/.ado/config.yaml with env var overrides.
type Config struct {
	Org      string `yaml:"org"`
	Project  string `yaml:"project"`
	PAT      string `yaml:"pat"`
	QueryID  string `yaml:"query_id"`
	Assignee string `yaml:"assignee"`

	Summary SummarySection `yaml:"summary"`
	LLM     LLMSection     `yaml:"llm"`
}

type SummarySection struct {
	Days     int      `yaml:"days"`
	Repos    []string `yaml:"repos"`
	Template string   `yaml:"template"`
	Author   string   `yaml:"author"`
	Output   string   `yaml:"output"`
}

type LLMSection struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	APIKey    string `yaml:"api_key"`
	APIKeyEnv string `yaml:"api_key_env"`
	BaseURL   string `yaml:"base_url"`
	MaxTokens int    `yaml:"max_tokens"`
}

// ResolvedAPIKey returns the API key, preferring the direct api_key field
// over the environment variable named in api_key_env.
func (l *LLMSection) ResolvedAPIKey() string {
	if l.APIKey != "" {
		return l.APIKey
	}
	if l.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(l.APIKeyEnv)
}

// Validate checks that required fields (Org, PAT) are set.
func (c *Config) Validate() error {
	if c.Org == "" || c.PAT == "" {
		return fmt.Errorf("org and pat are required — set them in %s or via ADO_ORG / ADO_PAT env vars", ConfigPath())
	}
	return nil
}

// ConfigPath returns the default config file path.
func ConfigPath() string {
	return expandHome("~/.ado/config.yaml")
}

// Load reads ~/.ado/config.yaml and applies env var overrides.
func Load() (*Config, error) {
	cfg := &Config{
		Summary: SummarySection{
			Days:     7,
			Template: "~/.ado/template.md",
			Output:   "~/.ado/reports",
		},
		LLM: LLMSection{
			Provider:  "claude",
			Model:     "claude-sonnet-4-20250514",
			APIKeyEnv: "ANTHROPIC_API_KEY",
			MaxTokens: 4096,
		},
	}

	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	// Overlay the active model profile (if any) onto the inline llm: section.
	// The inline section acts as the default; a selected profile overrides it
	// so users can switch providers/models without rewriting config.yaml.
	if name := CurrentModel(); name != "" {
		if p, err := LoadModelProfile(name); err == nil {
			applyProfile(&cfg.LLM, p)
		}
	}

	// Env var overrides (backward compat with .env / shell exports)
	envOverride(&cfg.Org, "ADO_ORG")
	envOverride(&cfg.Project, "ADO_PROJECT")
	envOverride(&cfg.PAT, "ADO_PAT")
	envOverride(&cfg.QueryID, "ADO_QUERY_ID")
	envOverride(&cfg.Assignee, "ADO_ASSIGNEE")

	// Note: org/pat validation moved to individual commands.
	// TUI settings screen needs to load even when unconfigured.

	// Apply defaults for zero values
	if cfg.Summary.Days == 0 {
		cfg.Summary.Days = 7
	}
	if cfg.LLM.Provider == "" {
		cfg.LLM.Provider = "claude"
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = "claude-sonnet-4-20250514"
	}
	if cfg.LLM.APIKeyEnv == "" {
		cfg.LLM.APIKeyEnv = "ANTHROPIC_API_KEY"
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = 4096
	}

	// Expand ~ in paths
	cfg.Summary.Template = expandHome(cfg.Summary.Template)
	cfg.Summary.Output = expandHome(cfg.Summary.Output)
	for i, r := range cfg.Summary.Repos {
		cfg.Summary.Repos[i] = expandHome(r)
	}

	// Default to CWD if no repos configured
	if len(cfg.Summary.Repos) == 0 {
		cwd, _ := os.Getwd()
		cfg.Summary.Repos = []string{cwd}
	}

	return cfg, nil
}

// Save writes the config to ~/.ado/config.yaml.
func Save(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(ConfigPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	return os.WriteFile(ConfigPath(), data, 0644)
}

// applyProfile overlays non-empty fields from a ModelProfile onto an LLMSection.
func applyProfile(l *LLMSection, p *ModelProfile) {
	if p.Provider != "" {
		l.Provider = p.Provider
	}
	if p.Model != "" {
		l.Model = p.Model
	}
	if p.APIKey != "" {
		l.APIKey = p.APIKey
	}
	if p.APIKeyEnv != "" {
		l.APIKeyEnv = p.APIKeyEnv
	}
	if p.BaseURL != "" {
		l.BaseURL = p.BaseURL
	}
	if p.MaxTokens != 0 {
		l.MaxTokens = p.MaxTokens
	}
}

func envOverride(field *string, key string) {
	if v := os.Getenv(key); v != "" {
		*field = v
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
