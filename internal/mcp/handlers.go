package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rethink-paradigms/mesh/internal/adapter"
)

func (s *Server) registerTools() {
	s.RegisterTool("ping", s.handlePing, ToolDefinition{
		Name:        "ping",
		Description: "Health check. Returns pong.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	})
	s.RegisterTool("list_bodies", s.handleListBodies, ToolDefinition{
		Name:        "list_bodies",
		Description: "List all managed bodies.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	})
	s.RegisterTool("get_body", s.handleGetBody, ToolDefinition{
		Name:        "get_body",
		Description: "Get body details by ID.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
	})
	s.RegisterTool("get_snapshot", s.handleGetSnapshot, ToolDefinition{
		Name:        "get_snapshot",
		Description: "Get snapshot details by ID.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
	})
	s.RegisterTool("create_body", s.handleCreateBody, ToolDefinition{
		Name:        "create_body",
		Description: "Create and start a new body on the substrate.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"image":{"type":"string"},"workdir":{"type":"string"},"env":{"type":"object","additionalProperties":{"type":"string"}},"cmd":{"type":"array","items":{"type":"string"}},"memory_mb":{"type":"integer"},"cpu_shares":{"type":"integer"}},"required":["name","image"]}`),
	})
	s.RegisterTool("delete_body", s.handleDeleteBody, ToolDefinition{
		Name:        "delete_body",
		Description: "Destroy a stopped or errored body.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`),
	})
	s.RegisterTool("migrate_body", s.handleMigrateBody, ToolDefinition{
		Name:        "migrate_body",
		Description: "Migrate a body to a different substrate.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"},"target_substrate":{"type":"string"}},"required":["body_id","target_substrate"]}`),
	})
	s.RegisterTool("execute_command", s.handleExecCommand, ToolDefinition{
		Name:        "execute_command",
		Description: "Execute a command inside a body.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"},"command":{"type":"array","items":{"type":"string"}}},"required":["body_id","command"]}`),
	})
}

func (s *Server) handlePing(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"pong": true}, nil
}

func (s *Server) handleListBodies(ctx context.Context, params json.RawMessage) (interface{}, error) {
	bodies, err := s.store.ListBodies(ctx)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}
	return bodies, nil
}

func (s *Server) handleGetBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: id"}
	}
	body, err := s.store.GetBody(ctx, p.ID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}
	return body, nil
}

func (s *Server) handleGetSnapshot(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: id"}
	}
	snap, err := s.store.GetSnapshot(ctx, p.ID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}
	return snap, nil
}

func (s *Server) handleCreateBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}
	var p struct {
		Name      string            `json:"name"`
		Image     string            `json:"image"`
		Workdir   string            `json:"workdir,omitempty"`
		Env       map[string]string `json:"env,omitempty"`
		Cmd       []string          `json:"cmd,omitempty"`
		MemoryMB  int               `json:"memory_mb,omitempty"`
		CPUShares int               `json:"cpu_shares,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if p.Name == "" || p.Image == "" {
		return nil, &RPCError{Code: -32602, Message: "name and image are required"}
	}

	spec := adapter.BodySpec{
		Image:     p.Image,
		Workdir:   p.Workdir,
		Env:       p.Env,
		Cmd:       p.Cmd,
		MemoryMB:  p.MemoryMB,
		CPUShares: p.CPUShares,
	}

	b, err := s.bodyMgr.Create(ctx, p.Name, spec)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}

	return map[string]interface{}{
		"id":     b.ID,
		"name":   b.Name,
		"state":  string(b.State),
		"handle": string(b.InstanceID),
	}, nil
}

func (s *Server) handleDeleteBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: id"}
	}
	if err := s.bodyMgr.Destroy(ctx, p.ID); err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}
	return map[string]bool{"deleted": true}, nil
}

func (s *Server) handleMigrateBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.migrator == nil {
		return nil, &RPCError{Code: -32603, Message: "migration coordinator not available"}
	}
	var p struct {
		BodyID          string `json:"body_id"`
		TargetSubstrate string `json:"target_substrate"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" || p.TargetSubstrate == "" {
		return nil, &RPCError{Code: -32602, Message: "body_id and target_substrate are required"}
	}
	migrationID, err := s.migrator.BeginMigration(ctx, p.BodyID, p.TargetSubstrate)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}
	return map[string]string{"migration_id": migrationID}, nil
}

func (s *Server) handleExecCommand(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p struct {
		BodyID  string   `json:"body_id"`
		Command []string `json:"command"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if p.BodyID == "" || len(p.Command) == 0 {
		return nil, &RPCError{Code: -32602, Message: "body_id and command are required"}
	}

	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32601, Message: "execute_command: not yet implemented"}
	}

	_, err := s.store.GetBody(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body not found: %s", p.BodyID)}
	}

	return nil, &RPCError{Code: -32601, Message: "execute_command: not yet implemented"}
}
