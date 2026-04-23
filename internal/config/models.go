package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ModelProfile is a saved LLM configuration that users can switch between.
// Stored as ~/.ado/models/<name>.yaml, mirroring protopub's nats context layout.
type ModelProfile struct {
	Name        string `yaml:"-"`
	Description string `yaml:"description,omitempty"`
	Provider    string `yaml:"provider"`
	Model       string `yaml:"model"`
	APIKey      string `yaml:"api_key,omitempty"`
	APIKeyEnv   string `yaml:"api_key_env,omitempty"`
	BaseURL     string `yaml:"base_url,omitempty"`
	MaxTokens   int    `yaml:"max_tokens,omitempty"`
}

// ResolvedAPIKey returns the key directly or via APIKeyEnv.
func (p *ModelProfile) ResolvedAPIKey() string {
	if p.APIKey != "" {
		return p.APIKey
	}
	if p.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(p.APIKeyEnv)
}

// ModelsDir returns ~/.ado/models.
func ModelsDir() string {
	return expandHome("~/.ado/models")
}

// CurrentModelPath returns the pointer file tracking the active profile.
func CurrentModelPath() string {
	return expandHome("~/.ado/models/current.txt")
}

func modelProfilePath(name string) string {
	return filepath.Join(ModelsDir(), name+".yaml")
}

// LoadModelProfile reads a named profile.
func LoadModelProfile(name string) (*ModelProfile, error) {
	data, err := os.ReadFile(modelProfilePath(name))
	if err != nil {
		return nil, err
	}
	var p ModelProfile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse model profile %q: %w", name, err)
	}
	p.Name = name
	return &p, nil
}

// SaveModelProfile writes a profile to disk.
func SaveModelProfile(p *ModelProfile) error {
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if err := os.MkdirAll(ModelsDir(), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(modelProfilePath(p.Name), data, 0o644)
}

// RemoveModelProfile deletes a profile file.
func RemoveModelProfile(name string) error {
	return os.Remove(modelProfilePath(name))
}

// CurrentModel returns the name of the active profile, or "" if none.
func CurrentModel() string {
	data, err := os.ReadFile(CurrentModelPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetCurrentModel updates the pointer file.
func SetCurrentModel(name string) error {
	if err := os.MkdirAll(ModelsDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(CurrentModelPath(), []byte(name+"\n"), 0o644)
}

// ListModelProfiles returns all profile names, sorted.
func ListModelProfiles() []string {
	matches, _ := filepath.Glob(filepath.Join(ModelsDir(), "*.yaml"))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := strings.TrimSuffix(filepath.Base(m), ".yaml")
		if name == "current" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
