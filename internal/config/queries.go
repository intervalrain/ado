package config

import "strings"

// NamedQuery is a saved Azure DevOps query with a user-chosen display name.
type NamedQuery struct {
	Name string `yaml:"name"`
	ID   string `yaml:"id"`
}

// QueryList returns the effective, de-duplicated list of named queries.
// The legacy/default query_id is always present: if it isn't already in
// `queries`, it is prepended as "Default". Entries with a blank ID are
// dropped; a blank name falls back to the ID.
func (c *Config) QueryList() []NamedQuery {
	seen := make(map[string]bool)
	var out []NamedQuery
	add := func(q NamedQuery) {
		q.Name = strings.TrimSpace(q.Name)
		q.ID = strings.TrimSpace(q.ID)
		if q.ID == "" || seen[q.ID] {
			return
		}
		if q.Name == "" {
			q.Name = q.ID
		}
		seen[q.ID] = true
		out = append(out, q)
	}

	if c.QueryID != "" && QueryIndex(c.Queries, c.QueryID) < 0 {
		add(NamedQuery{Name: "Default", ID: c.QueryID})
	}
	for _, q := range c.Queries {
		add(q)
	}
	return out
}

// ResolveQuery maps a query name (case-insensitive) or ID to a query ID.
// An empty input resolves to the default query_id; an unknown value is
// assumed to be a raw query ID and returned unchanged.
func (c *Config) ResolveQuery(nameOrID string) string {
	nameOrID = strings.TrimSpace(nameOrID)
	if nameOrID == "" {
		return c.QueryID
	}
	for _, q := range c.QueryList() {
		if strings.EqualFold(q.Name, nameOrID) {
			return q.ID
		}
	}
	return nameOrID
}

// QueryIndex returns the index of the query with the given ID, or -1.
func QueryIndex(qs []NamedQuery, id string) int {
	for i, q := range qs {
		if q.ID == id {
			return i
		}
	}
	return -1
}
