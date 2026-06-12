// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ErrNotFound is returned when a requested evaluator is not in the registry.
var ErrNotFound = errors.New("evaluator not found")

// Registry maps evaluator IDs to Evaluator implementations.
// Thread-safe for concurrent access.
type Registry struct {
	mu         sync.RWMutex
	evaluators map[string]Evaluator
}

// NewRegistry creates an empty evaluator registry.
func NewRegistry() *Registry {
	return &Registry{
		evaluators: make(map[string]Evaluator),
	}
}

// Register adds an evaluator to the registry.
// If an evaluator with the same ID already exists, it is replaced.
func (r *Registry) Register(e Evaluator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evaluators[e.ID()] = e
}

// Get retrieves an evaluator by ID.
// Returns an error if the evaluator is not registered.
func (r *Registry) Get(id string) (Evaluator, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.evaluators[id]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	return e, nil
}

// IDs returns a sorted list of all registered evaluator IDs.
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.evaluators))
	for id := range r.evaluators {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
