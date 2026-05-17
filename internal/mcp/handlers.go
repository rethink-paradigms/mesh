package mcp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/restore"
	"github.com/rethink-paradigms/mesh/internal/service"
	"github.com/rethink-paradigms/mesh/internal/store"
)

// mapServiceError converts domain errors from BodyService into RPC error codes.
func mapServiceError(err error) *RPCError {
	var notFound *service.NotFoundError
	if errors.As(err, &notFound) {
		return &RPCError{Code: -32001, Message: err.Error()}
	}
	var conflict *service.ConflictError
	if errors.As(err, &conflict) {
		return &RPCError{Code: -32002, Message: err.Error()}
	}
	var validation *service.ValidationError
	if errors.As(err, &validation) {
		return &RPCError{Code: -32602, Message: err.Error()}
	}
	return &RPCError{Code: -32603, Message: err.Error()}
}

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
	s.RegisterTool("list_capabilities", s.handleListCapabilities, ToolDefinition{
		Name:        "list_capabilities",
		Description: "List daemon capabilities including orchestrators, providers, features, and limits.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	})
	s.RegisterTool("daemon_status", s.handleDaemonStatus, ToolDefinition{
		Name:        "daemon_status",
		Description: "Get full daemon status including bodies, ports, ingress, and capacity.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	})
	s.RegisterTool("install_agent", s.handleInstallAgent, ToolDefinition{
		Name:        "install_agent",
		Description: "Install a built-in agent from a descriptor.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"agent_type":{"type":"string"},"name":{"type":"string"},"env":{"type":"object","additionalProperties":{"type":"string"}}},"required":["agent_type","name"]}`),
	})
}

func (s *Server) handlePing(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"pong": true}, nil
}

func (s *Server) handleListBodies(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
	}
	bodies, err := s.svc.List(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return bodies, nil
}

func (s *Server) handleGetBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
	}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: id"}
	}
	body, err := s.svc.Get(ctx, p.ID)
	if err != nil {
		return nil, mapServiceError(err)
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
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
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

	spec := orchestrator.BodySpec{
		Image:     p.Image,
		Workdir:   p.Workdir,
		Env:       p.Env,
		Cmd:       p.Cmd,
		MemoryMB:  p.MemoryMB,
		CPUShares: p.CPUShares,
	}

	b, err := s.svc.Create(ctx, p.Name, p.Image, spec)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return map[string]interface{}{
		"id":        b.ID,
		"name":      b.Name,
		"state":     string(b.State),
		"handle":    string(b.InstanceID),
		"substrate": b.Substrate,
	}, nil
}

func (s *Server) handleDeleteBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
	}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: id"}
	}
	if err := s.svc.Destroy(ctx, p.ID); err != nil {
		return nil, mapServiceError(err)
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
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
	}
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

	timeout := 30 * time.Second
	if p.TimeoutSeconds > 0 {
		timeout = time.Duration(p.TimeoutSeconds) * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := s.svc.Exec(execCtx, p.BodyID, p.Command)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, &RPCError{Code: -32603, Message: fmt.Sprintf("exec timeout after %v", timeout)}
		}
		return nil, mapServiceError(err)
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
	if body.State != orchestrator.StateRunning {
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

	sidecarJSON := fmt.Sprintf(`{"checksum":"%s","size":%d,"created_at":"%s"}`, digest, stat.Size(), time.Now().UTC().Format(time.RFC3339))
	if err := s.store.CreateSnapshot(ctx, snapID, p.BodyID, sidecarJSON, storagePath, stat.Size()); err != nil {
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
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	if err := s.svc.Start(ctx, p.BodyID); err != nil {
		return nil, mapServiceError(err)
	}

	b, err := s.svc.Get(ctx, p.BodyID)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return map[string]interface{}{
		"id":    b.ID,
		"name":  b.Name,
		"state": string(b.State),
	}, nil
}

func (s *Server) handleStopBody(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	if err := s.svc.Stop(ctx, p.BodyID); err != nil {
		return nil, mapServiceError(err)
	}

	b, err := s.svc.Get(ctx, p.BodyID)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return map[string]interface{}{
		"id":    b.ID,
		"name":  b.Name,
		"state": string(b.State),
	}, nil
}

func (s *Server) handleGetBodyLogs(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
		Tail   int    `json:"tail,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	tailLines := 100
	if p.Tail > 0 {
		tailLines = p.Tail
	}

	result, err := s.svc.Exec(ctx, p.BodyID, []string{"tail", "-n", fmt.Sprintf("%d", tailLines), "/var/log/mesh.log"})
	if err != nil {
		return nil, mapServiceError(err)
	}

	return map[string]interface{}{
		"body_id": p.BodyID,
		"logs":    result.Stdout,
		"tail":    tailLines,
	}, nil
}

func (s *Server) handleGetBodyStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.svc == nil {
		return nil, &RPCError{Code: -32603, Message: "body service not available"}
	}
	var p struct {
		BodyID string `json:"body_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.BodyID == "" {
		return nil, &RPCError{Code: -32602, Message: "missing required parameter: body_id"}
	}

	body, err := s.svc.Get(ctx, p.BodyID)
	if err != nil {
		return nil, mapServiceError(err)
	}

	status, err := s.svc.GetStatus(ctx, p.BodyID)
	if err != nil {
		return nil, mapServiceError(err)
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

func (s *Server) handleListCapabilities(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var orchCaps []map[string]interface{}
	if s.orchRegistry != nil {
		for _, name := range s.orchRegistry.List() {
			adapter, err := s.orchRegistry.Open(name)
			healthy := err == nil && adapter.IsHealthy(ctx)
			orchCaps = append(orchCaps, map[string]interface{}{
				"name":    name,
				"healthy": healthy,
			})
		}
	}
	if orchCaps == nil {
		orchCaps = []map[string]interface{}{}
	}

	providers := getMCPProviders()

	features := s.features
	if features == nil {
		features = make(map[string]bool)
	}

	tier := s.tier
	if tier == "" {
		tier = "solo"
	}

	maxBodies := s.maxBodies
	if maxBodies == 0 {
		maxBodies = 10
	}
	maxSnapshots := s.maxSnapshots
	if maxSnapshots == 0 {
		maxSnapshots = 5
	}

	return map[string]interface{}{
		"version":       s.version,
		"tier":          tier,
		"orchestrators": orchCaps,
		"providers":     json.RawMessage(providers),
		"features":      features,
		"limits": map[string]int{
			"max_bodies":    maxBodies,
			"max_snapshots": maxSnapshots,
		},
	}, nil
}

func (s *Server) handleDaemonStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	status := map[string]interface{}{}

	// Daemon info
	uptimeSec := int64(0)
	startTime := ""
	if !s.startedAt.IsZero() {
		uptimeSec = int64(time.Since(s.startedAt).Seconds())
		startTime = s.startedAt.Format(time.RFC3339)
	}
	status["daemon"] = map[string]interface{}{
		"version":        s.version,
		"uptime_seconds": uptimeSec,
		"start_time":     startTime,
	}

	// Tier
	tier := s.tier
	if tier == "" {
		tier = "solo"
	}
	status["tier"] = tier

	// Bodies
	bodiesInfo := map[string]interface{}{
		"total":   0,
		"running": 0,
		"stopped": 0,
		"error":   0,
		"list":    []map[string]interface{}{},
	}
	if s.store != nil {
		records, err := s.store.ListBodies(ctx)
		if err == nil {
			running, stopped, errorCount := 0, 0, 0
			list := make([]map[string]interface{}, 0, len(records))
			for _, rec := range records {
				switch rec.State {
				case orchestrator.StateRunning:
					running++
				case orchestrator.StateStopped, orchestrator.StateStopping:
					stopped++
				case orchestrator.StateError:
					errorCount++
				}
				list = append(list, map[string]interface{}{
					"id":    rec.ID,
					"name":  rec.Name,
					"state": string(rec.State),
				})
			}
			bodiesInfo = map[string]interface{}{
				"total":   len(records),
				"running": running,
				"stopped": stopped,
				"error":   errorCount,
				"list":    list,
			}
		}
	}
	status["bodies"] = bodiesInfo

	// Ports
	status["ports"] = map[string]interface{}{
		"used":       0,
		"free":       0,
		"pool_start": 9000,
		"pool_end":   9999,
	}

	// Ingress
	routeCount := 0
	if s.ingress != nil {
		if routes, err := s.ingress.ListRoutes(ctx); err == nil {
			routeCount = len(routes)
		}
	}
	status["ingress"] = map[string]interface{}{
		"route_count": routeCount,
	}

	// Capacity
	status["capacity"] = collectMCPCapacity()

	return status, nil
}

func collectMCPCapacity() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memTotalMB := int64(0)
	f, err := os.Open("/proc/meminfo")
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "MemTotal:") {
				var kb int64
				if _, err := fmt.Sscanf(line, "MemTotal: %d kB", &kb); err == nil {
					memTotalMB = kb / 1024
				}
			}
		}
		f.Close()
	}

	diskGBUsed, diskGBTotal := 0.0, 0.0
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		totalBytes := stat.Blocks * uint64(stat.Bsize)
		availBytes := stat.Bavail * uint64(stat.Bsize)
		diskGBTotal = float64(totalBytes) / (1024 * 1024 * 1024)
		diskGBUsed = float64(totalBytes-availBytes) / (1024 * 1024 * 1024)
	}

	return map[string]interface{}{
		"cpu_percent":     0.0,
		"memory_mb_used":  int64(m.Alloc) / 1024 / 1024,
		"memory_mb_total": memTotalMB,
		"disk_gb_used":    diskGBUsed,
		"disk_gb_total":   diskGBTotal,
	}
}

// getMCPProviders runs mesh-provision providers --output json and returns the raw JSON bytes.
// If mesh-provision is not available, returns a JSON status: "unavailable".
func getMCPProviders() json.RawMessage {
	cmd := exec.Command("mesh-provision", "providers", "--output", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fallback, _ := json.Marshal(map[string]interface{}{
			"status":    "unavailable",
			"providers": []interface{}{},
		})
		return fallback
	}

	return json.RawMessage(stdout.Bytes())
}

func (s *Server) handleInstallAgent(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.installer == nil {
		return nil, &RPCError{Code: -32603, Message: "installer not available"}
	}
	var p struct {
		AgentType      string            `json:"agent_type"`
		Name           string            `json:"name"`
		Env            map[string]string `json:"env,omitempty"`
		DescriptorYAML string            `json:"descriptor,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &RPCError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if p.AgentType == "" || p.Name == "" {
		return nil, &RPCError{Code: -32602, Message: "agent_type and name are required"}
	}

	result, err := s.installer.Install(ctx, p.AgentType, p.Name, p.Env, p.DescriptorYAML)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return map[string]interface{}{
		"body_id":         result.BodyID,
		"name":            result.Name,
		"access_urls":     result.AccessURLs,
		"allocated_ports": result.AllocatedPorts,
	}, nil
}
