// Package registry provides an S3-backed registry plugin for Mesh.
// It implements the MeshPlugin interface and provides streaming push/pull/verify
// operations for snapshot storage.
package registry

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rethink-paradigms/mesh/internal/adapter"
	meshplugin "github.com/rethink-paradigms/mesh/internal/plugin"
)

// RegistryConfig holds S3 registry configuration.
// Fields are read from mesh config with env var fallback.
type RegistryConfig struct {
	Type            string
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
}

// S3RegistryPlugin implements the MeshPlugin interface for S3 snapshot storage.
type S3RegistryPlugin struct {
	cfg    RegistryConfig
	client *s3.Client
}

// NewS3RegistryPlugin creates a new S3 registry plugin from config.
func NewS3RegistryPlugin(cfg RegistryConfig) (*S3RegistryPlugin, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("registry: bucket is required")
	}

	// Apply env var fallbacks
	if cfg.Region == "" {
		cfg.Region = os.Getenv("AWS_REGION")
		if cfg.Region == "" {
			cfg.Region = os.Getenv("AWS_DEFAULT_REGION")
		}
	}
	if cfg.AccessKeyID == "" {
		cfg.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if cfg.SecretAccessKey == "" {
		cfg.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = os.Getenv("AWS_ENDPOINT_URL_S3")
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("registry: load AWS config: %w", err)
	}

	// If explicit credentials provided, override
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		awsCfg.Credentials = credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)
	}

	opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			// For non-AWS endpoints (MinIO, etc.), disable path style
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, opts...)

	return &S3RegistryPlugin{
		cfg:    cfg,
		client: client,
	}, nil
}

// PluginInfo returns metadata about this plugin.
func (p *S3RegistryPlugin) PluginInfo(ctx context.Context) (meshplugin.PluginMeta, error) {
	return meshplugin.PluginMeta{
		Name:        "s3-registry",
		Version:     "1.0.0",
		Description: "S3-backed snapshot registry plugin",
		Author:      "mesh",
	}, nil
}

// GetAdapter returns the S3 registry adapter.
func (p *S3RegistryPlugin) GetAdapter(ctx context.Context) (adapter.SubstrateAdapter, error) {
	return &S3RegistryAdapter{plugin: p}, nil
}

// S3RegistryAdapter implements adapter.SubstrateAdapter for the S3 registry.
// It delegates snapshot operations to the plugin.
type S3RegistryAdapter struct {
	plugin *S3RegistryPlugin
}

func (a *S3RegistryAdapter) Create(ctx context.Context, spec adapter.BodySpec) (adapter.Handle, error) {
	return "", fmt.Errorf("s3-registry: Create not supported")
}
func (a *S3RegistryAdapter) Start(ctx context.Context, id adapter.Handle) error {
	return fmt.Errorf("s3-registry: Start not supported")
}
func (a *S3RegistryAdapter) Stop(ctx context.Context, id adapter.Handle, opts adapter.StopOpts) error {
	return fmt.Errorf("s3-registry: Stop not supported")
}
func (a *S3RegistryAdapter) Destroy(ctx context.Context, id adapter.Handle) error {
	return fmt.Errorf("s3-registry: Destroy not supported")
}
func (a *S3RegistryAdapter) GetStatus(ctx context.Context, id adapter.Handle) (adapter.BodyStatus, error) {
	return adapter.BodyStatus{}, fmt.Errorf("s3-registry: GetStatus not supported")
}
func (a *S3RegistryAdapter) Exec(ctx context.Context, id adapter.Handle, cmd []string) (adapter.ExecResult, error) {
	return adapter.ExecResult{}, fmt.Errorf("s3-registry: Exec not supported")
}
func (a *S3RegistryAdapter) ExportFilesystem(ctx context.Context, id adapter.Handle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("s3-registry: ExportFilesystem not supported")
}
func (a *S3RegistryAdapter) ImportFilesystem(ctx context.Context, id adapter.Handle, tarball io.Reader, opts adapter.ImportOpts) error {
	return fmt.Errorf("s3-registry: ImportFilesystem not supported")
}
func (a *S3RegistryAdapter) Inspect(ctx context.Context, id adapter.Handle) (adapter.ContainerMetadata, error) {
	return adapter.ContainerMetadata{}, fmt.Errorf("s3-registry: Inspect not supported")
}
func (a *S3RegistryAdapter) Capabilities() adapter.AdapterCapabilities {
	return adapter.AdapterCapabilities{}
}
func (a *S3RegistryAdapter) SubstrateName() string {
	return "s3-registry"
}
func (a *S3RegistryAdapter) IsHealthy(ctx context.Context) bool {
	_, err := a.plugin.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(a.plugin.cfg.Bucket),
	})
	return err == nil
}
