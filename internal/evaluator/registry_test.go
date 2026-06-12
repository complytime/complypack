// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"sync"
	"testing"

	"cuelang.org/go/cue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEvaluator is a test implementation of Evaluator.
type mockEvaluator struct {
	id string
}

func (m *mockEvaluator) ID() string                                   { return m.id }
func (m *mockEvaluator) Validate(filename string, src string) []error { return nil }
func (m *mockEvaluator) CheckContract(filename string, src string, schema cue.Value) ([]ContractViolation, error) {
	return nil, nil
}
func (m *mockEvaluator) Test(ctx context.Context, files map[string]string) (*TestResults, error) {
	return &TestResults{}, nil
}
func (m *mockEvaluator) Lint(filename string, src string) ([]LintWarning, error) { return nil, nil }
func (m *mockEvaluator) FileExtension() string                                   { return ".mock" }

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)
	assert.Empty(t, r.IDs())
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	e := &mockEvaluator{id: "test.evaluator"}

	r.Register(e)

	got, err := r.Get("test.evaluator")
	require.NoError(t, err)
	assert.Equal(t, e, got)
}

func TestGetUnknown(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("unknown")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Contains(t, err.Error(), "unknown")
}

func TestRegisterReplace(t *testing.T) {
	r := NewRegistry()
	e1 := &mockEvaluator{id: "test.evaluator"}
	e2 := &mockEvaluator{id: "test.evaluator"}

	r.Register(e1)
	r.Register(e2)

	got, err := r.Get("test.evaluator")
	require.NoError(t, err)
	assert.Equal(t, e2, got, "should return most recently registered evaluator")
}

func TestIDs(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockEvaluator{id: "zebra"})
	r.Register(&mockEvaluator{id: "alpha"})
	r.Register(&mockEvaluator{id: "beta"})

	ids := r.IDs()
	assert.Equal(t, []string{"alpha", "beta", "zebra"}, ids, "IDs should be sorted")
}

func TestConcurrentAccess(t *testing.T) {
	r := NewRegistry()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			e := &mockEvaluator{id: "concurrent.test"}
			r.Register(e)
			_, _ = r.Get("concurrent.test")
			_ = r.IDs()
		}(i)
	}

	wg.Wait()

	_, err := r.Get("concurrent.test")
	assert.NoError(t, err, "concurrent access should not corrupt registry")
}
