package ingress

import (
	"context"
	"testing"
)

func TestNoopAdapterAllocPort(t *testing.T) {
	n := NewNoopAdapter()
	port, err := n.AllocPort(context.Background(), 8080)
	if err != nil {
		t.Fatalf("AllocPort returned unexpected error: %v", err)
	}
	if port != 0 {
		t.Fatalf("AllocPort returned port %d, expected 0", port)
	}
}

func TestNoopAdapterFreePort(t *testing.T) {
	n := NewNoopAdapter()
	err := n.FreePort(8080)
	if err != nil {
		t.Fatalf("FreePort returned unexpected error: %v", err)
	}
}

// Compile-time check that NoopAdapter still implements IngressAdapter.
var _ IngressAdapter = (*NoopAdapter)(nil)
