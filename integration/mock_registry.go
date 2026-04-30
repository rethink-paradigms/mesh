//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

type mockRegistry struct {
	mu        sync.Mutex
	pushed    map[string]string
	pulled    []string
	pushErr   error
	pullErr   error
	verifyErr error
	failCount int
	failAfter int
}

func (m *mockRegistry) Push(_ context.Context, key string, r io.Reader, _ int64, sha256 string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pushErr != nil {
		return m.pushErr
	}
	if m.failAfter > 0 && m.failCount < m.failAfter {
		m.failCount++
		return fmt.Errorf("mock push failure %d", m.failCount)
	}
	data, _ := io.ReadAll(r)
	if m.pushed == nil {
		m.pushed = make(map[string]string)
	}
	m.pushed[key] = string(data)
	_ = sha256
	return nil
}

func (m *mockRegistry) Pull(_ context.Context, key string) (io.ReadCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pullErr != nil {
		return nil, "", m.pullErr
	}
	data, ok := m.pushed[key]
	if !ok {
		return nil, "", fmt.Errorf("key %s not found", key)
	}
	m.pulled = append(m.pulled, key)
	return io.NopCloser(strings.NewReader(data)), "", nil
}

func (m *mockRegistry) Verify(_ context.Context, key, expectedSHA256 string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.verifyErr != nil {
		return m.verifyErr
	}
	if m.pushed == nil {
		return fmt.Errorf("key %s not found", key)
	}
	_, ok := m.pushed[key]
	if !ok {
		return fmt.Errorf("key %s not found", key)
	}
	_ = expectedSHA256
	return nil
}
