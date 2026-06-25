package engine

import (
	"errors"
	"sort"
	"sync"
)

// ErrNotSupported is returned by adapter methods the adapter does
// not implement. Callers use errors.Is to detect this specifically
// (vs. engine-side failures).
var ErrNotSupported = errors.New("engine: operation not supported by this adapter")

// ErrUnknownKind is returned by Registry.Get when the requested
// adapter kind isn't registered.
var ErrUnknownKind = errors.New("engine: unknown adapter kind")

// Registry is a concurrent-safe lookup from adapter kind ("camunda7")
// to its Adapter implementation. In v0.1 we ship a single instance
// populated by main during startup; future versions can swap adapters
// at runtime for tenant-specific overrides.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

// Register adds an adapter to the registry. Panics on duplicate
// kind — wiring bugs should fail at boot, not at request time.
func (r *Registry) Register(a Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[a.Kind()]; exists {
		panic("engine: duplicate adapter registration for kind " + a.Kind())
	}
	r.adapters[a.Kind()] = a
}

// Get returns the adapter for a kind, or ErrUnknownKind.
func (r *Registry) Get(kind string) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[kind]
	if !ok {
		return nil, ErrUnknownKind
	}
	return a, nil
}

// Listing is what the UI's target dropdown reads from GET /engines/adapters.
type Listing struct {
	Kind         string       `json:"kind"`
	Name         string       `json:"name"`
	Capabilities Capabilities `json:"capabilities"`
}

// List returns every registered adapter in a deterministic order
// (sorted by kind), safe for serialization.
func (r *Registry) List() []Listing {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Listing, 0, len(r.adapters))
	for _, a := range r.adapters {
		out = append(out, Listing{
			Kind:         a.Kind(),
			Name:         a.Name(),
			Capabilities: a.Capabilities(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}
