package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
	"github.com/rethink-paradigms/mesh/internal/service"
)

func TestGetBodyNotFoundErrorCode(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := service.NewBodyService(bm, s, nil)

	h := newHarness(t, s)
	h.srv.SetBodyService(svc)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "get_body", "arguments": map[string]interface{}{"id": "nonexistent"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	code := rpcErr["code"].(float64)
	if code != -32001 {
		t.Fatalf("error code = %v, want -32001 (NotFoundError)", code)
	}
	msg := rpcErr["message"].(string)
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestDeleteBodyConflictErrorCode(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := service.NewBodyService(bm, s, nil)
	ctx := context.Background()

	created, err := bm.Create(ctx, "conflict-test", orchestrator.BodySpec{Image: "alpine"})
	if err != nil {
		t.Fatalf("create body: %v", err)
	}

	h := newHarness(t, s)
	h.srv.SetBodyService(svc)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "delete_body", "arguments": map[string]interface{}{"id": created.ID}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	code := rpcErr["code"].(float64)
	if code != -32002 {
		t.Fatalf("error code = %v, want -32002 (ConflictError)", code)
	}
}

func TestCreateBodyValidationErrorCode(t *testing.T) {
	s := tempStore(t)
	bm := testBodyManager(t, s)
	svc := service.NewBodyService(bm, s, nil)

	h := newHarness(t, s)
	h.srv.SetBodyService(svc)
	defer h.close()

	h.send(t, Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  rawMessage(t, map[string]interface{}{"name": "create_body", "arguments": map[string]interface{}{"name": "test"}}),
	})

	resp := h.readResponse(t)
	rpcErr := resp["error"].(map[string]interface{})
	code := rpcErr["code"].(float64)
	if code != -32602 {
		t.Fatalf("error code = %v, want -32602 (ValidationError)", code)
	}
}

func TestMapServiceErrorNotFound(t *testing.T) {
	err := &service.NotFoundError{ID: "abc"}
	rpcErr := mapServiceError(err)
	if rpcErr.Code != -32001 {
		t.Fatalf("code = %d, want -32001", rpcErr.Code)
	}
}

func TestMapServiceErrorConflict(t *testing.T) {
	err := &service.ConflictError{State: "Running", Required: "Stopped"}
	rpcErr := mapServiceError(err)
	if rpcErr.Code != -32002 {
		t.Fatalf("code = %d, want -32002", rpcErr.Code)
	}
}

func TestMapServiceErrorValidation(t *testing.T) {
	err := &service.ValidationError{Field: "name", Message: "required"}
	rpcErr := mapServiceError(err)
	if rpcErr.Code != -32602 {
		t.Fatalf("code = %d, want -32602", rpcErr.Code)
	}
}

func TestMapServiceErrorGeneric(t *testing.T) {
	err := json.Unmarshal([]byte("invalid"), &struct{}{})
	rpcErr := mapServiceError(err)
	if rpcErr.Code != -32603 {
		t.Fatalf("code = %d, want -32603", rpcErr.Code)
	}
}
