package store

import (
	"context"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func TestStoreScoping_ListByCluster_ReturnsOnlyMatching(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBodyWithCluster(ctx, "b1", "body-a", orchestrator.StateRunning, `{}`, "docker", "inst-1", "cluster-a")
	if err != nil {
		t.Fatalf("CreateBodyWithCluster b1: %v", err)
	}
	err = s.CreateBodyWithCluster(ctx, "b2", "body-b", orchestrator.StateRunning, `{}`, "docker", "inst-2", "cluster-b")
	if err != nil {
		t.Fatalf("CreateBodyWithCluster b2: %v", err)
	}
	err = s.CreateBodyWithCluster(ctx, "b3", "body-c", orchestrator.StateRunning, `{}`, "docker", "inst-3", "cluster-a")
	if err != nil {
		t.Fatalf("CreateBodyWithCluster b3: %v", err)
	}

	bodies, err := s.ListBodiesByCluster(ctx, "cluster-a")
	if err != nil {
		t.Fatalf("ListBodiesByCluster: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("len(bodies) = %d, want 2", len(bodies))
	}
	for _, b := range bodies {
		if b.ClusterID != "cluster-a" {
			t.Errorf("body %s cluster_id = %q, want cluster-a", b.ID, b.ClusterID)
		}
	}
}

func TestStoreScoping_GetBodyByCluster_WrongCluster(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBodyWithCluster(ctx, "b1", "body-a", orchestrator.StateRunning, `{}`, "docker", "inst-1", "cluster-a")
	if err != nil {
		t.Fatalf("CreateBodyWithCluster: %v", err)
	}

	_, err = s.GetBodyByCluster(ctx, "b1", "cluster-b")
	if err == nil {
		t.Fatal("GetBodyByCluster with wrong cluster should error")
	}
	if err.Error() != "body b1: not found" {
		t.Errorf("error = %q, want %q", err.Error(), "body b1: not found")
	}
}

func TestStoreScoping_ListByCluster_IncludesNullClusterID(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBodyWithCluster(ctx, "b1", "body-a", orchestrator.StateRunning, `{}`, "docker", "inst-1", "cluster-a")
	if err != nil {
		t.Fatalf("CreateBodyWithCluster b1: %v", err)
	}
	err = s.CreateBody(ctx, "b2", "body-null", orchestrator.StateRunning, `{}`, "docker", "inst-2")
	if err != nil {
		t.Fatalf("CreateBody b2: %v", err)
	}

	bodies, err := s.ListBodiesByCluster(ctx, "cluster-a")
	if err != nil {
		t.Fatalf("ListBodiesByCluster: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("len(bodies) = %d, want 2", len(bodies))
	}

	foundA, foundNull := false, false
	for _, b := range bodies {
		if b.ID == "b1" && b.ClusterID == "cluster-a" {
			foundA = true
		}
		if b.ID == "b2" && b.ClusterID == "" {
			foundNull = true
		}
	}
	if !foundA {
		t.Error("expected to find body b1 with cluster-a")
	}
	if !foundNull {
		t.Error("expected to find body b2 with NULL cluster_id")
	}
}

func TestStoreScoping_EmptyClusterID_ReturnsAll(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()

	err := s.CreateBodyWithCluster(ctx, "b1", "body-a", orchestrator.StateRunning, `{}`, "docker", "inst-1", "cluster-a")
	if err != nil {
		t.Fatalf("CreateBodyWithCluster b1: %v", err)
	}
	err = s.CreateBodyWithCluster(ctx, "b2", "body-b", orchestrator.StateRunning, `{}`, "docker", "inst-2", "cluster-b")
	if err != nil {
		t.Fatalf("CreateBodyWithCluster b2: %v", err)
	}

	bodies, err := s.ListBodiesByCluster(ctx, "")
	if err != nil {
		t.Fatalf("ListBodiesByCluster: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("len(bodies) = %d, want 2", len(bodies))
	}
}
