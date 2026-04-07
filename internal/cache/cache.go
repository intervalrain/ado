package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const cacheFile = ".ado_cache.json"

type Cache struct {
	Tags []string `json:"tags"`
	path string
}

func Load() *Cache {
	c := &Cache{path: cachePath()}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, c)
	return c
}

func (c *Cache) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

// AddTags merges new tags into the cached list (deduplicates).
func (c *Cache) AddTags(tags []string) {
	existing := make(map[string]bool, len(c.Tags))
	for _, t := range c.Tags {
		existing[t] = true
	}
	for _, t := range tags {
		if t != "" && !existing[t] {
			c.Tags = append(c.Tags, t)
			existing[t] = true
		}
	}
}

func cachePath() string {
	exe, err := os.Executable()
	if err != nil {
		return cacheFile
	}
	return filepath.Join(filepath.Dir(exe), cacheFile)
}
