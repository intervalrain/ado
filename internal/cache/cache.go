package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const cacheFile = ".ado_cache.json"

type FavRepo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Cache struct {
	Tags     []string  `json:"tags"`
	FavRepos []FavRepo `json:"fav_repos"`
	path     string
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

// AddFavRepo adds a repo to favorites (deduplicates by ID).
func (c *Cache) AddFavRepo(id, name string) {
	for _, r := range c.FavRepos {
		if r.ID == id {
			return
		}
	}
	c.FavRepos = append(c.FavRepos, FavRepo{ID: id, Name: name})
}

// RemoveFavRepo removes a repo from favorites by ID.
func (c *Cache) RemoveFavRepo(id string) {
	for i, r := range c.FavRepos {
		if r.ID == id {
			c.FavRepos = append(c.FavRepos[:i], c.FavRepos[i+1:]...)
			return
		}
	}
}

// IsFavRepo checks if a repo is in favorites.
func (c *Cache) IsFavRepo(id string) bool {
	for _, r := range c.FavRepos {
		if r.ID == id {
			return true
		}
	}
	return false
}

func cachePath() string {
	exe, err := os.Executable()
	if err != nil {
		return cacheFile
	}
	return filepath.Join(filepath.Dir(exe), cacheFile)
}
