package nomad

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/rethink-paradigms/mesh/internal/adapter"
)

type Adapter struct {
	mu     sync.Mutex
	client *api.Client
	config Config
}

type Config struct {
	Address   string
	Token     string
	Region    string
	Namespace string
}

func New(cfg Config) *Adapter {
	return &Adapter{config: cfg}
}

func NewFromEnv() *Adapter {
	cfg := Config{
		Address:   os.Getenv("NOMAD_ADDR"),
		Token:     os.Getenv("NOMAD_TOKEN"),
		Region:    os.Getenv("NOMAD_REGION"),
		Namespace: os.Getenv("NOMAD_NAMESPACE"),
	}
	if cfg.Address == "" {
		cfg.Address = "http://127.0.0.1:4646"
	}
	return New(cfg)
}

func (a *Adapter) getClient() (*api.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client != nil {
		return a.client, nil
	}
	config := api.DefaultConfig()
	if a.config.Address != "" {
		config.Address = a.config.Address
	}
	if a.config.Region != "" {
		config.Region = a.config.Region
	}
	if a.config.Namespace != "" {
		config.Namespace = a.config.Namespace
	}
	if a.config.Token != "" {
		config.SecretID = a.config.Token
	}
	c, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("nomad: create client: %w", err)
	}
	a.client = c
	return c, nil
}

func (a *Adapter) Create(ctx context.Context, spec adapter.BodySpec) (adapter.Handle, error) {
	client, err := a.getClient()
	if err != nil {
		return "", err
	}

	jobID := generateJobID(spec.Image)

	job := &api.Job{
		ID:          &jobID,
		Name:        &jobID,
		Type:        strPtr("service"),
		Datacenters: []string{"dc1"},
		TaskGroups: []*api.TaskGroup{
			{
				Name:  strPtr("mesh"),
				Count: intPtr(0),
				Tasks: []*api.Task{
					{
						Name:   "body",
						Driver: "docker",
						Config: map[string]interface{}{
							"image":    spec.Image,
							"command":  spec.Cmd,
							"work_dir": spec.Workdir,
						},
						Env: spec.Env,
						Resources: &api.Resources{
							CPU:      intToPtr(spec.CPUShares),
							MemoryMB: intToPtr(spec.MemoryMB),
						},
					},
				},
			},
		},
	}

	resp, _, err := client.Jobs().Register(job, nil)
	if err != nil {
		return "", fmt.Errorf("nomad: submit job: %w", err)
	}
	if resp.Warnings != "" {
		_ = resp.Warnings
	}

	return adapter.Handle(jobID), nil
}

func (a *Adapter) Start(ctx context.Context, id adapter.Handle) error {
	client, err := a.getClient()
	if err != nil {
		return err
	}

	jobID := string(id)
	count := 1
	_, _, err = client.Jobs().Scale(jobID, "mesh", &count, "", false, nil, nil)
	if err != nil {
		return fmt.Errorf("nomad: start job %s: %w", jobID, err)
	}
	return nil
}

func (a *Adapter) Stop(ctx context.Context, id adapter.Handle, opts adapter.StopOpts) error {
	client, err := a.getClient()
	if err != nil {
		return err
	}

	jobID := string(id)
	count := 0
	_, _, err = client.Jobs().Scale(jobID, "mesh", &count, "", false, nil, nil)
	if err != nil {
		return fmt.Errorf("nomad: stop job %s: %w", jobID, err)
	}
	return nil
}

func (a *Adapter) Destroy(ctx context.Context, id adapter.Handle) error {
	client, err := a.getClient()
	if err != nil {
		return err
	}

	jobID := string(id)
	_, _, err = client.Jobs().Deregister(jobID, true, nil)
	if err != nil {
		return fmt.Errorf("nomad: destroy job %s: %w", jobID, err)
	}
	return nil
}

func (a *Adapter) GetStatus(ctx context.Context, id adapter.Handle) (adapter.BodyStatus, error) {
	client, err := a.getClient()
	if err != nil {
		return adapter.BodyStatus{}, err
	}

	jobID := string(id)
	allocs, _, err := client.Jobs().Allocations(jobID, true, nil)
	if err != nil {
		return adapter.BodyStatus{}, fmt.Errorf("nomad: get allocations for %s: %w", jobID, err)
	}

	if len(allocs) == 0 {
		return adapter.BodyStatus{State: adapter.StateCreated}, nil
	}

	var latest *api.AllocationListStub
	for i := range allocs {
		if latest == nil || allocs[i].CreateIndex > latest.CreateIndex {
			latest = allocs[i]
		}
	}

	state := mapNomadClientStatus(latest.ClientStatus)
	status := adapter.BodyStatus{
		State: state,
	}

	alloc, _, err := client.Allocations().Info(latest.ID, nil)
	if err == nil && alloc != nil {
		if alloc.TaskStates != nil {
			if ts, ok := alloc.TaskStates["body"]; ok {
				if !ts.StartedAt.IsZero() {
					status.StartedAt = ts.StartedAt
					if state == adapter.StateRunning {
						status.Uptime = time.Since(ts.StartedAt)
					}
				}
			}
		}
		if alloc.Resources != nil && alloc.Resources.MemoryMB != nil {
			status.MemoryMB = int64(*alloc.Resources.MemoryMB)
		}
	}

	return status, nil
}

func (a *Adapter) Exec(ctx context.Context, id adapter.Handle, cmd []string) (adapter.ExecResult, error) {
	client, err := a.getClient()
	if err != nil {
		return adapter.ExecResult{}, err
	}

	jobID := string(id)
	allocs, _, err := client.Jobs().Allocations(jobID, true, nil)
	if err != nil {
		return adapter.ExecResult{}, fmt.Errorf("nomad: get allocations for %s: %w", jobID, err)
	}
	if len(allocs) == 0 {
		return adapter.ExecResult{}, fmt.Errorf("nomad: no allocations found for job %s", jobID)
	}

	var allocID string
	for _, alloc := range allocs {
		if alloc.ClientStatus == "running" {
			allocID = alloc.ID
			break
		}
	}
	if allocID == "" {
		return adapter.ExecResult{}, fmt.Errorf("nomad: no running allocation for job %s", jobID)
	}

	stdout, stderr, exitCode, err := a.execViaAllocFS(ctx, client, allocID, cmd)
	if err != nil {
		return adapter.ExecResult{}, fmt.Errorf("nomad: exec in %s: %w", allocID, err)
	}

	return adapter.ExecResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}, nil
}

func (a *Adapter) execViaAllocFS(ctx context.Context, client *api.Client, allocID string, cmd []string) (string, string, int, error) {
	path := fmt.Sprintf("/v1/client/allocation/%s/exec", allocID)

	var resp struct {
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode int    `json:"exit_code"`
	}

	_, err := client.Raw().Query(path, &resp, &api.QueryOptions{})
	if err != nil {
		return "", "", -1, fmt.Errorf("alloc exec not available: %w", err)
	}

	return resp.Stdout, resp.Stderr, resp.ExitCode, nil
}

func (a *Adapter) ExportFilesystem(ctx context.Context, id adapter.Handle) (io.ReadCloser, error) {
	client, err := a.getClient()
	if err != nil {
		return nil, err
	}

	jobID := string(id)
	allocs, _, err := client.Jobs().Allocations(jobID, true, nil)
	if err != nil {
		return nil, fmt.Errorf("nomad: get allocations for %s: %w", jobID, err)
	}
	if len(allocs) == 0 {
		return nil, fmt.Errorf("nomad: no allocations found for job %s", jobID)
	}

	allocID := allocs[0].ID
	path := fmt.Sprintf("/v1/client/fs/cat/%s?path=/alloc/data/body.tar", allocID)

	var buf strings.Builder
	_, err = client.Raw().Query(path, &buf, &api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("nomad: export filesystem from %s: %w", allocID, err)
	}

	return io.NopCloser(strings.NewReader(buf.String())), nil
}

func (a *Adapter) ImportFilesystem(ctx context.Context, id adapter.Handle, tarball io.Reader, opts adapter.ImportOpts) error {
	client, err := a.getClient()
	if err != nil {
		return err
	}

	jobID := string(id)
	allocs, _, err := client.Jobs().Allocations(jobID, true, nil)
	if err != nil {
		return fmt.Errorf("nomad: get allocations for %s: %w", jobID, err)
	}
	if len(allocs) == 0 {
		return fmt.Errorf("nomad: no allocations found for job %s", jobID)
	}

	allocID := allocs[0].ID

	data, err := io.ReadAll(tarball)
	if err != nil {
		return fmt.Errorf("nomad: read tarball: %w", err)
	}

	path := fmt.Sprintf("/v1/client/fs/write/%s", allocID)
	_, err = client.Raw().Write(path, data, nil, &api.WriteOptions{})
	if err != nil {
		return fmt.Errorf("nomad: import filesystem to %s: %w", allocID, err)
	}

	_ = opts
	return nil
}

func (a *Adapter) Inspect(ctx context.Context, id adapter.Handle) (adapter.ContainerMetadata, error) {
	client, err := a.getClient()
	if err != nil {
		return adapter.ContainerMetadata{}, err
	}

	jobID := string(id)
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return adapter.ContainerMetadata{}, fmt.Errorf("nomad: inspect job %s: %w", jobID, err)
	}

	meta := adapter.ContainerMetadata{}
	if len(job.TaskGroups) > 0 && len(job.TaskGroups[0].Tasks) > 0 {
		task := job.TaskGroups[0].Tasks[0]
		meta.Image = task.Config["image"].(string)
		meta.Env = task.Env
		if cmd, ok := task.Config["command"].([]string); ok {
			meta.Cmd = cmd
		}
		if wd, ok := task.Config["work_dir"].(string); ok {
			meta.Workdir = wd
		}
	}
	return meta, nil
}

func (a *Adapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{
		ExportFilesystem: true,
		ImportFilesystem: true,
		Inspect:          true,
	}
}

func (a *Adapter) SubstrateName() string {
	return "nomad"
}

func (a *Adapter) IsHealthy(ctx context.Context) bool {
	client, err := a.getClient()
	if err != nil {
		return false
	}
	_, err = client.Agent().Self()
	return err == nil
}

func generateJobID(image string) string {
	id := strings.ReplaceAll(image, "/", "-")
	id = strings.ReplaceAll(id, ":", "-")
	id = strings.ReplaceAll(id, ".", "-")
	return fmt.Sprintf("mesh-%s-%d", id, time.Now().Unix())
}

func mapNomadClientStatus(status string) adapter.BodyState {
	switch status {
	case "pending":
		return adapter.StateStarting
	case "running":
		return adapter.StateRunning
	case "failed":
		return adapter.StateError
	case "lost":
		return adapter.StateError
	case "complete":
		return adapter.StateStopped
	case "terminal":
		return adapter.StateStopped
	default:
		return adapter.StateCreated
	}
}

func strPtr(s string) *string {
	return &s
}

func intPtr(n int) *int {
	return &n
}

func intToPtr(n int) *int {
	return &n
}

type bytesWriter struct {
	buf strings.Builder
}

func (w *bytesWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *bytesWriter) String() string {
	return w.buf.String()
}
