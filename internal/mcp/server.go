// Package mcp provides MCP server (stdio transport) with tool registration and JSON-RPC request routing.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rethink-paradigms/mesh/internal/body"
	"github.com/rethink-paradigms/mesh/internal/plugin"
	"github.com/rethink-paradigms/mesh/internal/store"
)

// ToolHandler is a function that handles a tool call with given params.
type ToolHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)

// ToolDefinition describes a registered tool for tools/list responses.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// Server is an MCP server that reads JSON-RPC requests from an io.Reader and writes responses to an io.Writer.
type Server struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	store   *store.Store
	tools   map[string]ToolHandler
	defs    map[string]ToolDefinition
	bodyMgr       *body.BodyManager
	migrator     *body.MigrationCoordinator
	pluginMgr    *plugin.PluginManager

	reader io.Reader
	writer io.Writer
}

// SetBodyManager sets the body manager for lifecycle operations.
func (s *Server) SetBodyManager(mgr *body.BodyManager) {
	s.bodyMgr = mgr
}

// SetMigrator sets the migration coordinator for migration operations.
func (s *Server) SetMigrator(m *body.MigrationCoordinator) {
	s.migrator = m
}

// SetPluginManager sets the plugin manager for plugin operations.
func (s *Server) SetPluginManager(m *plugin.PluginManager) {
	s.pluginMgr = m
}

// New creates a new MCP server backed by the given store.
// It uses os.Stdin/os.Stdout for IO. Use NewWithIO for testing.
func New(s *store.Store) *Server {
	srv := &Server{
		store:  s,
		tools:  make(map[string]ToolHandler),
		defs:   make(map[string]ToolDefinition),
		reader: os.Stdin,
		writer: os.Stdout,
	}
	srv.registerTools()
	return srv
}

// NewWithIO creates a new MCP server with custom reader/writer for testing.
func NewWithIO(s *store.Store, r io.Reader, w io.Writer) *Server {
	srv := &Server{
		store:  s,
		tools:  make(map[string]ToolHandler),
		defs:   make(map[string]ToolDefinition),
		reader: r,
		writer: w,
	}
	srv.registerTools()
	return srv
}

// RegisterTool adds a tool handler and definition.
func (s *Server) RegisterTool(name string, handler ToolHandler, def ToolDefinition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = handler
	s.defs[name] = def
}

// Start begins reading JSON-RPC requests and dispatching them.
// Blocks until the reader returns EOF or the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	scanner := bufio.NewScanner(s.reader)
	// Allow lines up to 1MB
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("mcp: read: %w", err)
			}
			return nil // EOF
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeResponse(Response{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &RPCError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		s.handle(ctx, req)
	}
}

// Stop cancels the server context, causing Start to return.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
	return nil
}

// handle routes a JSON-RPC request to the appropriate handler.
func (s *Server) handle(ctx context.Context, req Request) {
	switch req.Method {
	case "initialize":
		s.writeResult(req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "mesh",
				"version": "0.1.0",
			},
		})

	case "tools/list":
		s.mu.Lock()
		tools := make([]ToolDefinition, 0, len(s.defs))
		for _, def := range s.defs {
			tools = append(tools, def)
		}
		s.mu.Unlock()
		s.writeResult(req.ID, map[string]interface{}{
			"tools": tools,
		})

	case "tools/call":
		s.handleToolCall(ctx, req)

	default:
		s.writeError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

// handleToolCall dispatches a tools/call request to the registered handler.
func (s *Server) handleToolCall(ctx context.Context, req Request) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		s.writeError(req.ID, -32602, "Invalid params: "+err.Error())
		return
	}

	s.mu.Lock()
	handler, ok := s.tools[p.Name]
	s.mu.Unlock()

	if !ok {
		s.writeError(req.ID, -32601, fmt.Sprintf("Tool not found: %s", p.Name))
		return
	}

	result, err := handler(ctx, p.Arguments)
	if err != nil {
		if rpcErr, ok := err.(*RPCError); ok {
			s.writeError(req.ID, rpcErr.Code, rpcErr.Message)
		} else {
			s.writeError(req.ID, -32603, "Internal error: "+err.Error())
		}
		return
	}

	// Wrap result in MCP content format
	s.writeResult(req.ID, map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": marshalJSON(result)},
		},
	})
}

// writeResult writes a successful JSON-RPC response.
func (s *Server) writeResult(id interface{}, result interface{}) {
	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

// writeError writes an error JSON-RPC response.
func (s *Server) writeError(id interface{}, code int, message string) {
	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	})
}

// writeResponse marshals and writes a response as a single JSON line.
func (s *Server) writeResponse(resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	s.writer.Write(append(data, '\n'))
}

// marshalJSON converts a value to JSON string, returning "{}" on error.
func marshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
