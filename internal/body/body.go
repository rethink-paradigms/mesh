// Package body provides body state machine (8 states), lifecycle operations, and migration coordinator.
package body

import (
	"fmt"
	"sync"

	"github.com/rethink-paradigms/mesh/internal/adapter"
)

// Body wraps a store record with an in-memory state machine and per-body mutex.
type Body struct {
	mu         sync.Mutex
	ID         string
	Name       string
	State      adapter.BodyState
	InstanceID adapter.Handle
	Spec       adapter.BodySpec
	Substrate  string
}

// validTransitions defines which state transitions are allowed.
var validTransitions = map[adapter.BodyState][]adapter.BodyState{
	adapter.StateCreated:   {adapter.StateStarting, adapter.StateError},
	adapter.StateStarting:  {adapter.StateRunning, adapter.StateError},
	adapter.StateRunning:   {adapter.StateStopping, adapter.StateMigrating, adapter.StateError},
	adapter.StateStopping:  {adapter.StateStopped, adapter.StateError},
	adapter.StateStopped:   {adapter.StateStarting, adapter.StateDestroyed},
	adapter.StateError:     {adapter.StateStarting, adapter.StateDestroyed},
	adapter.StateMigrating: {adapter.StateRunning, adapter.StateError},
	adapter.StateDestroyed: {},
}

// CanTransition reports whether transitioning to target is valid from current state.
func (b *Body) CanTransition(target adapter.BodyState) bool {
	allowed, ok := validTransitions[b.State]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

// Transition moves the body to the target state if valid.
func (b *Body) Transition(target adapter.BodyState) error {
	if !b.CanTransition(target) {
		return fmt.Errorf("invalid transition: %s → %s", b.State, target)
	}
	b.State = target
	return nil
}
