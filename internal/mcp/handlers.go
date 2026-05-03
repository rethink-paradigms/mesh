package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	"github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/restore"
	"github.com/rethink-paradigms/mesh/internal/store"
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
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"image":{"type":"string"},"substrate":{"type":"string","description":"Target substrate name. Optional when exactly one orchestrator is registered."},"workdir":{"type":"string"},"env":{"type":"object","additionalProperties":{"type":"string"}},"cmd":{"type":"array","items":{"type":"string"}},"memory_mb":{"type":"integer"},"cpu_shares":{"type":"integer"}},"required":["name","image"]}`),
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
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"},"command":{"type":"array","items":{"type":"string"}},"timeout_seconds":{"type":"integer","description":"Maximum time to wait for command execution in seconds. Default: 30."}},"required":["body_id","command"]}`),
	})
	s.RegisterTool("create_snapshot", s.handleCreateSnapshot, ToolDefinition{
		Name:        "create_snapshot",
		Description: "Create a filesystem snapshot of a running body.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"},"label":{"type":"string"}},"required":["body_id"]}`),
	})
	s.RegisterTool("list_snapshots", s.handleListSnapshots, ToolDefinition{
		Name:        "list_snapshots",
		Description: "List snapshots, optionally filtered by body_id.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"}}}`),
	})
	s.RegisterTool("restore_body", s.handleRestoreBody, ToolDefinition{
		Name:        "restore_body",
		Description: "Restore a body from a snapshot.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"snapshot_id":{"type":"string"},"target_substrate":{"type":"string"}},"required":["snapshot_id"]}`),
	})
	s.RegisterTool("start_body", s.handleStartBody, ToolDefinition{
		Name:        "start_body",
		Description: "Start a stopped body.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"}},"required":["body_id"]}`),
	})
	s.RegisterTool("stop_body", s.handleStopBody, ToolDefinition{
		Name:        "stop_body",
		Description: "Stop a running body.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"}},"required":["body_id"]}`),
	})
	s.RegisterTool("get_body_logs", s.handleGetBodyLogs, ToolDefinition{
		Name:        "get_body_logs",
		Description: "Get logs from a running body.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"},"tail":{"type":"integer","description":"Number of lines to return from the end. Default: 100."}},"required":["body_id"]}`),
	})
	s.RegisterTool("get_body_status", s.handleGetBodyStatus, ToolDefinition{
		Name:        "get_body_status",
		Description: "Get runtime status of a body (state, uptime, memory, cpu).",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"body_id":{"type":"string"}},"required":["body_id"]}`),
	})
	s.RegisterTool("list_plugins", s.handleListPlugins, ToolDefinition{
		Name:        "list_plugins",
		Description: "List all loaded plugins with name, version, state, and health status.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	})
	s.RegisterTool("plugin_health", s.handlePluginHealth, ToolDefinition{
		Name:        "plugin_health",
		Description: "Get detailed health information for a specific plugin.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"plugin_name":{"type":"string"}},"required":["plugin_name"]}`),
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
		Substrate string            `json:"substrate,omitempty"`
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

	if s.orchRegistry != nil {
		names := s.orchRegistry.List()
		if p.Substrate == "" {
			switch len(names) {
			case 0:
				return nil, &RPCError{Code: -32603, Message: "no substrate available: no orchestrators registered"}
			case 1:
				p.Substrate = names[0]
			default:
				return nil, &RPCError{Code: -32602, Message: fmt.Sprintf("substrate required when multiple orchestrators registered; available: %v", names)}
			}
		} else {
			if _, err := s.orchRegistry.Open(p.Substrate); err != nil {
				return nil, &RPCError{Code: -32602, Message: fmt.Sprintf("unknown substrate %q: %v", p.Substrate, err)}
			}
		}
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
		"id":        b.ID,
		"name":      b.Name,
		"state":     string(b.State),
		"handle":    string(b.InstanceID),
		"substrate": p.Substrate,
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
		BodyID         string   `json:"body_id"`
		Command        []string `json:"command"`
		TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if p.BodyID == "" || len(p.Command) == 0 {
		return nil, &RPCError{Code: -32602, Message: "body_id and command are required"}
	}

	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}

	body, err := s.store.GetBody(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body not found: %s", p.BodyID)}
	}

	if adapter.BodyState(body.State) != adapter.StateRunning {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body %s is not running (state: %s)", p.BodyID, body.State)}
	}

	timeout := 30 * time.Second
	if p.TimeoutSeconds > 0 {
		timeout = time.Duration(p.TimeoutSeconds) * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := s.bodyMgr.Exec(execCtx, p.BodyID, p.Command)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("exec timeout after %v", timeout)}
		}
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("exec failed: %v", err)}
	}

	return map[string]interface{}{
		"stdout":    result.Stdout,
		"stderr":    result.Stderr,
		"exit_code": result.ExitCode,
	}, nil
}

func (s *Server) handleCreateSnapshot(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
		Label  string `json:"label,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	body, err := s.store.GetBody(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body not found: %s", p.BodyID)}
	}
	if body.State != adapter.StateRunning {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body not running: %s (state: %s)", p.BodyID, body.State)}
	}

	snapID := fmt.Sprintf("%s-%s", p.BodyID, time.Now().Format("20060102-150405"))
	if p.Label != "" {
		snapID = fmt.Sprintf("%s-%s", p.BodyID, p.Label)
	}

	storagePath := fmt.Sprintf("/tmp/mesh-snapshot-%s.tar.zst", snapID)

	rc, err := s.bodyMgr.ExportFilesystem(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("export filesystem failed: %v", err)}
	}
	defer rc.Close()

	outFile, err := os.Create(storagePath)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("create output file: %v", err)}
	}
	defer outFile.Close()

	hasher := sha256.New()
	mw := io.MultiWriter(outFile, hasher)
	zw, err := zstd.NewWriter(mw)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("create zstd writer: %v", err)}
	}

	if _, err := io.Copy(zw, rc); err != nil {
		zw.Close()
		os.Remove(storagePath)
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("compress pipeline: %v", err)}
	}

	if err := zw.Close(); err != nil {
		os.Remove(storagePath)
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("flush zstd: %v", err)}
	}

	digest := hex.EncodeToString(hasher.Sum(nil))
	shaPath := storagePath + ".sha256"
	if err := os.WriteFile(shaPath, []byte(digest+"\n"), 0o644); err != nil {
		os.Remove(storagePath)
		os.Remove(shaPath)
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("write sha256 sidecar: %v", err)}
	}

	stat, err := os.Stat(storagePath)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("stat output: %v", err)}
	}

	manifestJSON := fmt.Sprintf(`{"checksum":"%s","size":%d,"created_at":"%s"}`, digest, stat.Size(), time.Now().UTC().Format(time.RFC3339))
	if err := s.store.CreateSnapshot(ctx, snapID, p.BodyID, manifestJSON, storagePath, stat.Size()); err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("persist snapshot: %v", err)}
	}

	return map[string]interface{}{
		"id":         snapID,
		"body_id":    p.BodyID,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"size_bytes": stat.Size(),
		"sha256":     digest,
	}, nil
}

func (s *Server) handleListSnapshots(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p struct {
		BodyID string `json:"body_id,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
	}

	if p.BodyID != "" {
		snaps, err := s.store.ListSnapshots(ctx, p.BodyID)
		if err != nil {
			return nil, &RPCError{Code: -32603, Message: err.Error()}
		}
		return snaps, nil
	}

	bodies, err := s.store.ListBodies(ctx)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}

	var allSnaps []*store.SnapshotRecord
	for _, b := range bodies {
		snaps, err := s.store.ListSnapshots(ctx, b.ID)
		if err != nil {
			return nil, &RPCError{Code: -32603, Message: err.Error()}
		}
		allSnaps = append(allSnaps, snaps...)
	}
	return allSnaps, nil
}

func (s *Server) handleRestoreBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}
	var p struct {
		SnapshotID      string `json:"snapshot_id"`
		TargetSubstrate string `json:"target_substrate,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.SnapshotID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: snapshot_id"}
	}

	snap, err := s.store.GetSnapshot(ctx, p.SnapshotID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("snapshot not found: %s", p.SnapshotID)}
	}

	_, err = s.store.GetBody(ctx, snap.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body not found: %s", snap.BodyID)}
	}

	targetDir := fmt.Sprintf("/tmp/mesh-restore-%s", snap.BodyID)
	if err := restore.RestoreFromStore(ctx, s.store, p.SnapshotID, targetDir, restore.RestoreOpts{}); err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("restore failed: %v", err)}
	}

	return map[string]interface{}{
		"restored":         true,
		"snapshot_id":      p.SnapshotID,
		"body_id":          snap.BodyID,
		"target_dir":       targetDir,
		"target_substrate": p.TargetSubstrate,
	}, nil
}

func (s *Server) handleStartBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	if err := s.bodyMgr.Start(ctx, p.BodyID); err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}

	b, err := s.bodyMgr.Get(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}

	return map[string]interface{}{
		"id":    b.ID,
		"name":  b.Name,
		"state": string(b.State),
	}, nil
}

func (s *Server) handleStopBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.bodyMgr.Stop(ctx, p.BodyID, adapter.StopOpts{Timeout: 30 * time.Second}); err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}

	b, err := s.bodyMgr.Get(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: err.Error()}
	}

	return map[string]interface{}{
		"id":    b.ID,
		"name":  b.Name,
		"state": string(b.State),
	}, nil
}

func (s *Server) handleGetBodyLogs(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
		Tail   int    `json:"tail,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	body, err := s.store.GetBody(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body not found: %s", p.BodyID)}
	}

	if adapter.BodyState(body.State) != adapter.StateRunning {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body %s is not running (state: %s)", p.BodyID, body.State)}
	}

	tailLines := 100
	if p.Tail > 0 {
		tailLines = p.Tail
	}

	result, err := s.bodyMgr.Exec(ctx, p.BodyID, []string{"tail", "-n", fmt.Sprintf("%d", tailLines), "/var/log/mesh.log"})
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("failed to get logs: %v", err)}
	}

	return map[string]interface{}{
		"body_id": p.BodyID,
		"logs":    result.Stdout,
		"tail":    tailLines,
	}, nil
}

func (s *Server) handleGetBodyStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.bodyMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "body manager not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	body, err := s.store.GetBody(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("body not found: %s", p.BodyID)}
	}

	status, err := s.bodyMgr.GetStatus(ctx, p.BodyID)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("failed to get status: %v", err)}
	}

	return map[string]interface{}{
		"id":         body.ID,
		"name":       body.Name,
		"state":      string(status.State),
		"uptime_sec": int64(status.Uptime.Seconds()),
		"memory_mb":  status.MemoryMB,
		"cpu_usage":  status.CPUPercent,
	}, nil
}

func (s *Server) handleListPlugins(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.pluginMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "plugin manager not available"}
	}

	names := s.pluginMgr.List()
	plugins := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		rec := s.pluginMgr.Get(name)
		if rec == nil {
			continue
		}
		plugins = append(plugins, map[string]interface{}{
			"name":    rec.Meta.Name,
			"version": rec.Meta.Version,
			"state":   string(rec.GetState()),
			"healthy": rec.GetState() == plugin.StateHealthy,
		})
	}
	return plugins, nil
}

func (s *Server) handlePluginHealth(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.pluginMgr == nil {
		return nil, &RPCError{Code: -32603, Message: "plugin manager not available"}
	}
	var p struct {
		PluginName string `json:"plugin_name"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.PluginName == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: plugin_name"}
	}

	rec := s.pluginMgr.Get(p.PluginName)
	if rec == nil {
		return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("plugin not found: %s", p.PluginName)}
	}

	return map[string]interface{}{
		"name":        rec.Meta.Name,
		"version":     rec.Meta.Version,
		"state":       string(rec.GetState()),
		"healthy":     rec.GetState() == plugin.StateHealthy,
		"fail_count":  rec.GetFailCount(),
		"retry_count": rec.GetRetryCount(),
		"description": rec.Meta.Description,
		"author":      rec.Meta.Author,
	}, nil
}
