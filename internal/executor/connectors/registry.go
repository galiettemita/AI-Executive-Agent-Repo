package connectors

import (
	"sort"
	"sync"
)

type Definition struct {
	ConnectorKey string
	Provider     string
	AuthType     string
	RiskLevel    string
	DataClass    string
}

type Registry struct {
	mu          sync.RWMutex
	definitions map[string]Definition
}

func NewRegistry() *Registry {
	return &Registry{definitions: map[string]Definition{}}
}

func (r *Registry) Upsert(definition Definition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions[definition.ConnectorKey] = definition
}

func (r *Registry) Get(connectorKey string) (Definition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	definition, ok := r.definitions[connectorKey]
	return definition, ok
}

func (r *Registry) Keys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.definitions))
	for connectorKey := range r.definitions {
		out = append(out, connectorKey)
	}
	sort.Strings(out)
	return out
}
