// Package body provides body state machine (8 states), lifecycle operations, and migration coordinator.
package body

import (
	"fmt"
	"sync"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

type AllocatedPort struct {
	Name          string `json:"name"`
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Protocol      string `json:"protocol"`
	AccessURL     string `json:"access_url,omitempty"`
}

type Body struct {
	mu              sync.Mutex
	ID              string
	Name            string
	State           orchestrator.BodyState
	InstanceID      orchestrator.Handle
	Spec            orchestrator.BodySpec
	Substrate       string
	PortAllocations []AllocatedPort
}

var validTransitions = map[orchestrator.BodyState][]orchestrator.BodyState{
	orchestrator.StateCreated:   {orchestrator.StateStarting, orchestrator.StateError},
	orchestrator.StateStarting:  {orchestrator.StateRunning, orchestrator.StateError},
	orchestrator.StateRunning:   {orchestrator.StateStopping, orchestrator.StateMigrating, orchestrator.StateError, orchestrator.StateRunning, orchestrator.StateExited},
	orchestrator.StateStopping:  {orchestrator.StateStopped, orchestrator.StateError},
	orchestrator.StateStopped:   {orchestrator.StateStarting, orchestrator.StateDestroyed},
	orchestrator.StateError:     {orchestrator.StateStarting, orchestrator.StateDestroyed, orchestrator.StateMigrating},
	orchestrator.StateExited:    {orchestrator.StateStarting, orchestrator.StateDestroyed, orchestrator.StateStopped},
	orchestrator.StateMigrating: {orchestrator.StateRunning, orchestrator.StateError},
	orchestrator.StateDestroyed: {},
}

func (b *Body) CanTransition(target orchestrator.BodyState) bool {
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

func (b *Body) Transition(target orchestrator.BodyState) error {
	if !b.CanTransition(target) {
		return fmt.Errorf("invalid transition: %s → %s", b.State, target)
	}
	b.State = target
	return nil
}
