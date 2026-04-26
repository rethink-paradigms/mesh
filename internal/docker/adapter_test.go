package docker

import (
	"context"
	"testing"

	"github.com/rethink-paradigms/mesh/internal/adapter"
)

func TestNew(t *testing.T) {
	a := New()
	if a == nil {
		t.Fatal("New() returned nil")
	}
}

func TestCapabilities(t *testing.T) {
	a := New()
	caps := a.Capabilities()

	if !caps.ExportFilesystem {
		t.Error("ExportFilesystem should be true")
	}
	if !caps.ImportFilesystem {
		t.Error("ImportFilesystem should be true")
	}
	if !caps.Inspect {
		t.Error("Inspect should be true")
	}
}

func TestMapDockerState(t *testing.T) {
	tests := []struct {
		dockerStatus string
		want         adapter.BodyState
	}{
		{"created", adapter.StateCreated},
		{"running", adapter.StateRunning},
		{"paused", adapter.StateRunning},
		{"restarting", adapter.StateStarting},
		{"removing", adapter.StateError},
		{"exited", adapter.StateStopped},
		{"dead", adapter.StateError},
		{"unknown", adapter.StateError},
	}

	for _, tt := range tests {
		t.Run(tt.dockerStatus, func(t *testing.T) {
			got := mapDockerState(tt.dockerStatus)
			if got != tt.want {
				t.Errorf("mapDockerState(%q) = %q, want %q", tt.dockerStatus, got, tt.want)
			}
		})
	}
}

func TestDemuxDockerOutput_RawData(t *testing.T) {
	stdout, stderr := demuxDockerOutput([]byte("plain output"))
	if stdout != "plain output" {
		t.Errorf("stdout = %q, want %q", stdout, "plain output")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestDemuxDockerOutput_Empty(t *testing.T) {
	stdout, stderr := demuxDockerOutput([]byte{})
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestDemuxDockerOutput_StdoutFrame(t *testing.T) {
	data := append(
		[]byte{1, 0, 0, 0, 0, 0, 0, 5},
		[]byte("hello")...,
	)
	stdout, stderr := demuxDockerOutput(data)
	if stdout != "hello" {
		t.Errorf("stdout = %q, want %q", stdout, "hello")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestDemuxDockerOutput_StderrFrame(t *testing.T) {
	data := append(
		[]byte{2, 0, 0, 0, 0, 0, 0, 3},
		[]byte("err")...,
	)
	stdout, stderr := demuxDockerOutput(data)
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if stderr != "err" {
		t.Errorf("stderr = %q, want %q", stderr, "err")
	}
}

func TestDemuxDockerOutput_MixedFrames(t *testing.T) {
	stdoutPayload := []byte("out")
	stderrPayload := []byte("err")

	data := make([]byte, 0, 8+len(stdoutPayload)+8+len(stderrPayload))
	data = append(data, 1, 0, 0, 0, 0, 0, 0, 3)
	data = append(data, stdoutPayload...)
	data = append(data, 2, 0, 0, 0, 0, 0, 0, 3)
	data = append(data, stderrPayload...)

	stdout, stderr := demuxDockerOutput(data)
	if stdout != "out" {
		t.Errorf("stdout = %q, want %q", stdout, "out")
	}
	if stderr != "err" {
		t.Errorf("stderr = %q, want %q", stderr, "err")
	}
}

func TestDemuxDockerOutput_TruncatedFrame(t *testing.T) {
	data := append(
		[]byte{1, 0, 0, 0, 0, 0, 0, 10},
		[]byte("short")...,
	)
	stdout, _ := demuxDockerOutput(data)
	if stdout != "short" {
		t.Errorf("stdout = %q, want %q", stdout, "short")
	}
}

func TestDemuxDockerOutput_IncompleteHeader(t *testing.T) {
	data := []byte{1, 0, 0}
	stdout, stderr := demuxDockerOutput(data)
	if stdout != string(data) {
		t.Errorf("stdout = %q, want %q (raw passthrough for < 8 bytes)", stdout, string(data))
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestCreateErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	_, err := a.Create(ctx, adapter.BodySpec{Image: "alpine"})
	if err == nil {
		t.Error("Create should fail when Docker daemon is not available")
	}
}

func TestStartErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	err := a.Start(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("Start should fail when Docker daemon is not available")
	}
}

func TestStopErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	err := a.Stop(ctx, adapter.Handle("nonexistent"), adapter.StopOpts{})
	if err == nil {
		t.Error("Stop should fail when Docker daemon is not available")
	}
}

func TestDestroyErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	err := a.Destroy(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("Destroy should fail when Docker daemon is not available")
	}
}

func TestGetStatusErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	_, err := a.GetStatus(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("GetStatus should fail when Docker daemon is not available")
	}
}

func TestExecErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	_, err := a.Exec(ctx, adapter.Handle("nonexistent"), []string{"echo", "hi"})
	if err == nil {
		t.Error("Exec should fail when Docker daemon is not available")
	}
}

func TestExportFilesystemErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	_, err := a.ExportFilesystem(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("ExportFilesystem should fail when Docker daemon is not available")
	}
}

func TestImportFilesystemErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	err := a.ImportFilesystem(ctx, adapter.Handle("nonexistent"), nil, adapter.ImportOpts{})
	if err == nil {
		t.Error("ImportFilesystem should fail when Docker daemon is not available")
	}
}

func TestInspectErrorWithoutDocker(t *testing.T) {
	a := New()
	ctx := context.Background()

	_, err := a.Inspect(ctx, adapter.Handle("nonexistent"))
	if err == nil {
		t.Error("Inspect should fail when Docker daemon is not available")
	}
}

func TestCompileTimeInterfaceCheck(t *testing.T) {
	var _ adapter.SubstrateAdapter = (*Adapter)(nil)
}
